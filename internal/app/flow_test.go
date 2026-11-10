package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/cng1985/dbgo/internal/app"
	"github.com/cng1985/dbgo/internal/platform/config"
)

func TestConsoleFlow(t *testing.T) {
	root := moduleRoot(t)
	dataDir := t.TempDir()
	port := freePort(t)
	t.Setenv("APP_AUTH_ADMIN_PASSWORD", "test-pass-123")
	t.Setenv("APP_PROFILE", "")
	cfg, err := config.Load(config.Locator{
		BaseDir:   root,
		ConfigDir: filepath.Join(root, "configs"),
		DataDir:   dataDir,
		Sets:      []string{fmt.Sprintf("server.port=%d", port)},
	})
	if err != nil {
		t.Fatalf("加载配置: %v", err)
	}
	application := fx.New(fx.Supply(cfg), app.Module, fx.NopLogger)
	startCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := application.Start(startCtx); err != nil {
		t.Fatalf("启动: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = application.Stop(stopCtx)
	})

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitReady(t, base)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}

	unknownBody := getRaw(t, client, base+"/api/missing")
	if unknownBody.Status != http.StatusNotFound || !bytes.Contains(unknownBody.Body, []byte("接口不存在")) {
		t.Fatalf("未知接口应返回 JSON 404，实际 %d %s", unknownBody.Status, unknownBody.Body)
	}
	page := getRaw(t, client, base+"/workspace")
	if page.Status != http.StatusOK || !bytes.Contains(page.Body, []byte("<div id=\"app\">")) {
		t.Fatalf("页面路由应回退到 SPA，实际 %d %s", page.Status, page.Body)
	}
	missingAsset := getRaw(t, client, base+"/assets/missing.js")
	if missingAsset.Status != http.StatusNotFound {
		t.Fatalf("缺失静态资源应返回 404，实际 %d", missingAsset.Status)
	}

	denied := post(t, client, base+"/api/overview/one", map[string]any{})
	if denied.Code != 401 {
		t.Fatalf("未登录应返回 401，实际 %#v", denied)
	}
	badLogin := post(t, client, base+"/api/auth/login", map[string]any{"username": "admin", "password": "wrong-password"})
	if badLogin.Code != 400 {
		t.Fatalf("错误口令应被拒绝，实际 %#v", badLogin)
	}
	login := post(t, client, base+"/api/auth/login", map[string]any{"username": "admin", "password": "test-pass-123"})
	if login.Code != 200 || login.Data["username"] != "admin" || login.Data["session_token"] != nil {
		t.Fatalf("登录结果不正确: %#v", login)
	}

	created := post(t, client, base+"/api/connection/create", map[string]any{
		"name":        "本地示例",
		"driver":      "sqlite",
		"sqlite_path": "demo.db",
		"remark":      "",
	})
	if created.Code != 200 {
		t.Fatalf("创建连接失败: %#v", created)
	}
	connectionID, _ := created.Data["id"].(string)
	if connectionID == "" || connectionID == "0" {
		t.Fatalf("连接主键应为十进制字符串: %#v", created.Data)
	}
	updated := post(t, client, base+"/api/connection/update", map[string]any{
		"id":          connectionID,
		"name":        "本地示例",
		"driver":      "sqlite",
		"sqlite_path": "demo.db",
		"remark":      "",
		"version":     1,
	})
	if updated.Code != 200 || updated.Data["remark"] != "" {
		t.Fatalf("空备注应能保存: %#v", updated)
	}
	conflict := post(t, client, base+"/api/connection/update", map[string]any{
		"id":          connectionID,
		"name":        "本地示例",
		"driver":      "sqlite",
		"sqlite_path": "demo.db",
		"version":     1,
	})
	if conflict.Code != 409 {
		t.Fatalf("旧版本更新应冲突: %#v", conflict)
	}
	tested := post(t, client, base+"/api/connection/test", map[string]any{"id": connectionID})
	if tested.Code != 200 || tested.Data["server_version"] == "" {
		t.Fatalf("连接测试失败: %#v", tested)
	}

	executed := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     false,
		"sql": "create table city (id integer primary key, name text not null, population integer not null default 0);\n" +
			"insert into city (name, population) values ('杭州', 12000000);\n" +
			"insert into city (name, population) values ('', 0);",
	})
	if executed.Code != 200 {
		t.Fatalf("执行 SQL 失败: %#v", executed)
	}
	selected := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"sql":           "select id, name, population from city order by id",
	})
	if selected.Code != 200 {
		t.Fatalf("查询失败: %#v", selected)
	}
	statements, _ := selected.Data["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("应返回一个结果集: %#v", selected.Data)
	}
	first, _ := statements[0].(map[string]any)
	rows, _ := first["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("应读到两行城市数据: %#v", first)
	}
	blocked := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"sql":           "delete from city",
	})
	if blocked.Code != 10002 {
		t.Fatalf("只读模式应拒绝写入: %#v", blocked)
	}

	tables := post(t, client, base+"/api/catalog/tables", map[string]any{"connection_id": connectionID})
	if tables.Code != 200 || !containsName(tables.Data["tables"], "city") {
		t.Fatalf("目录中应看到 city: %#v", tables)
	}
	columns := post(t, client, base+"/api/catalog/columns", map[string]any{"connection_id": connectionID, "table": "city"})
	if columns.Code != 200 || !containsName(columns.Data["columns"], "population") {
		t.Fatalf("应看到字段 population: %#v", columns)
	}
	suggest := post(t, client, base+"/api/catalog/suggest", map[string]any{"connection_id": connectionID})
	if suggest.Code != 200 || suggest.Data["driver"] != "sqlite" || suggest.Data["schema"] != "main" || !containsName(suggest.Data["schemas"], "main") || !suggestHasColumn(suggest.Data, "city", "population") {
		t.Fatalf("结构提示应包含 city.population: %#v", suggest)
	}
	if suggest.Data["truncated"] != false {
		t.Fatalf("小库不应截断结构提示: %#v", suggest.Data["truncated"])
	}

	badMigration := post(t, client, base+"/api/migration/create", map[string]any{
		"connection_id": connectionID,
		"version_no":    1,
		"name":          "失败的版本",
		"up_sql":        "create table",
		"down_sql":      "select 1",
	})
	if badMigration.Code != 200 {
		t.Fatalf("创建迁移失败: %#v", badMigration)
	}
	badID, _ := badMigration.Data["id"].(string)
	failedApply := post(t, client, base+"/api/migration/apply", map[string]any{"id": badID})
	if failedApply.Code != 10003 {
		t.Fatalf("错误脚本应执行失败: %#v", failedApply)
	}
	failedOne := post(t, client, base+"/api/migration/one", map[string]any{"id": badID})
	if failedOne.Data["status"] != "failed" {
		t.Fatalf("失败后不应标记为已执行: %#v", failedOne.Data)
	}
	fixed := post(t, client, base+"/api/migration/update", map[string]any{
		"id":       badID,
		"name":     "笔记",
		"up_sql":   "create table note (id integer primary key, body text not null); insert into note (body) values ('第一条');",
		"down_sql": "drop table note;",
	})
	if fixed.Code != 200 || fixed.Data["status"] != "draft" {
		t.Fatalf("失败迁移应能改回草稿: %#v", fixed)
	}
	applied := post(t, client, base+"/api/migration/apply", map[string]any{"id": badID})
	if applied.Code != 200 || applied.Data["status"] != "applied" {
		t.Fatalf("迁移执行失败: %#v", applied)
	}
	rolled := post(t, client, base+"/api/migration/rollback", map[string]any{"id": badID})
	if rolled.Code != 200 || rolled.Data["status"] != "draft" {
		t.Fatalf("迁移回滚失败: %#v", rolled)
	}
	afterRollback := post(t, client, base+"/api/catalog/tables", map[string]any{"connection_id": connectionID})
	if containsName(afterRollback.Data["tables"], "note") {
		t.Fatalf("回滚后不应仍存在 note: %#v", afterRollback.Data["tables"])
	}

	backup := post(t, client, base+"/api/backup/create", map[string]any{
		"connection_id": connectionID,
		"name":          "城市备份",
		"format":        "sql",
	})
	if backup.Code != 200 || backup.Data["status"] != "ready" {
		t.Fatalf("备份失败: %#v", backup)
	}
	backupID, _ := backup.Data["id"].(string)
	dropped := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     false,
		"sql":           "drop table city",
	})
	if dropped.Code != 200 {
		t.Fatalf("删除表失败: %#v", dropped)
	}
	restored := post(t, client, base+"/api/backup/restore", map[string]any{"id": backupID})
	if restored.Code != 200 {
		t.Fatalf("恢复失败: %#v", restored)
	}
	again := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"sql":           "select count(*) as total from city",
	})
	if again.Code != 200 {
		t.Fatalf("恢复后查询失败: %#v", again)
	}

	emptyPage := post(t, client, base+"/api/connection/search", map[string]any{"keyword": "不存在的连接", "page": 1, "page_size": 20})
	if emptyPage.Code != 0 || emptyPage.Raw["total"] != float64(0) {
		t.Fatalf("空分页应返回空列表和总数 0: %#v", emptyPage.Raw)
	}
	if _, ok := emptyPage.Raw["data"].([]any); !ok {
		t.Fatalf("空分页 data 不能为 null: %#v", emptyPage.Raw)
	}
	invalidPage := post(t, client, base+"/api/connection/search", map[string]any{"page": 1, "page_size": 201})
	if invalidPage.Code != 400 {
		t.Fatalf("超限分页应返回 400: %#v", invalidPage)
	}
	deleted := post(t, client, base+"/api/connection/delete", map[string]any{"id": connectionID})
	if deleted.Code != 200 {
		t.Fatalf("删除连接失败: %#v", deleted)
	}
	deletedAgain := post(t, client, base+"/api/connection/delete", map[string]any{"id": connectionID})
	if deletedAgain.Code != 404 {
		t.Fatalf("重复删除应返回不存在: %#v", deletedAgain)
	}
	overview := post(t, client, base+"/api/overview/one", map[string]any{})
	if overview.Code != 200 {
		t.Fatalf("总览失败: %#v", overview)
	}
}

func TestConfigRejectsUnknownProfileAndHonorsFalse(t *testing.T) {
	root := moduleRoot(t)
	_, err := config.Load(config.Locator{BaseDir: root, ConfigDir: filepath.Join(root, "configs"), Profile: "staging"})
	if err == nil {
		t.Fatal("未知 profile 应启动失败")
	}
	t.Setenv("APP_DATABASE_SQLITE_FOREIGN_KEYS", "false")
	t.Setenv("APP_SERVER_PORT", "0")
	cfg, err := config.Load(config.Locator{BaseDir: root, ConfigDir: filepath.Join(root, "configs"), DataDir: t.TempDir()})
	if err == nil {
		t.Fatal("端口 0 应校验失败")
	}
	t.Setenv("APP_SERVER_PORT", "18080")
	cfg, err = config.Load(config.Locator{BaseDir: root, ConfigDir: filepath.Join(root, "configs"), DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("覆盖端口失败: %v", err)
	}
	if cfg.Database.SQLite.ForeignKeys {
		t.Fatal("显式 false 不能被当成未配置")
	}
	if cfg.Server.Port != 18080 {
		t.Fatalf("环境变量端口未生效: %d", cfg.Server.Port)
	}
}

type apiResult struct {
	Code int
	Data map[string]any
	Raw  map[string]any
}

type rawResponse struct {
	Status int
	Body   []byte
}

func post(t *testing.T, client *http.Client, url string, body map[string]any) apiResult {
	t.Helper()
	payload, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-DBGo-Request", "1")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("响应不是 JSON: %s", raw)
	}
	result := apiResult{Raw: parsed}
	if code, ok := parsed["code"].(float64); ok {
		result.Code = int(code)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		result.Data = data
	}
	return result
}

func getRaw(t *testing.T, client *http.Client, url string) rawResponse {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return rawResponse{Status: response.StatusCode, Body: body}
}

func waitReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(base + "/health/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("服务未就绪")
}

func suggestHasColumn(data map[string]any, table, column string) bool {
	tables, _ := data["tables"].([]any)
	for _, item := range tables {
		row, _ := item.(map[string]any)
		if row["name"] != table {
			continue
		}
		columns, _ := row["columns"].([]any)
		for _, col := range columns {
			field, _ := col.(map[string]any)
			if field["name"] == column {
				return true
			}
		}
	}
	return false
}

func containsName(value any, name string) bool {
	items, _ := value.([]any)
	for _, item := range items {
		row, _ := item.(map[string]any)
		if row["name"] == name {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位模块根目录")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

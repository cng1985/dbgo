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
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
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
		"schema":        "main",
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
	badSchema := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"schema":        "bad\x00name",
		"sql":           "select 1",
	})
	if badSchema.Code != 400 {
		t.Fatalf("无效数据库名应拒绝: %#v", badSchema)
	}
	blocked := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"sql":           "delete from city",
	})
	if blocked.Code != 10002 {
		t.Fatalf("只读模式应拒绝写入: %#v", blocked)
	}

	dest := post(t, client, base+"/api/connection/create", map[string]any{
		"name":        "目标库",
		"driver":      "sqlite",
		"sqlite_path": "dest.db",
		"remark":      "",
	})
	if dest.Code != 200 {
		t.Fatalf("创建目标连接失败: %#v", dest)
	}
	destID, _ := dest.Data["id"].(string)
	createdPlace := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": destID,
		"read_only":     false,
		"sql":           "create table place (code integer primary key, title text)",
	})
	if createdPlace.Code != 200 {
		t.Fatalf("创建目标表失败: %#v", createdPlace)
	}
	rejectedETL := post(t, client, base+"/api/etl/create", map[string]any{
		"name":                 "非法抽取",
		"source_connection_id": connectionID,
		"source_sql":           "delete from city",
		"target_connection_id": destID,
		"target_table":         "place",
		"write_mode":           "append",
	})
	if rejectedETL.Code != 400 {
		t.Fatalf("写入语句不能当作抽取: %#v", rejectedETL)
	}
	etlPreview := post(t, client, base+"/api/etl/preview", map[string]any{
		"source_connection_id": connectionID,
		"source_sql":           "select id, upper(name) as title from city order by id",
	})
	previewColumns, _ := etlPreview.Data["columns"].([]any)
	if etlPreview.Code != 200 || len(previewColumns) != 2 {
		t.Fatalf("预览应返回字段: %#v", etlPreview)
	}
	task := post(t, client, base+"/api/etl/create", map[string]any{
		"name":                 "城市同步",
		"source_connection_id": connectionID,
		"source_sql":           "select id, upper(name) as title from city order by id",
		"target_connection_id": destID,
		"target_table":         "place",
		"write_mode":           "replace",
		"mappings": []any{
			map[string]any{"source": "id", "target": "code"},
			map[string]any{"source": "title", "target": "title"},
		},
	})
	if task.Code != 200 || task.Data["version"] != float64(1) {
		t.Fatalf("创建 ETL 任务失败: %#v", task)
	}
	taskID, _ := task.Data["id"].(string)
	ran := post(t, client, base+"/api/etl/run", map[string]any{"id": taskID})
	if ran.Code != 200 || ran.Data["status"] != "succeeded" || ran.Data["rows_written"] != float64(2) {
		t.Fatalf("ETL 应写入两行: %#v", ran)
	}
	copied := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": destID,
		"read_only":     true,
		"sql":           "select code, title from place order by code",
	})
	copiedStatements, _ := copied.Data["statements"].([]any)
	copiedFirst, _ := copiedStatements[0].(map[string]any)
	copiedRows, _ := copiedFirst["rows"].([]any)
	copiedRow, _ := copiedRows[0].([]any)
	if copied.Code != 200 || len(copiedRows) != 2 || len(copiedRow) < 2 || copiedRow[1] != "杭州" {
		t.Fatalf("目标表应读到转换后的杭州: %#v", copied)
	}
	listed := post(t, client, base+"/api/etl/search", map[string]any{"page": 1, "page_size": 20})
	listedRows, _ := listed.Raw["data"].([]any)
	listedFirst, _ := listedRows[0].(map[string]any)
	if listed.Raw["total"] != float64(1) || listedFirst["last_status"] != "succeeded" || listedFirst["last_rows_written"] != float64(2) {
		t.Fatalf("任务列表应带上最近一次执行: %#v", listed.Raw)
	}
	conflicted := post(t, client, base+"/api/etl/update", map[string]any{
		"id":                   taskID,
		"version":              1,
		"name":                 "城市同步",
		"source_connection_id": connectionID,
		"source_sql":           "select id, upper(name) as title from city order by id",
		"target_connection_id": destID,
		"target_table":         "place",
		"write_mode":           "append",
		"mappings": []any{
			map[string]any{"source": "id", "target": "code"},
			map[string]any{"source": "title", "target": "title"},
		},
	})
	if conflicted.Code != 200 || conflicted.Data["version"] != float64(2) || conflicted.Data["write_mode"] != "append" {
		t.Fatalf("应能用当前版本修改任务: %#v", conflicted)
	}
	stale := post(t, client, base+"/api/etl/update", map[string]any{
		"id":                   taskID,
		"version":              1,
		"name":                 "城市同步",
		"source_connection_id": connectionID,
		"source_sql":           "select id, name from city",
		"target_connection_id": destID,
		"target_table":         "place",
		"write_mode":           "append",
	})
	if stale.Code != 409 {
		t.Fatalf("旧版本更新应冲突: %#v", stale)
	}
	missingTask := post(t, client, base+"/api/etl/run", map[string]any{"id": "999999"})
	if missingTask.Code != 404 {
		t.Fatalf("不存在的任务应返回 404: %#v", missingTask)
	}

	tables := post(t, client, base+"/api/catalog/tables", map[string]any{"connection_id": connectionID})
	if tables.Code != 200 || !containsName(tables.Data["tables"], "city") {
		t.Fatalf("目录中应看到 city: %#v", tables)
	}
	columns := post(t, client, base+"/api/catalog/columns", map[string]any{"connection_id": connectionID, "table": "city"})
	if columns.Code != 200 || !containsName(columns.Data["columns"], "population") {
		t.Fatalf("应看到字段 population: %#v", columns)
	}
	cityColumns, _ := columns.Data["columns"].([]any)
	if len(cityColumns) != 3 {
		t.Fatalf("city 应返回全部字段: %#v", columns.Data["columns"])
	}
	cityDef := post(t, client, base+"/api/catalog/definition", map[string]any{
		"connection_id": connectionID,
		"kind":          "table",
		"name":          "city",
	})
	cityDrafts, _ := cityDef.Data["columns"].([]any)
	if cityDef.Code != 200 || len(cityDrafts) != 3 || !containsName(cityDef.Data["columns"], "population") {
		t.Fatalf("编辑表定义时应带回全部字段: %#v", cityDef)
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

	preview := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver":     "sqlite",
		"kind":       "view",
		"name":       "city_names",
		"select_sql": "select id, name from city",
	})
	viewSQL, _ := preview.Data["sql"].(string)
	if preview.Code != 200 || !strings.Contains(viewSQL, "city_names") {
		t.Fatalf("视图预览失败: %#v", preview)
	}
	appliedView := post(t, client, base+"/api/catalog/apply", map[string]any{
		"connection_id": connectionID,
		"sql":           viewSQL,
	})
	if appliedView.Code != 200 {
		t.Fatalf("执行建视图失败: %#v", appliedView)
	}
	objects := post(t, client, base+"/api/catalog/objects", map[string]any{"connection_id": connectionID, "kind": "view"})
	if objects.Code != 200 || !containsName(objects.Data["objects"], "city_names") {
		t.Fatalf("应列出视图 city_names: %#v", objects)
	}
	definition := post(t, client, base+"/api/catalog/definition", map[string]any{
		"connection_id": connectionID,
		"kind":          "view",
		"name":          "city_names",
	})
	definedSQL, _ := definition.Data["sql"].(string)
	if definition.Code != 200 || !strings.Contains(strings.ToLower(definedSQL), "city") {
		t.Fatalf("应读到视图定义: %#v", definition)
	}
	droppedView := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     false,
		"sql":           "drop view city_names",
	})
	if droppedView.Code != 200 {
		t.Fatalf("删除视图失败: %#v", droppedView)
	}

	snapshot := post(t, client, base+"/api/monitor/snapshot", map[string]any{"connection_id": connectionID})
	metrics, _ := snapshot.Data["metrics"].([]any)
	if snapshot.Code != 200 || snapshot.Data["driver"] != "sqlite" || len(metrics) == 0 {
		t.Fatalf("性能快照失败: %#v", snapshot)
	}

	placed := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     false,
		"sql": `create table place (
			id integer primary key autoincrement,
			name text not null /* 地名 */,
			code text not null default 'CN'
		)`,
	})
	if placed.Code != 200 {
		t.Fatalf("创建带注释的表失败: %#v", placed)
	}
	placeColumns := post(t, client, base+"/api/catalog/columns", map[string]any{"connection_id": connectionID, "table": "place"})
	placeList, _ := placeColumns.Data["columns"].([]any)
	if placeColumns.Code != 200 || len(placeList) != 3 || fieldOf(t, placeColumns.Data["columns"], "name", "comment") != "地名" {
		t.Fatalf("字段应包含注释: %#v", placeColumns)
	}
	if fieldOf(t, placeColumns.Data["columns"], "id", "identity") != true {
		t.Fatalf("应识别自增主键: %#v", placeColumns.Data["columns"])
	}
	placeDef := post(t, client, base+"/api/catalog/definition", map[string]any{
		"connection_id": connectionID,
		"kind":          "table",
		"name":          "place",
	})
	placeDrafts, _ := placeDef.Data["columns"].([]any)
	if placeDef.Code != 200 || len(placeDrafts) != 3 || fieldOf(t, placeDef.Data["columns"], "name", "comment") != "地名" {
		t.Fatalf("编辑定义应带回注释: %#v", placeDef)
	}
	seeded := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     false,
		"sql":           "insert into place (name) values ('杭州')",
	})
	if seeded.Code != 200 {
		t.Fatalf("写入示例行失败: %#v", seeded)
	}
	sameAlter := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver":        "sqlite",
		"kind":          "table",
		"mode":          "alter",
		"name":          "place",
		"baseline_name": "place",
		"baseline":      placeDef.Data["columns"],
		"columns":       placeDef.Data["columns"],
	})
	if sameAlter.Code != 200 || sameAlter.Data["changed"] == true || strings.TrimSpace(fmt.Sprint(sameAlter.Data["sql"])) != "" {
		t.Fatalf("未改动的表不应生成更新语句: %#v", sameAlter)
	}
	rejectedAlter := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver":        "sqlite",
		"kind":          "view",
		"mode":          "alter",
		"name":          "place",
		"baseline_name": "place",
	})
	if rejectedAlter.Code == 200 {
		t.Fatalf("视图不能走更新表结构: %#v", rejectedAlter)
	}
	placeRaw, err := json.Marshal(placeDef.Data["columns"])
	if err != nil {
		t.Fatal(err)
	}
	var withNote []any
	if err := json.Unmarshal(placeRaw, &withNote); err != nil {
		t.Fatal(err)
	}
	withNote = append(withNote, map[string]any{"name": "note", "type": "text", "nullable": true})
	addColumn := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver":        "sqlite",
		"kind":          "table",
		"mode":          "alter",
		"name":          "place",
		"baseline_name": "place",
		"baseline":      placeDef.Data["columns"],
		"columns":       withNote,
	})
	addSQL, _ := addColumn.Data["sql"].(string)
	if addColumn.Code != 200 || addColumn.Data["changed"] != true || !strings.Contains(addSQL, `ADD COLUMN "note"`) {
		t.Fatalf("应生成新增字段语句: %#v", addColumn)
	}
	appliedAlter := post(t, client, base+"/api/catalog/apply", map[string]any{
		"connection_id": connectionID,
		"sql":           addSQL,
	})
	if appliedAlter.Code != 200 {
		t.Fatalf("执行更新表结构失败: %#v", appliedAlter)
	}
	afterAdd := post(t, client, base+"/api/catalog/definition", map[string]any{
		"connection_id": connectionID,
		"kind":          "table",
		"name":          "place",
	})
	if afterAdd.Code != 200 || !containsName(afterAdd.Data["columns"], "note") {
		t.Fatalf("更新后应看到 note: %#v", afterAdd)
	}
	var retitled []any
	retitledRaw, err := json.Marshal(afterAdd.Data["columns"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(retitledRaw, &retitled); err != nil {
		t.Fatal(err)
	}
	for _, item := range retitled {
		row, _ := item.(map[string]any)
		if row["name"] == "name" {
			row["comment"] = "城市"
		}
	}
	commentAlter := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver":        "sqlite",
		"kind":          "table",
		"mode":          "alter",
		"name":          "place",
		"baseline_name": "place",
		"baseline":      afterAdd.Data["columns"],
		"columns":       retitled,
	})
	commentSQL, _ := commentAlter.Data["sql"].(string)
	if commentAlter.Code != 200 || !strings.Contains(commentSQL, "INSERT INTO") || !strings.Contains(commentSQL, "城市") {
		t.Fatalf("修改注释应生成重建语句: %#v", commentAlter)
	}
	appliedComment := post(t, client, base+"/api/catalog/apply", map[string]any{
		"connection_id": connectionID,
		"sql":           commentSQL,
	})
	if appliedComment.Code != 200 {
		t.Fatalf("执行重建失败: %#v", appliedComment)
	}
	retitledColumns := post(t, client, base+"/api/catalog/columns", map[string]any{"connection_id": connectionID, "table": "place"})
	if fieldOf(t, retitledColumns.Data["columns"], "name", "comment") != "城市" {
		t.Fatalf("重建后应读到新注释: %#v", retitledColumns)
	}
	keptRow := post(t, client, base+"/api/query/execute", map[string]any{
		"connection_id": connectionID,
		"read_only":     true,
		"sql":           "select name from place",
	})
	keptStatements, _ := keptRow.Data["statements"].([]any)
	keptFirst, _ := keptStatements[0].(map[string]any)
	keptRows, _ := keptFirst["rows"].([]any)
	keptCells, _ := keptRows[0].([]any)
	if keptRow.Code != 200 || len(keptRows) != 1 || keptCells[0] != "杭州" {
		t.Fatalf("更新表结构后应保留原数据: %#v", keptRow)
	}
	previewTable := post(t, client, base+"/api/catalog/preview", map[string]any{
		"driver": "sqlite",
		"kind":   "table",
		"name":   "place2",
		"columns": []map[string]any{
			{"name": "id", "type": "integer", "primary": true, "identity": true, "comment": "主键"},
			{"name": "title", "type": "text", "nullable": false, "comment": "标题"},
		},
	})
	tableSQL, _ := previewTable.Data["sql"].(string)
	if previewTable.Code != 200 || !strings.Contains(tableSQL, "主键") || !strings.Contains(tableSQL, "标题") {
		t.Fatalf("生成 SQL 应包含注释: %#v", previewTable)
	}
	appliedTable := post(t, client, base+"/api/catalog/apply", map[string]any{
		"connection_id": connectionID,
		"sql":           tableSQL,
	})
	if appliedTable.Code != 200 {
		t.Fatalf("执行带注释的建表失败: %#v", appliedTable)
	}
	reloaded := post(t, client, base+"/api/catalog/columns", map[string]any{"connection_id": connectionID, "table": "place2"})
	reloadedColumns, _ := reloaded.Data["columns"].([]any)
	if reloaded.Code != 200 || len(reloadedColumns) != 2 || fieldOf(t, reloaded.Data["columns"], "title", "comment") != "标题" {
		t.Fatalf("执行后应读回全部字段和注释: %#v", reloaded)
	}

	secret := post(t, client, base+"/api/connection/create", map[string]any{
		"name":        "口令库",
		"driver":      "sqlite",
		"sqlite_path": "secret.db",
		"password":    "db-secret",
	})
	if secret.Code != 200 {
		t.Fatalf("创建带口令的连接失败: %#v", secret)
	}
	secretID, _ := secret.Data["id"].(string)
	exported := post(t, client, base+"/api/connection/export", map[string]any{
		"ids":              []string{secretID},
		"include_password": false,
	})
	exportedItems, _ := exported.Data["connections"].([]any)
	if exported.Code != 200 || exported.Data["format"] != "dbgo-connections" || len(exportedItems) != 1 {
		t.Fatalf("应能导出连接: %#v", exported)
	}
	exportedFirst, _ := exportedItems[0].(map[string]any)
	if exportedFirst["name"] != "口令库" || exportedFirst["password"] != nil || exportedFirst["sqlite_path"] == "" {
		t.Fatalf("默认导出不应包含口令: %#v", exportedFirst)
	}
	withPassword := post(t, client, base+"/api/connection/export", map[string]any{
		"ids":              []string{secretID},
		"include_password": true,
	})
	withItems, _ := withPassword.Data["connections"].([]any)
	withFirst, _ := withItems[0].(map[string]any)
	if withPassword.Code != 200 || withFirst["password"] != "db-secret" {
		t.Fatalf("勾选后应导出连接口令: %#v", withPassword.Data)
	}

	createdSnippet := post(t, client, base+"/api/snippet/create", map[string]any{
		"name":        "查询城市",
		"description": "按名称筛选",
		"driver":      "sqlite",
		"body":        "select id, name from city where name = '杭州'",
	})
	if createdSnippet.Code != 200 || createdSnippet.Data["body"] == "" {
		t.Fatalf("创建代码段失败: %#v", createdSnippet)
	}
	snippetID, _ := createdSnippet.Data["id"].(string)
	exportedSnippet := post(t, client, base+"/api/snippet/export", map[string]any{
		"ids": []string{snippetID},
	})
	snippetItems, _ := exportedSnippet.Data["snippets"].([]any)
	if exportedSnippet.Code != 200 || exportedSnippet.Data["format"] != "dbgo-snippets" || len(snippetItems) != 1 {
		t.Fatalf("应能导出代码段: %#v", exportedSnippet)
	}
	importedSnippet := post(t, client, base+"/api/snippet/import", map[string]any{
		"document":    exportedSnippet.Data,
		"on_conflict": "rename",
	})
	if importedSnippet.Code != 200 || importedSnippet.Data["created"] != float64(1) {
		t.Fatalf("重名代码段导入应改名后创建: %#v", importedSnippet)
	}
	imported := post(t, client, base+"/api/connection/import", map[string]any{
		"document": map[string]any{
			"format":  "dbgo-connections",
			"version": 1,
			"connections": []map[string]any{{
				"name":        "本地示例",
				"driver":      "sqlite",
				"sqlite_path": "imported.db",
			}},
		},
		"on_conflict": "rename",
	})
	if imported.Code != 200 || imported.Data["created"] != float64(1) {
		t.Fatalf("重名导入应改名后创建: %#v", imported)
	}

	options := post(t, client, base+"/api/auth/options", map[string]any{})
	if options.Code != 200 || options.Data["allow_register"] != true {
		t.Fatalf("注册开关应开启: %#v", options)
	}
	memberJar, _ := cookiejar.New(nil)
	member := &http.Client{Jar: memberJar, Timeout: 20 * time.Second}
	registered := post(t, member, base+"/api/auth/register", map[string]any{
		"username":     "member1",
		"password":     "test-pass-123",
		"display_name": "成员",
		"email":        "member@example.com",
	})
	if registered.Code != 200 || registered.Data["role"] != "member" || registered.Data["session_token"] != nil {
		t.Fatalf("注册结果不正确: %#v", registered)
	}
	memberOverview := post(t, member, base+"/api/overview/one", map[string]any{})
	recent, _ := memberOverview.Data["recent_queries"].([]any)
	if memberOverview.Code != 200 || memberOverview.Data["connection_count"] != "0" || len(recent) != 0 {
		t.Fatalf("注册用户不应看到别人的连接和查询: %#v", memberOverview)
	}
	hidden := post(t, member, base+"/api/connection/search", map[string]any{"page": 1, "page_size": 50})
	if hidden.Raw["total"] != float64(0) {
		t.Fatalf("注册用户的连接列表应为空: %#v", hidden.Raw)
	}
	deniedOne := post(t, member, base+"/api/connection/one", map[string]any{"id": connectionID})
	if deniedOne.Code != 404 {
		t.Fatalf("不能读取别人的连接: %#v", deniedOne)
	}
	hiddenSnippets := post(t, member, base+"/api/snippet/search", map[string]any{"page": 1, "page_size": 50})
	if hiddenSnippets.Raw["total"] != float64(0) {
		t.Fatalf("注册用户不应看到别人的代码段: %#v", hiddenSnippets.Raw)
	}
	deniedSnippet := post(t, member, base+"/api/snippet/one", map[string]any{"id": snippetID})
	if deniedSnippet.Code != 404 {
		t.Fatalf("不能读取别人的代码段: %#v", deniedSnippet)
	}
	deniedSQL := post(t, member, base+"/api/query/execute", map[string]any{"connection_id": connectionID, "sql": "select 1"})
	if deniedSQL.Code != 404 {
		t.Fatalf("不能在别人的连接上执行 SQL: %#v", deniedSQL)
	}
	deniedMigration := post(t, member, base+"/api/migration/search", map[string]any{"connection_id": connectionID, "page": 1, "page_size": 20})
	if deniedMigration.Raw["total"] != float64(0) {
		t.Fatalf("不能看到别人的迁移: %#v", deniedMigration.Raw)
	}
	own := post(t, member, base+"/api/connection/create", map[string]any{
		"name":        "本地示例",
		"driver":      "sqlite",
		"sqlite_path": "member.db",
	})
	if own.Code != 200 {
		t.Fatalf("另一用户应能创建同名连接: %#v", own)
	}
	ownID, _ := own.Data["id"].(string)
	memberList := post(t, member, base+"/api/connection/search", map[string]any{"page": 1, "page_size": 50})
	memberItems, _ := memberList.Raw["data"].([]any)
	if memberList.Raw["total"] != float64(1) || len(memberItems) != 1 {
		t.Fatalf("注册用户应只看到自己的连接: %#v", memberList.Raw)
	}
	adminList := post(t, client, base+"/api/connection/search", map[string]any{"page": 1, "page_size": 100})
	for _, item := range adminList.Raw["data"].([]any) {
		row, _ := item.(map[string]any)
		if row["id"] == ownID {
			t.Fatalf("管理员不应看到成员的连接: %#v", row)
		}
	}
	profile := post(t, member, base+"/api/auth/profile/update", map[string]any{
		"display_name": "成员甲",
		"email":        "member@example.com",
	})
	if profile.Code != 200 || profile.Data["display_name"] != "成员甲" {
		t.Fatalf("个人资料更新失败: %#v", profile)
	}
	duplicate := post(t, member, base+"/api/auth/register", map[string]any{
		"username": "member1",
		"password": "test-pass-123",
	})
	if duplicate.Code != 409 {
		t.Fatalf("重复用户名应冲突: %#v", duplicate)
	}

	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"改写完成\",\"sql\":\"select id, name from city\",\"notes\":[\"避免 select *\"]}"}}]}`))
	}))
	defer llmServer.Close()
	model := post(t, client, base+"/api/llm/create", map[string]any{
		"name":       "本地模型",
		"base_url":   llmServer.URL + "/v1",
		"api_key":    "sk-test",
		"model":      "test-model",
		"is_default": true,
	})
	if model.Code != 200 || model.Data["api_key_set"] != true || model.Data["api_key"] != nil {
		t.Fatalf("模型配置不应回显密钥: %#v", model)
	}
	optimized := post(t, client, base+"/api/llm/optimize", map[string]any{
		"connection_id": connectionID,
		"sql":           "select * from city",
	})
	if optimized.Code != 200 || optimized.Data["sql"] != "select id, name from city" {
		t.Fatalf("SQL 优化失败: %#v", optimized)
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

func fieldOf(t *testing.T, value any, name, field string) any {
	t.Helper()
	items, _ := value.([]any)
	for _, item := range items {
		row, _ := item.(map[string]any)
		if row["name"] == name {
			return row[field]
		}
	}
	t.Fatalf("缺少字段 %s: %#v", name, value)
	return nil
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

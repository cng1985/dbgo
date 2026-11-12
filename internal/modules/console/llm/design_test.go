package llm

import (
	"strings"
	"testing"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
)

func TestParseDesignReadsFencedJSON(t *testing.T) {
	parsed, err := parseDesign("```json\n{\"summary\":\"订单\",\"name\":\"orders\",\"columns\":[{\"name\":\"id\",\"type\":\"integer\",\"primary\":true}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Name != "orders" || len(parsed.Columns) != 1 || !parsed.Columns[0].Primary.Value {
		t.Fatalf("解析结果不正确: %#v", parsed)
	}
}

func TestNormalizeDesignBuildsSQLiteTable(t *testing.T) {
	raw := tableDraft{
		Summary: "订单表",
		Name:    "orders",
		Comment: "订单",
		Columns: []draftColumn{
			{Name: "id", Type: "int", Primary: optionalBool{Value: true, Set: true}, Identity: optionalBool{Value: true, Set: true}, Default: "1", Comment: "主键"},
			{Name: "amount", Type: "varchar(100)", Nullable: optionalBool{Value: false, Set: true}, Default: "0", Comment: "金额"},
			{Name: "status", Type: "text", Nullable: optionalBool{Value: false, Set: true}, Default: "now()", Comment: "状态"},
			{Name: "status", Type: "text"},
			{Name: "note", Type: "text", Default: "1; drop table orders"},
			{Name: "extra", Type: "integer", Primary: optionalBool{Value: true, Set: true}, Identity: optionalBool{Value: true, Set: true}},
		},
	}
	got, err := normalizeDesign("sqlite", raw, []string{"Orders"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.Comment != "订单" || len(got.Columns) != 5 {
		t.Fatalf("表结构不正确: %#v", got)
	}
	if got.Columns[0].Type != "integer" || !got.Columns[0].Primary || !got.Columns[0].Identity || got.Columns[0].Default != "" || got.Columns[0].Nullable {
		t.Fatalf("主键不正确: %#v", got.Columns[0])
	}
	if got.Columns[1].Type != "text" {
		t.Fatalf("金额类型应改成 text: %#v", got.Columns[1])
	}
	if got.Columns[2].Default != "CURRENT_TIMESTAMP" {
		t.Fatalf("时间默认值应改成 CURRENT_TIMESTAMP: %#v", got.Columns[2])
	}
	if got.Columns[3].Default != "" {
		t.Fatalf("危险默认值应去掉: %#v", got.Columns[3])
	}
	if got.Columns[4].Identity || got.Columns[4].Primary {
		t.Fatalf("第二自增列不应保留主键或自增: %#v", got.Columns[4])
	}
	joined := strings.Join(got.Notes, "\n")
	for _, want := range []string{"已改成 text", "已忽略", "默认值无法使用", "已去掉", "唯一主键", "已有同名表"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("说明缺少 %s: %s", want, joined)
		}
	}
	sqlText, err := shared.RenderObject(shared.ObjectDraft{
		Driver:  "sqlite",
		Kind:    "table",
		Name:    got.Name,
		Columns: designColumns(got.Columns),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "PRIMARY KEY AUTOINCREMENT") || !strings.Contains(sqlText, "amount") {
		t.Fatalf("生成的建表语句不正确: %s", sqlText)
	}
}

func TestNormalizeDesignSuppliesPrimaryAndDriverTypes(t *testing.T) {
	mysql, err := normalizeDesign("mysql", tableDraft{
		Name: "customer",
		Columns: []draftColumn{
			{Name: "email", Type: "string"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mysql.Columns[0].Type != "varchar(255)" || !mysql.Columns[0].Primary || mysql.Columns[0].Nullable {
		t.Fatalf("MySQL 字段不正确: %#v", mysql.Columns[0])
	}
	if !strings.Contains(strings.Join(mysql.Notes, " "), "设为主键") {
		t.Fatalf("应说明已补主键: %v", mysql.Notes)
	}
	postgres, err := normalizeDesign("postgres", tableDraft{
		Name: "customer",
		Columns: []draftColumn{
			{Name: "id", Type: "uuid", Primary: optionalBool{Value: true, Set: true}},
			{Name: "created", Type: "timestamptz"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if postgres.Columns[0].Type != "uuid" || postgres.Columns[1].Type != "timestamptz" {
		t.Fatalf("Postgres 类型应保持原样: %#v", postgres.Columns)
	}
	server, err := normalizeDesign("sqlserver", tableDraft{
		Name: "customer",
		Columns: []draftColumn{
			{Name: "id", Type: "int", Primary: optionalBool{Value: true, Set: true}, Identity: optionalBool{Value: true, Set: true}},
			{Name: "name", Type: "varchar(20)"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if server.Columns[0].Type != "int" || !server.Columns[0].Identity || server.Columns[1].Type != "varchar(255)" {
		t.Fatalf("SQL Server 类型不正确: %#v", server.Columns)
	}
}

func TestNormalizeDesignRejectsEmptyDraft(t *testing.T) {
	if _, err := normalizeDesign("sqlite", tableDraft{Name: "orders"}, nil); err == nil {
		t.Fatal("没有字段时应失败")
	}
	if _, err := normalizeDesign("sqlite", tableDraft{Columns: []draftColumn{{Name: "id", Type: "integer"}}}, nil); err == nil {
		t.Fatal("没有表名时应失败")
	}
}

func designColumns(columns []DesignColumn) []shared.ColumnDraft {
	drafts := make([]shared.ColumnDraft, 0, len(columns))
	for _, column := range columns {
		drafts = append(drafts, shared.ColumnDraft{
			Name:     column.Name,
			Type:     column.Type,
			Nullable: column.Nullable,
			Primary:  column.Primary,
			Identity: column.Identity,
			Default:  column.Default,
			Comment:  column.Comment,
		})
	}
	return drafts
}

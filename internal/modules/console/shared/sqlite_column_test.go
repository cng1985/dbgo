package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSQLiteColumnsKeepsCommentAndIdentity(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "columns.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`create table place (
		id integer primary key autoincrement,
		name text not null /* 地名 */,
		note text default 'a,b' /* 含逗号 */,
		code text not null default 'CN',
		"primary" text
	)`)
	if err != nil {
		t.Fatal(err)
	}
	items, err := ListColumns(context.Background(), db, "sqlite", "main", "place")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("应读到全部 5 个字段: %#v", items)
	}
	byName := map[string]ColumnInfo{}
	for _, item := range items {
		byName[item.Name] = item
	}
	if !byName["id"].Primary || !byName["id"].Identity || byName["id"].Nullable || byName["id"].Type != "INTEGER" {
		t.Fatalf("自增主键: %#v", byName["id"])
	}
	if byName["name"].Comment != "地名" || byName["name"].Nullable {
		t.Fatalf("name: %#v", byName["name"])
	}
	if byName["note"].Comment != "含逗号" || byName["note"].Default != "'a,b'" {
		t.Fatalf("note: %#v", byName["note"])
	}
	if byName["code"].Comment != "" || !strings.EqualFold(byName["code"].Type, "text") || byName["code"].Default != "'CN'" {
		t.Fatalf("code: %#v", byName["code"])
	}
	if byName["primary"].Name != "primary" || !strings.EqualFold(byName["primary"].Type, "text") {
		t.Fatalf("名为 primary 的字段不应被当成表级约束: %#v", byName["primary"])
	}
}

func TestFormatSQLServerTypeKeepsLength(t *testing.T) {
	if got := formatSQLServerType("nvarchar", 510, 0, 0); got != "nvarchar(255)" {
		t.Fatalf("nvarchar 长度应按字符计算: %s", got)
	}
	if got := formatSQLServerType("varchar", -1, 0, 0); got != "varchar(max)" {
		t.Fatalf("varchar(max): %s", got)
	}
	if got := formatSQLServerType("decimal", 0, 18, 2); got != "decimal(18,2)" {
		t.Fatalf("decimal: %s", got)
	}
	if got := formatSQLServerType("datetime2", 0, 0, 7); got != "datetime2(7)" {
		t.Fatalf("datetime2: %s", got)
	}
}

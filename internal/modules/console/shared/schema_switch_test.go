package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
)

func TestSchemaSwitchSQL(t *testing.T) {
	mysql, err := SchemaSwitchSQL("mysql", "a_demo")
	if err != nil || mysql != "use `a_demo`" {
		t.Fatalf("mysql: %s %v", mysql, err)
	}
	quoted, err := SchemaSwitchSQL("mysql", "a`demo")
	if err != nil || quoted != "use `a``demo`" {
		t.Fatalf("mysql 引号: %s %v", quoted, err)
	}
	postgres, err := SchemaSwitchSQL("pgsql", "archive")
	if err != nil || postgres != `set search_path to "archive"` {
		t.Fatalf("postgres: %s %v", postgres, err)
	}
	escaped, err := SchemaSwitchSQL("postgres", `arch"ive`)
	if err != nil || escaped != `set search_path to "arch""ive"` {
		t.Fatalf("postgres 引号: %s %v", escaped, err)
	}
	for _, item := range []struct{ driver, schema string }{
		{"sqlite", "main"},
		{"sqlserver", "dbo"},
		{"mssql", "sales"},
		{"mysql", "  "},
		{"postgres", ""},
	} {
		got, err := SchemaSwitchSQL(item.driver, item.schema)
		if err != nil || got != "" {
			t.Fatalf("%s %q 不应切换: %s %v", item.driver, item.schema, got, err)
		}
	}
	if _, err := SchemaSwitchSQL("mysql", "bad\x00name"); err == nil || !strings.Contains(err.Error(), "数据库名无效") {
		t.Fatalf("无效名称应拒绝: %v", err)
	}
}

func TestExecuteUsesSelectedMySQLDatabase(t *testing.T) {
	db := openSchemaDB(t)
	_, err := Execute(context.Background(), db, "mysql", "select 1", ExecOptions{ReadOnly: true, Schema: "a_demo"})
	if err == nil || !strings.Contains(err.Error(), "切换到选中的数据库失败") {
		t.Fatalf("应先执行 USE: %v", err)
	}
	_, err = Execute(context.Background(), db, "postgres", "select 1", ExecOptions{ReadOnly: true, Schema: "archive"})
	if err == nil || !strings.Contains(err.Error(), "切换到选中的数据库失败") {
		t.Fatalf("应先设置 search_path: %v", err)
	}
}

func TestExecuteLeavesSQLServerAndSQLiteDatabase(t *testing.T) {
	db := openSchemaDB(t)
	if _, err := db.Exec(`create table city (id integer primary key, name text)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into city (name) values ('杭州')`); err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"main", ""} {
		results, err := Execute(context.Background(), db, "sqlite", "select name from city", ExecOptions{ReadOnly: true, Schema: schema})
		if err != nil {
			t.Fatalf("sqlite %q: %v", schema, err)
		}
		if len(results) != 1 || len(results[0].Rows) != 1 || results[0].Rows[0][0] == nil || *results[0].Rows[0][0] != "杭州" {
			t.Fatalf("sqlite %q 结果不对: %#v", schema, results)
		}
	}
	_, err := Execute(context.Background(), db, "sqlserver", "select name from city", ExecOptions{ReadOnly: true, Schema: "dbo"})
	if err != nil && strings.Contains(err.Error(), "切换到选中的数据库失败") {
		t.Fatalf("SQL Server 的模式名不应被当成数据库切换: %v", err)
	}
}

func TestExecuteSwitchedKeepsTheSameConnection(t *testing.T) {
	db := openSchemaDB(t)
	db.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results, err := executeSwitched(ctx, db, "sqlite", []sqlscript.Part{
		{Text: "insert into marker (id) values (1)", Line: 1},
		{Text: "select count(*) from marker", Line: 2},
	}, ExecOptions{MaxRows: 10}, "create temp table marker (id integer)")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || len(results[1].Rows) != 1 || results[1].Rows[0][0] == nil || *results[1].Rows[0][0] != "1" {
		t.Fatalf("后续语句应看到同一条连接上的临时表: %#v", results)
	}
}

func openSchemaDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

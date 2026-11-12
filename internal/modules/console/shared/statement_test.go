package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteKeepsStatementText(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "statement.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`create table city (id integer primary key, name text)`); err != nil {
		t.Fatal(err)
	}
	results, err := Execute(context.Background(), db, "sqlite", "select id, name from city", ExecOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].SQL != "select id, name from city" {
		t.Fatalf("结果应带回原语句: %#v", results)
	}
}

func TestExecuteReportsFailingStatementLine(t *testing.T) {
	db := openStatementDB(t)
	_, err := Execute(context.Background(), db, "sqlite", "select 1;\nselect missing_column from city;", ExecOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("第二条语句应失败")
	}
	if ErrorLine(err) != 2 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
	message := PublicSQLError(err)
	if !strings.HasPrefix(message, "第 2 行：SQL 执行失败：") {
		t.Fatalf("提示 = %s", message)
	}
}

func TestExecuteKeepsLeadingBlankLines(t *testing.T) {
	db := openStatementDB(t)
	_, err := Execute(context.Background(), db, "sqlite", "\n\nselect missing_column from city;", ExecOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("语句应失败")
	}
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestExecuteReportsCommentedStatementLine(t *testing.T) {
	db := openStatementDB(t)
	_, err := Execute(context.Background(), db, "sqlite", "select 1;\n-- 下一行才是语句\nselect missing_column from city;", ExecOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("第三条语句应失败")
	}
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestExecuteReportsReadOnlyLine(t *testing.T) {
	db := openStatementDB(t)
	_, err := Execute(context.Background(), db, "sqlite", "select 1;\ndelete from city;", ExecOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("只读模式应拒绝删除")
	}
	if ErrorLine(err) != 2 || !strings.Contains(err.Error(), "只读模式只允许查询语句") {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func openStatementDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "statement.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`create table city (id integer primary key, name text)`); err != nil {
		t.Fatal(err)
	}
	return db
}

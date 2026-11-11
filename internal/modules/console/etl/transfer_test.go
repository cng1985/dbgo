package etl

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func TestValidateExtractRejectsWrites(t *testing.T) {
	ok, err := ValidateExtract("select id, name from city where note = 'into'")
	if err != nil || ok == "" {
		t.Fatalf("字符串里的 into 应保留: %v", err)
	}
	if _, err := ValidateExtract("-- into\nselect 1"); err != nil {
		t.Fatalf("注释里的 into 应忽略: %v", err)
	}
	if _, err := ValidateExtract("select id, name from city; delete from city"); err == nil {
		t.Fatal("多条语句应被拒绝")
	}
	if _, err := ValidateExtract("select * into copy from city"); err == nil {
		t.Fatal("select into 应被拒绝")
	}
	if _, err := ValidateExtract("delete from city"); err == nil {
		t.Fatal("写入语句应被拒绝")
	}
	if _, err := ValidateExtract("select * from city where load_extension('x') = 1"); err == nil {
		t.Fatal("load_extension 应被拒绝")
	}
}

func TestTransferMapsAndRollsBack(t *testing.T) {
	dir := t.TempDir()
	source := openSQLite(t, filepath.Join(dir, "source.db"))
	target := openSQLite(t, filepath.Join(dir, "target.db"))
	execSQL(t, source, `create table city (id integer primary key, name text)`)
	execSQL(t, source, `insert into city (id, name) values (1, '杭州'), (2, null), (3, '苏州')`)
	execSQL(t, target, `create table place (code integer primary key, title text)`)
	execSQL(t, target, `insert into place (code, title) values (9, '多余')`)

	outcome, err := Transfer(context.Background(), source, target, Spec{
		SourceSQL: "select id, name from city order by id",
		Driver:    "sqlite",
		Table:     "place",
		Mode:      "replace",
		Mappings:  []ColumnMap{{Source: "id", Target: "code"}, {Source: "Name", Target: "title"}},
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("替换写入失败: %v", err)
	}
	if outcome.RowsRead != 3 || outcome.RowsWritten != 3 {
		t.Fatalf("行数不对: %+v", outcome)
	}
	if countRows(t, target, `select count(*) from place where code = 9`) != 0 {
		t.Fatal("清空后写入应删掉原来的行")
	}
	if got := scalar(t, target, `select title from place where code = 2`); got != nil {
		t.Fatalf("空值应写成 NULL，实际 %#v", got)
	}
	if got := scalar(t, target, `select title from place where code = 1`); got == nil || *got != "杭州" {
		t.Fatalf("字段对应没有写到 title: %#v", got)
	}

	execSQL(t, target, `insert into place (code, title) values (8, '保留')`)
	_, err = Transfer(context.Background(), source, target, Spec{
		SourceSQL: "select id, name from city",
		Driver:    "sqlite",
		Table:     "place",
		Mode:      "replace",
		Mappings:  []ColumnMap{{Source: "id", Target: "missing"}},
	})
	if err == nil {
		t.Fatal("目标字段不存在时应失败")
	}
	if countRows(t, target, `select count(*) from place where code = 8`) != 1 {
		t.Fatal("写入失败后应回滚，原来的行还在")
	}
}

func TestTransferStopsBeforeWritingTooManyRows(t *testing.T) {
	dir := t.TempDir()
	source := openSQLite(t, filepath.Join(dir, "source.db"))
	target := openSQLite(t, filepath.Join(dir, "target.db"))
	execSQL(t, source, `create table city (id integer primary key, name text not null)`)
	execSQL(t, source, `insert into city (id, name) values (1, '杭州'), (2, '苏州')`)
	execSQL(t, target, `create table place (id integer primary key, name text not null)`)
	execSQL(t, target, `insert into place (id, name) values (7, '原有')`)
	_, err := Transfer(context.Background(), source, target, Spec{
		SourceSQL: "select id, name from city",
		Driver:    "sqlite",
		Table:     "place",
		Mode:      "replace",
		MaxRows:   1,
	})
	var app *apperrors.Error
	if !errorsAs(err, &app) || app.Message == "" {
		t.Fatalf("超限应返回明确错误: %v", err)
	}
	if got := scalar(t, target, `select name from place where id = 7`); got == nil || *got != "原有" {
		t.Fatalf("超限后不应改目标表: %#v", got)
	}
}

func TestPreviewStopsEarly(t *testing.T) {
	dir := t.TempDir()
	source := openSQLite(t, filepath.Join(dir, "source.db"))
	execSQL(t, source, `create table city (id integer primary key, name text not null)`)
	execSQL(t, source, `insert into city (id, name) values (1, '杭州'), (2, '苏州')`)
	outcome, err := Preview(context.Background(), source, "select name from city order by id", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Truncated || len(outcome.Rows) != 1 || outcome.Rows[0][0] == nil || *outcome.Rows[0][0] != "杭州" {
		t.Fatalf("预览应只返回限制行数: %+v", outcome)
	}
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func execSQL(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func scalar(t *testing.T, db *sql.DB, query string) *string {
	t.Helper()
	var value sql.NullString
	if err := db.QueryRow(query).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if !value.Valid {
		return nil
	}
	return &value.String
}

func errorsAs(err error, target **apperrors.Error) bool {
	found, ok := apperrors.As(err)
	if !ok {
		return false
	}
	*target = found
	return true
}

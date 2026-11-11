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

func TestTransferKeepsColumnCaseAndBlob(t *testing.T) {
	dir := t.TempDir()
	source := openSQLite(t, filepath.Join(dir, "source.db"))
	target := openSQLite(t, filepath.Join(dir, "target.db"))
	execSQL(t, source, `create table src (id integer primary key, payload blob)`)
	if _, err := source.Exec(`insert into src (id, payload) values (?, ?)`, 1, []byte{0, 1, 255}); err != nil {
		t.Fatal(err)
	}
	execSQL(t, target, `create table dst (low integer, high integer, payload blob) strict`)
	_, err := Transfer(context.Background(), source, target, Spec{
		SourceSQL: `select 1 as "foo", 2 as "FOO", payload from src`,
		Driver:    "sqlite",
		Table:     "dst",
		Mode:      "append",
		Mappings: []ColumnMap{
			{Source: "FOO", Target: "high"},
			{Source: "foo", Target: "low"},
			{Source: "payload", Target: "payload"},
		},
	})
	if err != nil {
		t.Fatalf("应按精确字段名写入: %v", err)
	}
	var low, high int
	var payload []byte
	var kind string
	if err := target.QueryRow(`select low, high, payload, typeof(payload) from dst`).Scan(&low, &high, &payload, &kind); err != nil {
		t.Fatal(err)
	}
	if low != 1 || high != 2 || kind != "blob" || string(payload) != string([]byte{0, 1, 255}) {
		t.Fatalf("列或二进制被写错: low=%d high=%d kind=%s payload=%v", low, high, kind, payload)
	}

	execSQL(t, target, `delete from dst`)
	execSQL(t, target, `insert into dst (low, high) values (7, 8)`)
	_, err = Transfer(context.Background(), source, target, Spec{
		SourceSQL: `select 1 as "foo", 2 as "FOO"`,
		Driver:    "sqlite",
		Table:     "dst",
		Mode:      "replace",
		Mappings:  []ColumnMap{{Source: "Foo", Target: "low"}},
	})
	if err == nil {
		t.Fatal("只差大小写的字段应拒绝，不能猜第一列")
	}
	if countRows(t, target, `select count(*) from dst where low = 7 and high = 8`) != 1 {
		t.Fatal("含糊字段不应改目标表")
	}
}

func TestBuildPlanKeepsDistinctCase(t *testing.T) {
	plan, err := buildPlan([]string{"foo", "FOO"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.indexes) != 2 || plan.quoted[0] != "foo" || plan.quoted[1] != "FOO" {
		t.Fatalf("未指定对应时不应丢掉只差大小写的列: %+v", plan)
	}
	if _, err := buildPlan([]string{"name"}, []ColumnMap{{Source: "Name", Target: "title"}}); err != nil {
		t.Fatalf("只有一个不区分大小写的列时应能对应: %v", err)
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

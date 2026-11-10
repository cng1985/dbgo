package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSuggestSQLite(t *testing.T) {
	db := openSuggestDB(t)
	if _, err := db.Exec(`create table city (
		id integer primary key,
		name text not null,
		population integer not null default 0
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create view city_names as select name from city`); err != nil {
		t.Fatal(err)
	}

	got, err := ListSuggest(context.Background(), db, "sqlite", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Truncated {
		t.Fatal("两张对象不应截断")
	}
	if len(got.Tables) != 2 {
		t.Fatalf("应有表和视图: %#v", got.Tables)
	}
	city := got.Tables[0]
	if city.Schema != "main" || city.Name != "city" || city.Type != "table" {
		t.Fatalf("city: %#v", city)
	}
	if len(city.Columns) != 3 || city.Columns[0].Name != "id" || city.Columns[1].Name != "name" || city.Columns[2].Name != "population" {
		t.Fatalf("字段顺序: %#v", city.Columns)
	}
	view := got.Tables[1]
	if view.Name != "city_names" || view.Type != "view" || len(view.Columns) != 1 || view.Columns[0].Name != "name" {
		t.Fatalf("视图字段: %#v", view)
	}

	limited, err := listSuggestSQLite(context.Background(), db, "main", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !limited.Truncated || len(limited.Tables) != 1 || limited.Tables[0].Name != "city" {
		t.Fatalf("应按表数量截断: %#v", limited)
	}
	columns, err := listSuggestSQLite(context.Background(), db, "main", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !columns.Truncated || len(columns.Tables[0].Columns) != 1 {
		t.Fatalf("应按字段数量截断: %#v", columns)
	}
	if _, err := ListSuggest(context.Background(), db, "oracle", "main"); err == nil {
		t.Fatal("未知驱动应失败")
	}
}

func TestSuggestRemoteQueryPlaceholders(t *testing.T) {
	postgres := suggestRemoteQuery("postgres")
	if !strings.Contains(postgres, "$1") || strings.Contains(postgres, "?") {
		t.Fatalf("postgres 应使用编号占位符: %s", postgres)
	}
	mysql := suggestRemoteQuery("mysql")
	if !strings.Contains(mysql, "?") || strings.Contains(mysql, "$1") {
		t.Fatalf("mysql 应保留问号占位符: %s", mysql)
	}
}

func openSuggestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "suggest.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	return db
}

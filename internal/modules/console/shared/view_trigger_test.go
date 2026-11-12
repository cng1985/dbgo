package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteViewTriggersSurviveReplace(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "view.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	setup := `
create table city(id integer primary key, name text);
create view city_names as select id, name from city;
create trigger city_names_ins instead of insert on city_names
begin
  insert into city(id, name) values (new.id, new.name);
end;
`
	if _, err := Execute(ctx, db, "sqlite", setup, ExecOptions{}); err != nil {
		t.Fatal(err)
	}
	triggers, err := SQLiteViewTriggers(ctx, db, "main", "city_names")
	if err != nil || len(triggers) != 1 || !strings.Contains(strings.ToLower(triggers[0]), "instead of insert") {
		t.Fatalf("应读到视图触发器: %v %#v", err, triggers)
	}
	replace := "drop view if exists city_names;\ncreate view city_names as select id, name from city;\n" + triggers[0]
	if _, err := Execute(ctx, db, "sqlite", replace, ExecOptions{SQLiteSchema: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into city_names(id, name) values (1, '杭州')`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from city`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("触发器没有跟着视图回来: %d %v", count, err)
	}
}

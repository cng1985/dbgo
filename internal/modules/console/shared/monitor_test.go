package shared

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCollectSQLiteSnapshot(t *testing.T) {
	db, err := Open(context.Background(), Endpoint{
		Driver:     "sqlite",
		SQLitePath: filepath.Join(t.TempDir(), "demo.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`create table city (id integer primary key, name text not null)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into city (name) values ('杭州')`); err != nil {
		t.Fatal(err)
	}
	snap, err := CollectSnapshot(context.Background(), db, "sqlite", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Metrics) < 3 {
		t.Fatalf("应读到页大小等指标: %#v", snap.Metrics)
	}
	found := false
	for _, size := range snap.Sizes {
		if size.Name == "city" && size.Rows == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("应统计 city 的行数: %#v", snap.Sizes)
	}
}

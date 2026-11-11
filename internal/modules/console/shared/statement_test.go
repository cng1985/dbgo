package shared

import (
	"context"
	"database/sql"
	"path/filepath"
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

package database

import "testing"

func TestRewritePlaceholders(t *testing.T) {
	got := rewritePlaceholders("sqlserver", "insert into schema_migration(version, name, checksum, applied_at) values (?, ?, ?, ?)")
	want := "insert into schema_migration(version, name, checksum, applied_at) values (@p1, @p2, @p3, @p4)"
	if got != want {
		t.Fatalf("sqlserver 占位符: %s", got)
	}
	postgres := rewritePlaceholders("postgres", "select '?' where id = ?")
	if postgres != "select '?' where id = $1" {
		t.Fatalf("postgres 占位符: %s", postgres)
	}
	if rewritePlaceholders("mysql", "select ?") != "select ?" {
		t.Fatal("mysql 不应改写占位符")
	}
}

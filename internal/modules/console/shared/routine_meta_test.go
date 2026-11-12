package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRoutineMetaSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "routine.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	objectID, description, err := RoutineMeta(context.Background(), db, "sqlite", "main", "save_order", "procedure")
	if err != nil || objectID != "" || description != "" {
		t.Fatalf("SQLite 没有存储过程编号和说明: %q %q %v", objectID, description, err)
	}
	viewID, viewText, err := RoutineMeta(context.Background(), db, "sqlite", "main", "city_names", "view")
	if err != nil || viewID != "" || viewText != "" {
		t.Fatalf("SQLite 视图没有编号和说明: %q %q %v", viewID, viewText, err)
	}
}

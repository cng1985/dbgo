package query

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/database"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

type fixedOpener struct {
	db *sql.DB
}

func (o fixedOpener) Open(context.Context, int64) (*shared.Target, error) {
	return &shared.Target{Driver: "sqlite", Name: "行号", DB: o.db}, nil
}

func TestExecuteKeepsLeadingBlankLines(t *testing.T) {
	target, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = target.Close() })
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := gdb.AutoMigrate(&QueryHistory{}); err != nil {
		t.Fatal(err)
	}
	resource := NewResource(NewRepository(&database.Handle{Gorm: gdb}), fixedOpener{db: target}, &config.Config{
		Server: config.ServerConfig{LongRequestTimeout: time.Minute},
		Query:  config.QueryConfig{MaxRows: 100, MaxSQLBytes: 100000},
	}, nil)
	raw := "\n\nselect missing_column from city;"
	readOnly := true
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1, RequestID: "line", Role: "admin"})
	_, err = resource.Execute(ctx, &QueryExecuteRequest{
		ConnectionID: 1,
		SQL:          raw,
		ReadOnly:     &readOnly,
		MaxRows:      10,
	})
	appErr, ok := apperrors.As(err)
	if !ok || !strings.HasPrefix(appErr.Message, "第 3 行：SQL 执行失败：") {
		t.Fatalf("提示 = %v", err)
	}
	var saved QueryHistory
	if err := gdb.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.SQLText != raw || !strings.HasPrefix(saved.ErrorMessage, "第 3 行：") {
		t.Fatalf("历史应保留原文和行号: %#v", saved)
	}
}

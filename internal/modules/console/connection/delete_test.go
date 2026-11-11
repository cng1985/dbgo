package connection

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cng1985/dbgo/internal/modules/console/migration"
	"github.com/cng1985/dbgo/internal/platform/database"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func TestDeleteRollsBackWhileMigrationBusy(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := gdb.AutoMigrate(&Connection{}, &migration.DataMigration{}); err != nil {
		t.Fatal(err)
	}
	handle := &database.Handle{Gorm: gdb, SQL: sqlDB, Driver: "sqlite", BatchSize: 100}
	repo := NewRepository(handle, migration.NewRepository(handle))
	now := time.Now().UTC()
	created, err := repo.Create(context.Background(), &Connection{
		Name: "演示", Driver: "sqlite", SQLitePath: "demo.db", Remark: "", Version: 1, CreatedBy: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&migration.DataMigration{
		ConnectionID: created.Data.ID, ConnectionName: "演示", VersionNo: 1, Name: "进行中",
		UpSQL: "select 1", Checksum: "abc", Status: "applying", CreatedBy: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = repo.Delete(context.Background(), 1, &ConnectionDeleteRequest{ID: created.Data.ID})
	appErr, ok := apperrors.As(err)
	if !ok || appErr.Kind != apperrors.KindConflict {
		t.Fatalf("执行中的迁移应阻止删除: %v", err)
	}
	var connections int64
	var migrations int64
	if err := gdb.Model(&Connection{}).Count(&connections).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&migration.DataMigration{}).Count(&migrations).Error; err != nil {
		t.Fatal(err)
	}
	if connections != 1 || migrations != 1 {
		t.Fatalf("失败后连接和迁移都应保留，实际连接 %d 迁移 %d", connections, migrations)
	}
	if err := gdb.Model(&migration.DataMigration{}).Where("connection_id = ?", created.Data.ID).Update("status", "draft").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Delete(context.Background(), 1, &ConnectionDeleteRequest{ID: created.Data.ID}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&Connection{}).Count(&connections).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&migration.DataMigration{}).Count(&migrations).Error; err != nil {
		t.Fatal(err)
	}
	if connections != 0 || migrations != 0 {
		t.Fatalf("删除应同时清掉迁移定义，实际连接 %d 迁移 %d", connections, migrations)
	}
}

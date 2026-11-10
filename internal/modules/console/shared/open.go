package shared

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/sqlite"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

type Target struct {
	Driver string
	Name   string
	Schema string
	DB     *sql.DB
}

func (t *Target) Close() {
	if t != nil && t.DB != nil {
		_ = t.DB.Close()
	}
}

type Opener interface {
	Open(ctx context.Context, connectionID int64) (*Target, error)
}

func DriverName(driver string) string {
	if NormalizeDriver(driver) == "postgres" {
		return "pgx"
	}
	return NormalizeDriver(driver)
}

func Open(ctx context.Context, endpoint Endpoint) (*sql.DB, error) {
	dsn, err := BuildDSN(endpoint)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(DriverName(endpoint.Driver), dsn)
	if err != nil {
		return nil, fmt.Errorf("打开目标数据库失败: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接目标数据库失败: %w", err)
	}
	return db, nil
}

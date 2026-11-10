package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/logging"
)

// Handle 是应用自身的元数据库连接。目标业务库不走这里。
type Handle struct {
	Gorm      *gorm.DB
	SQL       *sql.DB
	Driver    string
	BatchSize int
	ready     atomic.Bool
}

func Open(cfg *config.Config, log *logging.Logger) (*Handle, error) {
	dsn, err := appDSN(cfg)
	if err != nil {
		return nil, err
	}
	dialector, err := dialectorFor(cfg.Database.Driver, dsn)
	if err != nil {
		return nil, err
	}
	gormLog := gormlogger.New(slogAdapter{log: log}, gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	})
	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger:                 gormLog,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("打开应用数据库失败: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("应用数据库无法连接: %w", err)
	}
	if cfg.Database.Driver == "sqlite" {
		if err := applySQLite(ctx, sqlDB, cfg); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	return &Handle{
		Gorm:      gdb,
		SQL:       sqlDB,
		Driver:    cfg.Database.Driver,
		BatchSize: cfg.Database.BatchSize,
	}, nil
}

func (h *Handle) MarkReady() { h.ready.Store(true) }

func (h *Handle) MarkNotReady() { h.ready.Store(false) }

func (h *Handle) Ready(ctx context.Context) error {
	if h == nil || !h.ready.Load() {
		return fmt.Errorf("迁移尚未完成")
	}
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := h.SQL.PingContext(pingCtx); err != nil {
		return fmt.Errorf("数据库不可用")
	}
	return nil
}

func (h *Handle) Close() error {
	if h == nil || h.SQL == nil {
		return nil
	}
	return h.SQL.Close()
}

func appDSN(cfg *config.Config) (string, error) {
	if cfg.Database.Network() {
		return cfg.Database.DSN, nil
	}
	path := cfg.SQLiteFile()
	return sqliteDSN(path, cfg.Database.SQLite), nil
}

func sqliteDSN(path string, sqliteCfg config.SQLiteConfig) string {
	values := url.Values{}
	timeoutMS := sqliteCfg.BusyTimeout / time.Millisecond
	if timeoutMS < 1 {
		timeoutMS = 5000
	}
	fk := "0"
	if sqliteCfg.ForeignKeys {
		fk = "1"
	}
	mode := strings.ToUpper(sqliteCfg.JournalMode)
	if mode == "" {
		mode = "DELETE"
	}
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", timeoutMS))
	values.Add("_pragma", "foreign_keys("+fk+")")
	values.Add("_pragma", "journal_mode("+mode+")")
	return "file:" + path + "?" + values.Encode()
}

func applySQLite(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	fk := "off"
	if cfg.Database.SQLite.ForeignKeys {
		fk = "on"
	}
	statements := []string{
		"pragma foreign_keys = " + fk,
		fmt.Sprintf("pragma busy_timeout = %d", cfg.Database.SQLite.BusyTimeout/time.Millisecond),
		"pragma journal_mode = " + strings.ToUpper(cfg.Database.SQLite.JournalMode),
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("设置 sqlite 连接参数失败: %w", err)
		}
	}
	return nil
}

func dialectorFor(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "sqlite":
		return sqlite.Open(dsn), nil
	case "mysql":
		return mysql.Open(dsn), nil
	case "postgres":
		return postgres.Open(dsn), nil
	case "sqlserver":
		return sqlserver.Open(dsn), nil
	default:
		return nil, fmt.Errorf("未知数据库驱动 %s", driver)
	}
}

type slogAdapter struct {
	log *logging.Logger
}

func (s slogAdapter) Printf(format string, args ...any) {
	if s.log == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	if strings.Contains(strings.ToLower(message), "password") {
		message = "已省略可能包含凭据的数据库日志"
	}
	s.log.Warn(message)
}

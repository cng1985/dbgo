package config

import (
	"fmt"
	"strings"
)

func Validate(cfg *Config) error {
	if cfg.App.Name == "" {
		return fmt.Errorf("app.name 不能为空")
	}
	if err := validateProfile(cfg.App.Profile); err != nil {
		return err
	}
	if cfg.Server.Host == "" {
		return fmt.Errorf("server.host 不能为空")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	if cfg.Server.ReadHeaderTimeout <= 0 || cfg.Server.IdleTimeout <= 0 || cfg.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server 超时必须大于 0")
	}
	if cfg.Server.RequestTimeout <= 0 || cfg.Server.LongRequestTimeout <= 0 || cfg.Server.SSETimeout <= 0 {
		return fmt.Errorf("请求超时必须大于 0")
	}
	if cfg.Server.LongRequestTimeout < cfg.Server.RequestTimeout {
		return fmt.Errorf("server.long_request_timeout 不能短于 request_timeout")
	}
	if cfg.Server.MaxHeaderBytes < 1024 || cfg.Server.MaxBodyBytes < 1024 {
		return fmt.Errorf("请求头和请求体限制过小")
	}
	if cfg.Server.RatePerMinute < 1 || cfg.Server.RatePerMinute > 10000 {
		return fmt.Errorf("server.rate_per_minute 必须在 1 到 10000 之间")
	}
	if !cfg.Database.Enabled {
		return fmt.Errorf("database.enabled 不能为 false：控制台需要保存连接、会话和迁移记录")
	}
	switch cfg.Database.Driver {
	case "sqlite", "mysql", "postgres", "sqlserver":
	default:
		return fmt.Errorf("database.driver 不受支持: %s", cfg.Database.Driver)
	}
	if cfg.Database.Network() && strings.TrimSpace(cfg.Database.DSN) == "" {
		return fmt.Errorf("使用 %s 时必须通过 APP_DATABASE_DSN 提供连接串", cfg.Database.Driver)
	}
	if cfg.Database.Driver == "sqlite" && strings.TrimSpace(cfg.Database.DSN) != "" {
		return fmt.Errorf("sqlite 使用 database.sqlite.path，不能同时设置 database.dsn")
	}
	if cfg.Database.Driver == "sqlite" && strings.TrimSpace(cfg.Database.SQLite.Path) == "" {
		return fmt.Errorf("database.sqlite.path 不能为空")
	}
	if cfg.Database.MaxOpenConns < 1 || cfg.Database.MaxIdleConns < 0 || cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		return fmt.Errorf("数据库连接池参数无效")
	}
	if cfg.Database.ConnMaxLifetime <= 0 || cfg.Database.BatchSize < 1 || cfg.Database.BatchSize > 1000 {
		return fmt.Errorf("database.conn_max_lifetime 与 batch_size 无效")
	}
	switch strings.ToUpper(cfg.Database.SQLite.JournalMode) {
	case "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF":
	default:
		return fmt.Errorf("database.sqlite.journal_mode 不受支持")
	}
	if cfg.Database.SQLite.BusyTimeout <= 0 {
		return fmt.Errorf("database.sqlite.busy_timeout 必须大于 0")
	}
	if !cfg.Auth.Enabled {
		return fmt.Errorf("auth.enabled 不能为 false：数据库控制台必须先认证")
	}
	if cfg.Auth.SessionTTL <= 0 {
		return fmt.Errorf("auth.session_ttl 必须大于 0")
	}
	if cfg.Auth.AdminUsername == "" {
		return fmt.Errorf("auth.admin_username 不能为空")
	}
	if cfg.Auth.AdminPassword != "" && len(cfg.Auth.AdminPassword) < 8 {
		return fmt.Errorf("管理员密码至少 8 位")
	}
	if cfg.Auth.LoginRatePerMinute < 1 || cfg.Auth.LoginRatePerMinute > 100 {
		return fmt.Errorf("auth.login_rate_per_minute 必须在 1 到 100 之间")
	}
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level 只允许 debug、info、warn、error")
	}
	switch strings.ToLower(cfg.Logging.Format) {
	case "json", "text":
	default:
		return fmt.Errorf("logging.format 只允许 json 或 text")
	}
	if cfg.Logging.Level == "debug" && strings.EqualFold(cfg.App.Profile, "pro") {
		return fmt.Errorf("生产环境不能打开 debug 日志")
	}
	if cfg.Query.MaxRows < 1 || cfg.Query.MaxRows > 1000 {
		return fmt.Errorf("query.max_rows 必须在 1 到 1000 之间")
	}
	if cfg.Query.MaxSQLBytes < 1024 || cfg.Query.MaxSQLBytes > 2_000_000 {
		return fmt.Errorf("query.max_sql_bytes 超出允许范围")
	}
	if cfg.Backup.MaxRows < 1 || cfg.Backup.MaxBytes < 1024 {
		return fmt.Errorf("备份上限无效")
	}
	return nil
}

// SQLiteFile 返回应用库的绝对路径。相对路径相对 data-dir，不重复拼接 data。
func (c *Config) SQLiteFile() string {
	path := strings.TrimSpace(c.Database.SQLite.Path)
	if path == "" {
		return ""
	}
	return resolvePath(c.App.DataDir, path)
}

package config

import "time"

// Config 是启动后的有效配置。敏感字段只从环境变量或 --set 注入，不写入模板。
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	Query    QueryConfig    `yaml:"query"`
	Backup   BackupConfig   `yaml:"backup"`

	Sources []string `yaml:"-"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Profile   string `yaml:"profile"`
	BaseDir   string `yaml:"base_dir"`
	ConfigDir string `yaml:"config_dir"`
	DataDir   string `yaml:"data_dir"`
}

type ServerConfig struct {
	Host               string        `yaml:"host"`
	Port               int           `yaml:"port"`
	ReadHeaderTimeout  time.Duration `yaml:"read_header_timeout"`
	IdleTimeout        time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes     int           `yaml:"max_header_bytes"`
	ShutdownTimeout    time.Duration `yaml:"shutdown_timeout"`
	RequestTimeout     time.Duration `yaml:"request_timeout"`
	LongRequestTimeout time.Duration `yaml:"long_request_timeout"`
	SSETimeout         time.Duration `yaml:"sse_timeout"`
	MaxBodyBytes       int64         `yaml:"max_body_bytes"`
	TrustedProxies     []string      `yaml:"trusted_proxies"`
	RatePerMinute      int           `yaml:"rate_per_minute"`
}

type DatabaseConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Driver          string        `yaml:"driver"`
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	BatchSize       int           `yaml:"batch_size"`
	SQLite          SQLiteConfig  `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path        string        `yaml:"path"`
	ForeignKeys bool          `yaml:"foreign_keys"`
	BusyTimeout time.Duration `yaml:"busy_timeout"`
	JournalMode string        `yaml:"journal_mode"`
}

type AuthConfig struct {
	Enabled            bool          `yaml:"enabled"`
	SessionTTL         time.Duration `yaml:"session_ttl"`
	CookieSecure       bool          `yaml:"cookie_secure"`
	AdminUsername      string        `yaml:"admin_username"`
	AdminPassword      string        `yaml:"admin_password"`
	Secret             string        `yaml:"secret"`
	LoginRatePerMinute int           `yaml:"login_rate_per_minute"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type QueryConfig struct {
	MaxRows     int `yaml:"max_rows"`
	MaxSQLBytes int `yaml:"max_sql_bytes"`
}

type BackupConfig struct {
	MaxRows  int   `yaml:"max_rows"`
	MaxBytes int64 `yaml:"max_bytes"`
}

func (c DatabaseConfig) Network() bool {
	return c.Driver != "" && c.Driver != "sqlite"
}

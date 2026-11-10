package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Locator struct {
	BaseDir     string
	ConfigDir   string
	DataDir     string
	Profile     string
	ShowVersion bool
	Sets        []string
}

func ParseArgs(args []string) (Locator, error) {
	var loc Locator
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--version" || arg == "-version":
			loc.ShowVersion = true
		case arg == "--help" || arg == "-h":
			return Locator{}, errors.New("用法: dbgo [--base-dir DIR] [--config-dir DIR] [--data-dir DIR] [--profile dev|pro] [--set key=value] [--version]")
		case strings.HasPrefix(arg, "--base-dir="):
			loc.BaseDir = strings.TrimPrefix(arg, "--base-dir=")
		case arg == "--base-dir":
			value, err := takeArg(args, &i, arg)
			if err != nil {
				return Locator{}, err
			}
			loc.BaseDir = value
		case strings.HasPrefix(arg, "--config-dir="):
			loc.ConfigDir = strings.TrimPrefix(arg, "--config-dir=")
		case arg == "--config-dir":
			value, err := takeArg(args, &i, arg)
			if err != nil {
				return Locator{}, err
			}
			loc.ConfigDir = value
		case strings.HasPrefix(arg, "--data-dir="):
			loc.DataDir = strings.TrimPrefix(arg, "--data-dir=")
		case arg == "--data-dir":
			value, err := takeArg(args, &i, arg)
			if err != nil {
				return Locator{}, err
			}
			loc.DataDir = value
		case strings.HasPrefix(arg, "--profile="):
			loc.Profile = strings.TrimPrefix(arg, "--profile=")
		case arg == "--profile":
			value, err := takeArg(args, &i, arg)
			if err != nil {
				return Locator{}, err
			}
			loc.Profile = value
		case strings.HasPrefix(arg, "--set="):
			loc.Sets = append(loc.Sets, strings.TrimPrefix(arg, "--set="))
		case arg == "--set":
			value, err := takeArg(args, &i, arg)
			if err != nil {
				return Locator{}, err
			}
			loc.Sets = append(loc.Sets, value)
		default:
			return Locator{}, fmt.Errorf("无法识别的参数 %s", arg)
		}
	}
	return loc, nil
}

func takeArg(args []string, i *int, name string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("%s 缺少取值", name)
	}
	*i++
	return args[*i], nil
}

func Load(loc Locator) (*Config, error) {
	base, err := resolveBase(loc.BaseDir)
	if err != nil {
		return nil, err
	}
	profile := firstSet(loc.Profile, os.Getenv("APP_PROFILE"))
	if err := validateProfile(profile); err != nil {
		return nil, err
	}
	configDir := firstSet(loc.ConfigDir, os.Getenv("APP_CONFIG_DIR"))
	if configDir == "" {
		configDir = "configs"
	}
	configDir = resolvePath(base, configDir)

	sources := []string{}
	baseFile := filepath.Join(configDir, "application.yaml")
	root, err := readNode(baseFile)
	if err != nil {
		return nil, err
	}
	sources = append(sources, "file:"+baseFile)
	if profile != "" {
		profileFile := filepath.Join(configDir, "application-"+profile+".yaml")
		over, err := readNode(profileFile)
		if err != nil {
			return nil, fmt.Errorf("加载环境配置 %s 失败: %w", profileFile, err)
		}
		root = mergeNodes(root, over)
		sources = append(sources, "file:"+profileFile)
	}

	cfg := &Config{}
	if err := decodeNode(root, cfg); err != nil {
		return nil, err
	}
	if err := matchLocator(cfg, base, configDir, profile); err != nil {
		return nil, err
	}
	cfg.App.BaseDir = base
	cfg.App.ConfigDir = configDir
	cfg.App.Profile = profile

	dataDir := cfg.App.DataDir
	if value, ok := os.LookupEnv("APP_DATA_DIR"); ok {
		if value == "" {
			return nil, errors.New("APP_DATA_DIR 已设置但不能为空")
		}
		dataDir = value
		sources = append(sources, "env:APP_DATA_DIR")
	}
	if loc.DataDir != "" {
		dataDir = loc.DataDir
		sources = append(sources, "flag:data-dir")
	}
	if dataDir == "" {
		dataDir = "./data"
	}
	cfg.App.DataDir = resolvePath(base, dataDir)

	envSources, err := applyEnv(cfg)
	if err != nil {
		return nil, err
	}
	sources = append(sources, envSources...)
	flagSources, err := applySets(cfg, loc.Sets)
	if err != nil {
		return nil, err
	}
	sources = append(sources, flagSources...)

	cfg.App.BaseDir = base
	cfg.App.ConfigDir = configDir
	cfg.App.Profile = profile
	cfg.Database.Driver = normalizeDriver(cfg.Database.Driver)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.App.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	cfg.Sources = sources
	return cfg, nil
}

func resolveBase(raw string) (string, error) {
	if raw == "" {
		raw = os.Getenv("APP_BASE_DIR")
	}
	if raw == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("无法确定工作目录: %w", err)
		}
		raw = cwd
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("无法解析 base-dir: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("base-dir 不存在或不是目录: %s", abs)
	}
	return abs, nil
}

func resolvePath(base, raw string) string {
	if raw == "" {
		return base
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(base, raw))
}

func firstSet(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func validateProfile(profile string) error {
	switch profile {
	case "", "dev", "pro":
		return nil
	default:
		return fmt.Errorf("未知 profile %q，只允许 dev 或 pro", profile)
	}
}

func matchLocator(cfg *Config, base, configDir, profile string) error {
	if cfg.App.BaseDir != "" {
		got, err := filepath.Abs(cfg.App.BaseDir)
		if err != nil || filepath.Clean(got) != filepath.Clean(base) {
			return errors.New("application.yaml 中的 app.base_dir 与启动定位不一致")
		}
	}
	if cfg.App.ConfigDir != "" {
		got := resolvePath(base, cfg.App.ConfigDir)
		if filepath.Clean(got) != filepath.Clean(configDir) {
			return errors.New("application.yaml 中的 app.config_dir 与启动定位不一致")
		}
	}
	if cfg.App.Profile != "" && cfg.App.Profile != profile {
		return errors.New("application.yaml 中的 app.profile 与启动定位不一致")
	}
	return nil
}

func readNode(path string) (*yaml.Node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	}
	return node.Content[0], nil
}

func mergeNodes(base, over *yaml.Node) *yaml.Node {
	if over == nil {
		return base
	}
	if base == nil || base.Kind != yaml.MappingNode || over.Kind != yaml.MappingNode {
		return over
	}
	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	index := map[string]*yaml.Node{}
	for i := 0; i+1 < len(base.Content); i += 2 {
		key := base.Content[i]
		value := base.Content[i+1]
		merged.Content = append(merged.Content, key, value)
		index[key.Value] = value
	}
	for i := 0; i+1 < len(over.Content); i += 2 {
		key := over.Content[i]
		value := over.Content[i+1]
		if existing, ok := index[key.Value]; ok && existing.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			index[key.Value] = mergeNodes(existing, value)
			for j := 0; j+1 < len(merged.Content); j += 2 {
				if merged.Content[j].Value == key.Value {
					merged.Content[j+1] = index[key.Value]
				}
			}
			continue
		}
		if _, ok := index[key.Value]; ok {
			for j := 0; j+1 < len(merged.Content); j += 2 {
				if merged.Content[j].Value == key.Value {
					merged.Content[j+1] = value
				}
			}
			continue
		}
		merged.Content = append(merged.Content, key, value)
		index[key.Value] = value
	}
	return merged
}

func decodeNode(node *yaml.Node, cfg *Config) error {
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{node}}
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("整理配置失败: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("配置字段无效: %w", err)
	}
	return nil
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite":
		return "sqlite"
	case "mysql":
		return "mysql"
	case "postgres", "pgsql", "postgresql":
		return "postgres"
	case "sqlserver", "mssql":
		return "sqlserver"
	default:
		return driver
	}
}

func applyEnv(cfg *Config) ([]string, error) {
	var sources []string
	bindings := envBindings(cfg)
	for _, binding := range bindings {
		raw, ok := os.LookupEnv(binding.env)
		if !ok {
			continue
		}
		if raw == "" {
			if binding.sensitive {
				return nil, fmt.Errorf("%s 已设置但不能为空", binding.env)
			}
			if !binding.allowEmpty {
				return nil, fmt.Errorf("%s 已设置但不能为空", binding.env)
			}
		}
		if err := binding.apply(raw); err != nil {
			return nil, fmt.Errorf("%s: %w", binding.env, err)
		}
		sources = append(sources, "env:"+binding.env)
	}
	return sources, nil
}

type envBinding struct {
	env        string
	sensitive  bool
	allowEmpty bool
	apply      func(raw string) error
}

func envBindings(cfg *Config) []envBinding {
	return []envBinding{
		{env: "APP_NAME", apply: func(raw string) error { cfg.App.Name = raw; return nil }},
		{env: "APP_SERVER_HOST", apply: func(raw string) error { cfg.Server.Host = raw; return nil }},
		{env: "APP_SERVER_PORT", apply: func(raw string) error { return setInt(raw, &cfg.Server.Port) }},
		{env: "APP_SERVER_READ_HEADER_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.ReadHeaderTimeout) }},
		{env: "APP_SERVER_IDLE_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.IdleTimeout) }},
		{env: "APP_SERVER_MAX_HEADER_BYTES", apply: func(raw string) error { return setInt(raw, &cfg.Server.MaxHeaderBytes) }},
		{env: "APP_SERVER_SHUTDOWN_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.ShutdownTimeout) }},
		{env: "APP_SERVER_REQUEST_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.RequestTimeout) }},
		{env: "APP_SERVER_LONG_REQUEST_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.LongRequestTimeout) }},
		{env: "APP_SERVER_SSE_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Server.SSETimeout) }},
		{env: "APP_SERVER_MAX_BODY_BYTES", apply: func(raw string) error { return setInt64(raw, &cfg.Server.MaxBodyBytes) }},
		{env: "APP_SERVER_TRUSTED_PROXIES", apply: func(raw string) error { return setStringList(raw, &cfg.Server.TrustedProxies) }},
		{env: "APP_SERVER_RATE_PER_MINUTE", apply: func(raw string) error { return setInt(raw, &cfg.Server.RatePerMinute) }},
		{env: "APP_DATABASE_ENABLED", apply: func(raw string) error { return setBool(raw, &cfg.Database.Enabled) }},
		{env: "APP_DATABASE_DRIVER", apply: func(raw string) error { cfg.Database.Driver = normalizeDriver(raw); return nil }},
		{env: "APP_DATABASE_DSN", sensitive: true, apply: func(raw string) error { cfg.Database.DSN = raw; return nil }},
		{env: "APP_DATABASE_SQLITE_PATH", apply: func(raw string) error { cfg.Database.SQLite.Path = raw; return nil }},
		{env: "APP_DATABASE_SQLITE_FOREIGN_KEYS", apply: func(raw string) error { return setBool(raw, &cfg.Database.SQLite.ForeignKeys) }},
		{env: "APP_DATABASE_SQLITE_BUSY_TIMEOUT", apply: func(raw string) error { return setDuration(raw, &cfg.Database.SQLite.BusyTimeout) }},
		{env: "APP_DATABASE_SQLITE_JOURNAL_MODE", apply: func(raw string) error { cfg.Database.SQLite.JournalMode = raw; return nil }},
		{env: "APP_DATABASE_MAX_OPEN_CONNS", apply: func(raw string) error { return setInt(raw, &cfg.Database.MaxOpenConns) }},
		{env: "APP_DATABASE_MAX_IDLE_CONNS", apply: func(raw string) error { return setInt(raw, &cfg.Database.MaxIdleConns) }},
		{env: "APP_DATABASE_CONN_MAX_LIFETIME", apply: func(raw string) error { return setDuration(raw, &cfg.Database.ConnMaxLifetime) }},
		{env: "APP_DATABASE_BATCH_SIZE", apply: func(raw string) error { return setInt(raw, &cfg.Database.BatchSize) }},
		{env: "APP_AUTH_ENABLED", apply: func(raw string) error { return setBool(raw, &cfg.Auth.Enabled) }},
		{env: "APP_AUTH_SESSION_TTL", apply: func(raw string) error { return setDuration(raw, &cfg.Auth.SessionTTL) }},
		{env: "APP_AUTH_COOKIE_SECURE", apply: func(raw string) error { return setBool(raw, &cfg.Auth.CookieSecure) }},
		{env: "APP_AUTH_ADMIN_USERNAME", apply: func(raw string) error { cfg.Auth.AdminUsername = raw; return nil }},
		{env: "APP_AUTH_ADMIN_PASSWORD", sensitive: true, apply: func(raw string) error { cfg.Auth.AdminPassword = raw; return nil }},
		{env: "APP_AUTH_SECRET", sensitive: true, apply: func(raw string) error { cfg.Auth.Secret = raw; return nil }},
		{env: "APP_AUTH_LOGIN_RATE_PER_MINUTE", apply: func(raw string) error { return setInt(raw, &cfg.Auth.LoginRatePerMinute) }},
		{env: "APP_LOGGING_LEVEL", apply: func(raw string) error { cfg.Logging.Level = raw; return nil }},
		{env: "APP_LOGGING_FORMAT", apply: func(raw string) error { cfg.Logging.Format = raw; return nil }},
		{env: "APP_QUERY_MAX_ROWS", apply: func(raw string) error { return setInt(raw, &cfg.Query.MaxRows) }},
		{env: "APP_QUERY_MAX_SQL_BYTES", apply: func(raw string) error { return setInt(raw, &cfg.Query.MaxSQLBytes) }},
		{env: "APP_BACKUP_MAX_ROWS", apply: func(raw string) error { return setInt(raw, &cfg.Backup.MaxRows) }},
		{env: "APP_BACKUP_MAX_BYTES", apply: func(raw string) error { return setInt64(raw, &cfg.Backup.MaxBytes) }},
	}
}

func applySets(cfg *Config, sets []string) ([]string, error) {
	if len(sets) == 0 {
		return nil, nil
	}
	var sources []string
	for _, item := range sets {
		key, raw, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("--set 需要 key=value，收到 %q", item)
		}
		key = strings.TrimSpace(key)
		binding, ok := setBinding(cfg, key)
		if !ok {
			return nil, fmt.Errorf("不支持从命令行覆盖 %s", key)
		}
		if raw == "" && binding.sensitive {
			return nil, fmt.Errorf("%s 不能显式设置为空", key)
		}
		if err := binding.apply(raw); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		sources = append(sources, "flag:"+key)
	}
	return sources, nil
}

func setBinding(cfg *Config, key string) (envBinding, bool) {
	for _, binding := range envBindings(cfg) {
		if envKeyToPath(binding.env) == key {
			return binding, true
		}
	}
	switch key {
	case "app.data_dir":
		return envBinding{apply: func(raw string) error {
			if raw == "" {
				return errors.New("不能为空")
			}
			cfg.App.DataDir = resolvePath(cfg.App.BaseDir, raw)
			return nil
		}}, true
	default:
		return envBinding{}, false
	}
}

func envKeyToPath(env string) string {
	mapping := map[string]string{
		"APP_NAME":                         "app.name",
		"APP_SERVER_HOST":                  "server.host",
		"APP_SERVER_PORT":                  "server.port",
		"APP_SERVER_READ_HEADER_TIMEOUT":   "server.read_header_timeout",
		"APP_SERVER_IDLE_TIMEOUT":          "server.idle_timeout",
		"APP_SERVER_MAX_HEADER_BYTES":      "server.max_header_bytes",
		"APP_SERVER_SHUTDOWN_TIMEOUT":      "server.shutdown_timeout",
		"APP_SERVER_REQUEST_TIMEOUT":       "server.request_timeout",
		"APP_SERVER_LONG_REQUEST_TIMEOUT":  "server.long_request_timeout",
		"APP_SERVER_SSE_TIMEOUT":           "server.sse_timeout",
		"APP_SERVER_MAX_BODY_BYTES":        "server.max_body_bytes",
		"APP_SERVER_TRUSTED_PROXIES":       "server.trusted_proxies",
		"APP_SERVER_RATE_PER_MINUTE":       "server.rate_per_minute",
		"APP_DATABASE_ENABLED":             "database.enabled",
		"APP_DATABASE_DRIVER":              "database.driver",
		"APP_DATABASE_DSN":                 "database.dsn",
		"APP_DATABASE_SQLITE_PATH":         "database.sqlite.path",
		"APP_DATABASE_SQLITE_FOREIGN_KEYS": "database.sqlite.foreign_keys",
		"APP_DATABASE_SQLITE_BUSY_TIMEOUT": "database.sqlite.busy_timeout",
		"APP_DATABASE_SQLITE_JOURNAL_MODE": "database.sqlite.journal_mode",
		"APP_DATABASE_MAX_OPEN_CONNS":      "database.max_open_conns",
		"APP_DATABASE_MAX_IDLE_CONNS":      "database.max_idle_conns",
		"APP_DATABASE_CONN_MAX_LIFETIME":   "database.conn_max_lifetime",
		"APP_DATABASE_BATCH_SIZE":          "database.batch_size",
		"APP_AUTH_ENABLED":                 "auth.enabled",
		"APP_AUTH_SESSION_TTL":             "auth.session_ttl",
		"APP_AUTH_COOKIE_SECURE":           "auth.cookie_secure",
		"APP_AUTH_ADMIN_USERNAME":          "auth.admin_username",
		"APP_AUTH_ADMIN_PASSWORD":          "auth.admin_password",
		"APP_AUTH_SECRET":                  "auth.secret",
		"APP_AUTH_LOGIN_RATE_PER_MINUTE":   "auth.login_rate_per_minute",
		"APP_LOGGING_LEVEL":                "logging.level",
		"APP_LOGGING_FORMAT":               "logging.format",
		"APP_QUERY_MAX_ROWS":               "query.max_rows",
		"APP_QUERY_MAX_SQL_BYTES":          "query.max_sql_bytes",
		"APP_BACKUP_MAX_ROWS":              "backup.max_rows",
		"APP_BACKUP_MAX_BYTES":             "backup.max_bytes",
	}
	return mapping[env]
}

func setBool(raw string, dst *bool) error {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		*dst = true
	case "false":
		*dst = false
	default:
		return errors.New("布尔值只接受 true 或 false")
	}
	return nil
}

func setInt(raw string, dst *int) error {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("必须是整数")
	}
	*dst = value
	return nil
}

func setInt64(raw string, dst *int64) error {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return errors.New("必须是整数")
	}
	*dst = value
	return nil
}

func setDuration(raw string, dst *time.Duration) error {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("时长必须带单位，例如 30s 或 5m")
	}
	*dst = value
	return nil
}

func setStringList(raw string, dst *[]string) error {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return errors.New("列表必须是 JSON 数组字符串")
	}
	*dst = values
	return nil
}

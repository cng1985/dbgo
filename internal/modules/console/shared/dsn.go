package shared

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

type Endpoint struct {
	Driver     string
	Host       string
	Port       int
	Database   string
	Username   string
	Password   string
	SQLitePath string
	SSLMode    string
}

func BuildDSN(endpoint Endpoint) (string, error) {
	driver := NormalizeDriver(endpoint.Driver)
	switch driver {
	case "sqlite":
		if endpoint.SQLitePath == "" {
			return "", fmt.Errorf("sqlite 路径为空")
		}
		values := url.Values{}
		values.Add("_pragma", "busy_timeout(5000)")
		values.Add("_pragma", "foreign_keys(1)")
		return "file:" + endpoint.SQLitePath + "?" + values.Encode(), nil
	case "mysql":
		port := endpoint.Port
		if port == 0 {
			port = DefaultPort(driver)
		}
		cfg := mysql.Config{
			User:                 endpoint.Username,
			Passwd:               endpoint.Password,
			Net:                  "tcp",
			Addr:                 net.JoinHostPort(endpoint.Host, strconv.Itoa(port)),
			DBName:               endpoint.Database,
			ParseTime:            true,
			Loc:                  time.UTC,
			Timeout:              5 * time.Second,
			ReadTimeout:          60 * time.Second,
			WriteTimeout:         60 * time.Second,
			AllowNativePasswords: true,
			Params:               map[string]string{"charset": "utf8mb4"},
		}
		return cfg.FormatDSN(), nil
	case "postgres":
		port := endpoint.Port
		if port == 0 {
			port = DefaultPort(driver)
		}
		sslMode := endpoint.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(endpoint.Username, endpoint.Password),
			Host:   net.JoinHostPort(endpoint.Host, strconv.Itoa(port)),
			Path:   "/" + endpoint.Database,
		}
		query := u.Query()
		query.Set("sslmode", sslMode)
		query.Set("connect_timeout", "5")
		u.RawQuery = query.Encode()
		return u.String(), nil
	case "sqlserver":
		port := endpoint.Port
		if port == 0 {
			port = DefaultPort(driver)
		}
		query := url.Values{}
		query.Set("database", endpoint.Database)
		query.Set("connection timeout", "5")
		u := &url.URL{
			Scheme:   "sqlserver",
			User:     url.UserPassword(endpoint.Username, endpoint.Password),
			Host:     net.JoinHostPort(endpoint.Host, strconv.Itoa(port)),
			RawQuery: query.Encode(),
		}
		return u.String(), nil
	default:
		return "", fmt.Errorf("不支持的数据库类型")
	}
}

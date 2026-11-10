package connection

import (
	"path/filepath"
	"strings"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type normalizedConnection struct {
	Name         string
	Driver       string
	Host         string
	Port         int
	DatabaseName string
	Username     string
	Password     string
	SQLitePath   string
	SSLMode      string
	Remark       string
}

func normalizeConnection(cfg *config.Config, request *ConnectionRequest, keepPassword bool) (normalizedConnection, error) {
	var zero normalizedConnection
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	name, err := response.TrimRequired(request.Name, "连接名称", 80)
	if err != nil {
		return zero, err
	}
	driver := shared.NormalizeDriver(request.Driver)
	if driver == "" {
		return zero, apperrors.Invalid("数据库类型只支持 sqlite、mysql、postgres、sqlserver")
	}
	remark, err := response.TrimLimit(request.Remark, "备注", 500)
	if err != nil {
		return zero, err
	}
	item := normalizedConnection{Name: name, Driver: driver, Remark: remark, Password: request.Password}
	if !keepPassword {
		item.Password = strings.TrimSpace(request.Password)
	}
	if driver == "sqlite" {
		path, err := resolveSQLitePath(cfg, request.SQLitePath)
		if err != nil {
			return zero, err
		}
		item.SQLitePath = path
		return item, nil
	}
	host, err := response.TrimRequired(request.Host, "主机", 255)
	if err != nil {
		return zero, err
	}
	if strings.ContainsAny(host, " /\\") {
		return zero, apperrors.Invalid("主机格式不正确")
	}
	databaseName, err := response.TrimRequired(request.DatabaseName, "数据库名", 128)
	if err != nil {
		return zero, err
	}
	username, err := response.TrimRequired(request.Username, "用户名", 128)
	if err != nil {
		return zero, err
	}
	port := request.Port
	if port == 0 {
		port = shared.DefaultPort(driver)
	}
	if port < 1 || port > 65535 {
		return zero, apperrors.Invalid("端口必须在 1 到 65535 之间")
	}
	sslMode := ""
	if driver == "postgres" {
		sslMode = strings.ToLower(strings.TrimSpace(request.SSLMode))
		if sslMode == "" {
			sslMode = "disable"
		}
		switch sslMode {
		case "disable", "require", "verify-ca", "verify-full":
		default:
			return zero, apperrors.Invalid("ssl_mode 只支持 disable、require、verify-ca、verify-full")
		}
	} else if strings.TrimSpace(request.SSLMode) != "" {
		return zero, apperrors.Invalid("只有 postgres 可以设置 ssl_mode")
	}
	item.Host = host
	item.Port = port
	item.DatabaseName = databaseName
	item.Username = username
	item.SSLMode = sslMode
	return item, nil
}

func resolveSQLitePath(cfg *config.Config, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperrors.Invalid("sqlite 路径不能为空")
	}
	if strings.ContainsAny(raw, "\x00?#") {
		return "", apperrors.Invalid("sqlite 路径包含不允许的字符")
	}
	cleaned := filepath.Clean(raw)
	var absolute string
	if filepath.IsAbs(cleaned) {
		absolute = cleaned
	} else {
		if strings.HasPrefix(cleaned, "..") {
			return "", apperrors.Invalid("sqlite 相对路径必须位于数据目录内")
		}
		absolute = filepath.Clean(filepath.Join(cfg.App.DataDir, cleaned))
		relative, err := filepath.Rel(cfg.App.DataDir, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", apperrors.Invalid("sqlite 相对路径必须位于数据目录内")
		}
	}
	if filepath.Clean(absolute) == filepath.Clean(cfg.SQLiteFile()) {
		return "", apperrors.Invalid("不能把应用库登记为业务连接")
	}
	return absolute, nil
}

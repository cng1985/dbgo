package shared

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func NormalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite":
		return "sqlite"
	case "mysql":
		return "mysql"
	case "postgres", "pgsql", "postgresql":
		return "postgres"
	case "sqlserver", "mssql":
		return "sqlserver"
	default:
		return ""
	}
}

func DefaultPort(driver string) int {
	switch driver {
	case "mysql":
		return 3306
	case "postgres":
		return 5432
	case "sqlserver":
		return 1433
	default:
		return 0
	}
}

func QuoteIdent(driver, name string) (string, error) {
	if strings.TrimSpace(name) == "" || strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) > 128 {
		return "", fmt.Errorf("标识符无效")
	}
	switch driver {
	case "mysql":
		return "`" + strings.ReplaceAll(name, "`", "``") + "`", nil
	case "sqlserver":
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]", nil
	default:
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`, nil
	}
}

func Qualified(driver, schema, name string) (string, error) {
	quoted, err := QuoteIdent(driver, name)
	if err != nil {
		return "", err
	}
	if schema == "" || driver == "sqlite" {
		return quoted, nil
	}
	quotedSchema, err := QuoteIdent(driver, schema)
	if err != nil {
		return "", err
	}
	return quotedSchema + "." + quoted, nil
}

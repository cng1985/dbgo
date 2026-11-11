package shared

import (
	"fmt"
	"strings"
)

func formatSQLServerType(name string, maxLength, precision, scale int) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "varchar", "char", "varbinary", "binary":
		if maxLength < 0 {
			return name + "(max)"
		}
		if maxLength <= 0 {
			return name
		}
		return fmt.Sprintf("%s(%d)", name, maxLength)
	case "nvarchar", "nchar":
		if maxLength < 0 {
			return name + "(max)"
		}
		if maxLength <= 0 {
			return name
		}
		length := maxLength / 2
		if length < 1 {
			length = 1
		}
		return fmt.Sprintf("%s(%d)", name, length)
	case "decimal", "numeric":
		if precision <= 0 {
			return name
		}
		if scale < 0 {
			scale = 0
		}
		return fmt.Sprintf("%s(%d,%d)", name, precision, scale)
	case "float":
		if precision > 0 && precision < 53 {
			return fmt.Sprintf("float(%d)", precision)
		}
		return "float"
	case "time", "datetime2", "datetimeoffset":
		if scale < 0 {
			return name
		}
		return fmt.Sprintf("%s(%d)", name, scale)
	default:
		if name == "" {
			return "nvarchar(max)"
		}
		return name
	}
}

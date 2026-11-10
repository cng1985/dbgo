package shared

import (
	"strconv"
	"strings"
	"unicode"
)

func RewritePlaceholders(driver, query string) string {
	if driver != "postgres" {
		return query
	}
	var builder strings.Builder
	number := 1
	inQuote := false
	for _, ch := range query {
		if ch == '\'' {
			inQuote = !inQuote
		}
		if ch == '?' && !inQuote {
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(number))
			number++
			continue
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}

func FirstToken(statement string) string {
	text := stripLeadingComments(statement)
	fields := strings.FieldsFunc(text, unicode.IsSpace)
	if len(fields) == 0 {
		return ""
	}
	token := strings.Trim(fields[0], "`\"[];")
	return strings.ToLower(token)
}

func stripLeadingComments(statement string) string {
	text := strings.TrimSpace(statement)
	for {
		switch {
		case strings.HasPrefix(text, "--"):
			if index := strings.IndexByte(text, '\n'); index >= 0 {
				text = strings.TrimSpace(text[index+1:])
				continue
			}
			return ""
		case strings.HasPrefix(text, "/*"):
			if index := strings.Index(text, "*/"); index >= 0 {
				text = strings.TrimSpace(text[index+2:])
				continue
			}
			return ""
		default:
			return text
		}
	}
}

func ReturnsRows(statement string) bool {
	switch FirstToken(statement) {
	case "select", "with", "show", "explain", "pragma", "values", "describe", "desc", "table":
		return true
	case "insert", "update", "delete":
		return strings.Contains(strings.ToLower(statement), " returning ")
	default:
		return false
	}
}

func ReadOnlyToken(token string) bool {
	switch token {
	case "select", "with", "show", "explain", "pragma", "values", "describe", "desc", "table":
		return true
	default:
		return false
	}
}

func Forbidden(statement string) bool {
	token := FirstToken(statement)
	switch token {
	case "load_extension", "xp_cmdshell", "sp_oacreate", "sp_oamethod":
		return true
	}
	lower := strings.ToLower(statement)
	if strings.Contains(lower, "load_extension") || strings.Contains(lower, "from program") {
		return true
	}
	return false
}

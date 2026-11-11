package etl

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/sqlscript"
)

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) > 80 {
		return "", apperrors.Invalid("任务名称需要 1 到 80 个字符")
	}
	return name, nil
}

func cleanMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "append":
		return "append", nil
	case "replace":
		return "replace", nil
	default:
		return "", apperrors.Invalid("写入方式只支持追加或清空后写入")
	}
}

// ValidateExtract 只接受一条 SELECT 或 VALUES。转换写在这条语句里，不能带 INTO。
func ValidateExtract(sqlText string) (string, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return "", apperrors.Invalid("请填写抽取语句")
	}
	if len(sqlText) > maxSQLLength {
		return "", apperrors.Invalid("抽取语句过长")
	}
	parts := sqlscript.Split(sqlText)
	if len(parts) != 1 {
		return "", apperrors.Invalid("抽取语句只能有一条")
	}
	statement := parts[0]
	token := shared.FirstToken(statement)
	if token != "select" && token != "values" {
		return "", apperrors.Invalid("抽取语句只支持单条 SELECT 或 VALUES")
	}
	if shared.Forbidden(statement) || containsIdent(statement, "into") {
		return "", apperrors.Invalid("抽取语句不能写入或导出文件")
	}
	return statement, nil
}

func normalizeMappings(items []ColumnMap) ([]ColumnMap, string, error) {
	if len(items) > maxMappings {
		return nil, "", apperrors.Invalid("字段对应不能超过 64 个")
	}
	cleaned := make([]ColumnMap, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		source := strings.TrimSpace(item.Source)
		target := strings.TrimSpace(item.Target)
		if source == "" || target == "" {
			return nil, "", apperrors.Invalid("字段对应需要同时填写来源和目标")
		}
		if strings.ContainsRune(source, 0) || strings.ContainsRune(target, 0) || utf8.RuneCountInString(source) > 128 || utf8.RuneCountInString(target) > 128 {
			return nil, "", apperrors.Invalid("字段名无效")
		}
		key := strings.ToLower(target)
		if _, ok := seen[key]; ok {
			return nil, "", apperrors.Invalid("目标字段 " + target + " 重复")
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, ColumnMap{Source: source, Target: target})
	}
	raw, err := json.Marshal(cleaned)
	if err != nil {
		return nil, "", err
	}
	return cleaned, string(raw), nil
}

func checkTable(driver, schema, table string) error {
	driver = shared.NormalizeDriver(driver)
	if driver == "" {
		return apperrors.Invalid("目标连接的数据库类型不受支持")
	}
	table = strings.TrimSpace(table)
	schema = strings.TrimSpace(schema)
	if _, err := shared.QuoteIdent(driver, table); err != nil {
		return apperrors.Invalid("目标表名无效")
	}
	if schema != "" {
		if _, err := shared.QuoteIdent(driver, schema); err != nil {
			return apperrors.Invalid("目标模式名无效")
		}
	}
	return nil
}

func containsIdent(sqlText, want string) bool {
	want = strings.ToLower(want)
	runes := []rune(sqlText)
	for i := 0; i < len(runes); {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch {
		case ch == '-' && next == '-':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
		case ch == '/' && next == '*':
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i += 2
		case ch == '\'':
			i = skipQuote(runes, i, '\'')
		case ch == '"':
			i = skipQuote(runes, i, '"')
		case ch == '`':
			i = skipQuote(runes, i, '`')
		case ch == '[':
			i = skipBracket(runes, i)
		case ch == '$':
			if next := skipDollar(runes, i); next > i {
				i = next
				continue
			}
			i++
		case ch == '_' || unicode.IsLetter(ch):
			end := i + 1
			for end < len(runes) && (runes[end] == '_' || unicode.IsLetter(runes[end]) || unicode.IsDigit(runes[end])) {
				end++
			}
			if strings.ToLower(string(runes[i:end])) == want {
				return true
			}
			i = end
		default:
			i++
		}
	}
	return false
}

func skipQuote(runes []rune, i int, quote rune) int {
	i++
	for i < len(runes) {
		if runes[i] == quote && i+1 < len(runes) && runes[i+1] == quote {
			i += 2
			continue
		}
		if runes[i] == quote {
			return i + 1
		}
		i++
	}
	return len(runes)
}

func skipBracket(runes []rune, i int) int {
	i++
	for i < len(runes) {
		if runes[i] == ']' && i+1 < len(runes) && runes[i+1] == ']' {
			i += 2
			continue
		}
		if runes[i] == ']' {
			return i + 1
		}
		i++
	}
	return len(runes)
}

func skipDollar(runes []rune, i int) int {
	j := i + 1
	for j < len(runes) && (runes[j] == '_' || unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
		j++
	}
	if j >= len(runes) || runes[j] != '$' {
		return i
	}
	tag := string(runes[i : j+1])
	rest := j + 1
	for rest+len(tag) <= len(runes) {
		if string(runes[rest:rest+len(tag)]) == tag {
			return rest + len(tag)
		}
		rest++
	}
	return len(runes)
}

// Package sqlscript 按语句切分受控 SQL 文本，供迁移和目标库执行使用。
package sqlscript

import "strings"

// Split 按分号切分语句，保留引号、括号标识符和注释中的分号。
func Split(sqlText string) []string {
	var (
		statements []string
		buf        strings.Builder
		inSingle   bool
		inDouble   bool
		inBacktick bool
		inBracket  bool
		inLine     bool
		inBlock    bool
	)
	flush := func() {
		statement := strings.TrimSpace(buf.String())
		buf.Reset()
		if statement != "" {
			statements = append(statements, statement)
		}
	}
	runes := []rune(sqlText)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLine {
			buf.WriteRune(ch)
			if ch == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			buf.WriteRune(ch)
			if ch == '*' && next == '/' {
				buf.WriteRune(next)
				i++
				inBlock = false
			}
			continue
		}
		if inSingle {
			buf.WriteRune(ch)
			if ch == '\'' && next == '\'' {
				buf.WriteRune(next)
				i++
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			buf.WriteRune(ch)
			if ch == '"' && next == '"' {
				buf.WriteRune(next)
				i++
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			buf.WriteRune(ch)
			if ch == '`' && next == '`' {
				buf.WriteRune(next)
				i++
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			buf.WriteRune(ch)
			if ch == ']' && next == ']' {
				buf.WriteRune(next)
				i++
				continue
			}
			if ch == ']' {
				inBracket = false
			}
			continue
		}
		switch ch {
		case '-':
			if next == '-' {
				buf.WriteRune(ch)
				buf.WriteRune(next)
				i++
				inLine = true
				continue
			}
		case '/':
			if next == '*' {
				buf.WriteRune(ch)
				buf.WriteRune(next)
				i++
				inBlock = true
				continue
			}
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case '[':
			inBracket = true
		case ';':
			flush()
			continue
		}
		buf.WriteRune(ch)
	}
	flush()
	return statements
}

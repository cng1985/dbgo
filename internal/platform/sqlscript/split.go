// Package sqlscript 按语句切分受控 SQL 文本，供迁移和目标库执行使用。
package sqlscript

import (
	"strings"
	"unicode"
)

// Split 按分号切分语句，保留引号、括号标识符、注释、美元引号和 BEGIN/END、CASE/END 块中的分号。
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
		depth      int
		dollar     string
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
		if dollar != "" {
			if hasPrefixRunes(runes, i, dollar) {
				buf.WriteString(dollar)
				i += len([]rune(dollar)) - 1
				dollar = ""
				continue
			}
			buf.WriteRune(ch)
			continue
		}
		if ch == '$' {
			if tag := dollarTag(runes, i); tag != "" {
				buf.WriteString(tag)
				i += len([]rune(tag)) - 1
				dollar = tag
				continue
			}
		}
		if isIdentStart(ch) {
			word, end := readWord(runes, i)
			switch strings.ToLower(word) {
			case "begin":
				if beginsBlock(runes, end) {
					depth++
				}
			case "case":
				if !prevIsDot(runes, i) && adjacentWord(runes, i, false) != "end" {
					depth++
				}
			case "end":
				if depth > 0 && !prevIsDot(runes, i) && closesBlock(runes, end) {
					depth--
				}
			}
			buf.WriteString(word)
			i = end - 1
			continue
		}
		if ch == ';' && depth == 0 {
			flush()
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
		}
		buf.WriteRune(ch)
	}
	flush()
	return statements
}

func isIdentStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isIdent(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

func readWord(runes []rune, i int) (string, int) {
	end := i + 1
	for end < len(runes) && isIdent(runes[end]) {
		end++
	}
	return string(runes[i:end]), end
}

func prevIsDot(runes []rune, i int) bool {
	j := i - 1
	for j >= 0 && unicode.IsSpace(runes[j]) {
		j--
	}
	return j >= 0 && runes[j] == '.'
}

func dollarTag(runes []rune, i int) string {
	if i >= len(runes) || runes[i] != '$' {
		return ""
	}
	j := i + 1
	for j < len(runes) && (isIdent(runes[j])) {
		j++
	}
	if j < len(runes) && runes[j] == '$' {
		return string(runes[i : j+1])
	}
	return ""
}

func hasPrefixRunes(runes []rune, i int, tag string) bool {
	text := []rune(tag)
	if i+len(text) > len(runes) {
		return false
	}
	for k := range text {
		if runes[i+k] != text[k] {
			return false
		}
	}
	return true
}

func beginsBlock(runes []rune, i int) bool {
	word := nextWord(runes, i)
	if word == "" {
		return true
	}
	switch word {
	case "select", "insert", "update", "delete", "with", "declare", "if", "while", "loop", "return",
		"set", "create", "drop", "alter", "call", "execute", "exec", "open", "close", "fetch",
		"leave", "iterate", "begin", "end", "case", "null", "raise", "commit", "rollback",
		"start", "savepoint", "perform", "merge", "truncate", "grant", "revoke", "comment":
		return true
	default:
		return false
	}
}

func closesBlock(runes []rune, i int) bool {
	switch nextWord(runes, i) {
	case "if", "loop", "while", "repeat":
		return false
	default:
		return true
	}
}

func nextWord(runes []rune, i int) string {
	for i < len(runes) {
		ch := runes[i]
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < len(runes) {
				i += 2
			}
			continue
		}
		if !isIdentStart(ch) {
			return ""
		}
		word, _ := readWord(runes, i)
		return strings.ToLower(word)
	}
	return ""
}

func adjacentWord(runes []rune, i int, forward bool) string {
	if forward {
		return nextWord(runes, i)
	}
	j := i - 1
	for j >= 0 && unicode.IsSpace(runes[j]) {
		j--
	}
	if j < 0 || !isIdent(runes[j]) {
		return ""
	}
	end := j + 1
	for j >= 0 && isIdent(runes[j]) {
		j--
	}
	return strings.ToLower(string(runes[j+1 : end]))
}

// Package sqlscript 按语句切分受控 SQL 文本，供迁移和目标库执行使用。
package sqlscript

import (
	"strings"
	"unicode"
)

// Part 是切分后的一条语句。行号从 1 开始。
// TextLine 是语句文本的第一行，包含它开头的注释。Line 是第一行代码，不计前面的空行和注释。
type Part struct {
	Text     string
	TextLine int
	Line     int
}

// Split 按分号切分语句，保留引号、括号标识符、注释、美元引号和 BEGIN/END、CASE/END 块中的分号。
func Split(sqlText string) []string {
	parts := SplitParts(sqlText)
	statements := make([]string, len(parts))
	for i, part := range parts {
		statements[i] = part.Text
	}
	return statements
}

// SplitParts 与 Split 切出相同的语句，并记下每条语句在原脚本中的行号。
func SplitParts(sqlText string) []Part {
	var (
		parts       []Part
		buf         strings.Builder
		line        = 1
		contentLine int
		inSingle    bool
		inDouble    bool
		inBacktick  bool
		inBracket   bool
		inLine      bool
		inBlock     bool
		depth       int
		dollar      string
	)
	write := func(ch rune) {
		if contentLine == 0 && !unicode.IsSpace(ch) {
			contentLine = line
		}
		buf.WriteRune(ch)
		if ch == '\n' {
			line++
		}
	}
	writeString := func(text string) {
		for _, ch := range text {
			write(ch)
		}
	}
	flush := func() {
		statement := strings.TrimSpace(buf.String())
		buf.Reset()
		if statement != "" {
			textLine := contentLine
			if textLine < 1 {
				textLine = 1
			}
			parts = append(parts, Part{Text: statement, TextLine: textLine, Line: textLine + codeOffset(statement)})
		}
		contentLine = 0
	}
	runes := []rune(sqlText)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLine {
			write(ch)
			if ch == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			write(ch)
			if ch == '*' && next == '/' {
				write(next)
				i++
				inBlock = false
			}
			continue
		}
		if inSingle {
			write(ch)
			if ch == '\'' && next == '\'' {
				write(next)
				i++
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			write(ch)
			if ch == '"' && next == '"' {
				write(next)
				i++
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			write(ch)
			if ch == '`' && next == '`' {
				write(next)
				i++
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			write(ch)
			if ch == ']' && next == ']' {
				write(next)
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
				writeString(dollar)
				i += len([]rune(dollar)) - 1
				dollar = ""
				continue
			}
			write(ch)
			continue
		}
		if ch == '$' {
			if tag := dollarTag(runes, i); tag != "" {
				writeString(tag)
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
			writeString(word)
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
				write(ch)
				write(next)
				i++
				inLine = true
				continue
			}
		case '/':
			if next == '*' {
				write(ch)
				write(next)
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
		write(ch)
	}
	flush()
	return parts
}

// codeOffset 返回语句开头的注释和空白占了多少行。调用方把它加到内容起始行上，得到第一行代码。
func codeOffset(statement string) int {
	runes := []rune(statement)
	newlines := 0
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '\n' {
			newlines++
			continue
		}
		if unicode.IsSpace(ch) {
			continue
		}
		if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			if i < len(runes) && runes[i] == '\n' {
				newlines++
			}
			continue
		}
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				if runes[i] == '\n' {
					newlines++
				}
				i++
			}
			if i+1 < len(runes) {
				i++
			}
			continue
		}
		return newlines
	}
	return 0
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

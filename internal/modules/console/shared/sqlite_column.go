package shared

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type sqliteNote struct {
	identity bool
	comment  string
}

func sqliteColumnNotes(sqlText string) map[string]sqliteNote {
	list, ok := extractParenList(sqlText)
	if !ok {
		return nil
	}
	notes := map[string]sqliteNote{}
	for _, clause := range splitTopLevel(list) {
		name, identity, comment, ok := readSQLiteColumn(clause)
		if !ok || name == "" {
			continue
		}
		notes[name] = sqliteNote{identity: identity, comment: comment}
	}
	return notes
}

func noteFor(notes map[string]sqliteNote, name string) (sqliteNote, bool) {
	if notes == nil {
		return sqliteNote{}, false
	}
	if note, ok := notes[name]; ok {
		return note, true
	}
	var found sqliteNote
	matches := 0
	for key, note := range notes {
		if strings.EqualFold(key, name) {
			found = note
			matches++
		}
	}
	if matches == 1 {
		return found, true
	}
	return sqliteNote{}, false
}

func extractParenList(sqlText string) (string, bool) {
	runes := []rune(sqlText)
	depth := 0
	start := -1
	inSingle, inDouble, inBacktick, inBracket := false, false, false, false
	inLine, inBlock := false, false
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLine {
			if ch == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			if ch == '*' && next == '/' {
				i++
				inBlock = false
			}
			continue
		}
		if inSingle {
			if ch == '\'' && next == '\'' {
				i++
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' && next == '"' {
				i++
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' && next == '`' {
				i++
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			if ch == ']' && next == ']' {
				i++
				continue
			}
			if ch == ']' {
				inBracket = false
			}
			continue
		}
		if ch == '-' && next == '-' {
			inLine = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			inBlock = true
			i++
			continue
		}
		switch ch {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case '[':
			inBracket = true
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			if depth == 0 {
				return "", false
			}
			depth--
			if depth == 0 && start >= 0 {
				return string(runes[start:i]), true
			}
		}
	}
	return "", false
}

func splitTopLevel(sqlText string) []string {
	runes := []rune(sqlText)
	parts := []string{}
	start := 0
	depth := 0
	inSingle, inDouble, inBacktick, inBracket := false, false, false, false
	inLine, inBlock := false, false
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if inLine {
			if ch == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			if ch == '*' && next == '/' {
				i++
				inBlock = false
			}
			continue
		}
		if inSingle {
			if ch == '\'' && next == '\'' {
				i++
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' && next == '"' {
				i++
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' && next == '`' {
				i++
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			if ch == ']' && next == ']' {
				i++
				continue
			}
			if ch == ']' {
				inBracket = false
			}
			continue
		}
		if ch == '-' && next == '-' {
			inLine = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			inBlock = true
			i++
			continue
		}
		switch ch {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case '[':
			inBracket = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, string(runes[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, string(runes[start:]))
	return parts
}

func readSQLiteColumn(clause string) (string, bool, string, bool) {
	name, quoted, _, ok := readIdent(clause)
	if !ok {
		return "", false, "", false
	}
	if !quoted && isTableConstraint(name) {
		return "", false, "", false
	}
	identity, comment := scanColumnTail(clause)
	return name, identity, comment, true
}

func readIdent(clause string) (string, bool, string, bool) {
	text := strings.TrimLeftFunc(clause, unicode.IsSpace)
	if text == "" {
		return "", false, "", false
	}
	runes := []rune(text)
	switch runes[0] {
	case '"', '`', '[':
		end := runes[0]
		if runes[0] == '[' {
			end = ']'
		}
		var name strings.Builder
		for i := 1; i < len(runes); i++ {
			if runes[i] == end {
				if i+1 < len(runes) && runes[i+1] == end {
					name.WriteRune(end)
					i++
					continue
				}
				return name.String(), true, string(runes[i+1:]), true
			}
			name.WriteRune(runes[i])
		}
		return "", false, "", false
	default:
		i := 0
		for i < len(runes) && isIdentRune(runes[i], i == 0) {
			i++
		}
		if i == 0 {
			return "", false, "", false
		}
		return string(runes[:i]), false, string(runes[i:]), true
	}
}

func isIdentRune(ch rune, first bool) bool {
	if ch == '_' || ch == '$' || unicode.IsLetter(ch) {
		return true
	}
	return !first && unicode.IsDigit(ch)
}

func isTableConstraint(word string) bool {
	switch strings.ToLower(word) {
	case "primary", "unique", "constraint", "foreign", "check":
		return true
	default:
		return false
	}
}

func scanColumnTail(clause string) (bool, string) {
	runes := []rune(clause)
	identity := false
	comment := ""
	inSingle, inDouble, inBacktick, inBracket := false, false, false, false
	wordStart := -1
	flushWord := func(end int) {
		if wordStart < 0 || end <= wordStart {
			wordStart = -1
			return
		}
		if strings.EqualFold(string(runes[wordStart:end]), "autoincrement") {
			identity = true
		}
		wordStart = -1
	}
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		quoted := inSingle || inDouble || inBacktick || inBracket
		if !quoted && ch == '-' && next == '-' {
			flushWord(i)
			comment = strings.TrimSpace(string(runes[i+2:]))
			break
		}
		if !quoted && ch == '/' && next == '*' {
			flushWord(i)
			end := i + 2
			for end+1 < len(runes) && !(runes[end] == '*' && runes[end+1] == '/') {
				end++
			}
			if end+1 >= len(runes) {
				break
			}
			comment = strings.TrimSpace(string(runes[i+2 : end]))
			i = end + 1
			continue
		}
		if inSingle {
			if ch == '\'' && next == '\'' {
				i++
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if ch == '"' && next == '"' {
				i++
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' && next == '`' {
				i++
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			if ch == ']' && next == ']' {
				i++
				continue
			}
			if ch == ']' {
				inBracket = false
			}
			continue
		}
		switch ch {
		case '\'':
			flushWord(i)
			inSingle = true
		case '"':
			flushWord(i)
			inDouble = true
		case '`':
			flushWord(i)
			inBacktick = true
		case '[':
			flushWord(i)
			inBracket = true
		default:
			if isIdentRune(ch, wordStart < 0) {
				if wordStart < 0 {
					wordStart = i
				}
				continue
			}
			flushWord(i)
		}
	}
	flushWord(len(runes))
	return identity, comment
}

func sqliteCommentText(comment string) string {
	comment = strings.ReplaceAll(comment, "*/", "* /")
	if !utf8.ValidString(comment) {
		return ""
	}
	return comment
}

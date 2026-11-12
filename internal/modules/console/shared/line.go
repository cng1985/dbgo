package shared

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	messageLinePattern = regexp.MustCompile(`(?i)\bat line (\d+)\b`)
	objectPatterns     = []*regexp.Regexp{
		regexp.MustCompile(`(?i)no such column: ([^ (\n]+)`),
		regexp.MustCompile(`(?i)no such table: ([^ (\n]+)`),
		regexp.MustCompile(`(?i)unknown column '([^']+)'`),
		regexp.MustCompile(`(?i)invalid column name '([^']+)'`),
		regexp.MustCompile(`(?i)invalid object name '([^']+)'`),
		regexp.MustCompile(`(?i)table '([^']+)' doesn't exist`),
	}
	nearPattern = regexp.MustCompile(`(?i)near ["']([^"']+)["']`)
)

// lineError 记下失败语句在原脚本中的行号，对外仍返回驱动或校验给出的原文。
type lineError struct {
	Line int
	Err  error
}

func (e *lineError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *lineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorLine 返回脚本里出错的行。没有行号时返回 0。
func ErrorLine(err error) int {
	var marked *lineError
	if errors.As(err, &marked) && marked != nil && marked.Line > 0 {
		return marked.Line
	}
	return 0
}

// PublicSQLError 把执行失败转成给用户看的句子，并在知道行号时写在最前面。
func PublicSQLError(err error) string {
	message := PublicError("SQL 执行失败", "", err)
	if line := ErrorLine(err); line > 0 {
		return fmt.Sprintf("第 %d 行：%s", line, message)
	}
	return message
}

func failStatement(part sqlscript.Part, err error) error {
	if err == nil {
		return nil
	}
	line := part.Line
	if line < 1 {
		line = 1
	}
	if rel := driverRelativeLine(part.Text, err); rel > 0 {
		// 驱动的行号从发给它的语句文本算起，文本开头的注释也算一行。
		// 没有驱动行号时才退回第一行代码。
		base := part.TextLine
		if base < 1 {
			base = line
		}
		line = base + rel - 1
		if line < 1 {
			line = 1
		}
	}
	return &lineError{Line: line, Err: err}
}

// driverRelativeLine 读取驱动给出的语句内行号。没有可靠行号时返回 0，调用方就用语句起始行。
func driverRelativeLine(statement string, err error) int {
	var numbered interface{ SQLErrorLineNo() int32 }
	if errors.As(err, &numbered) && numbered != nil {
		if n := int(numbered.SQLErrorLineNo()); n > 0 {
			return n
		}
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg != nil && pg.Position > 0 {
		// 位置是发给数据库的文本。问号会被改成 $1，行号要按改写后的文本计算，换行本身不会变。
		return lineOfRune(RewritePlaceholders("postgres", statement), int(pg.Position))
	}
	if line := lineInMessage(err); line > 0 {
		return line
	}
	return lineOfErrorToken(statement, err)
}

func lineInMessage(err error) int {
	if err == nil {
		return 0
	}
	match := messageLinePattern.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return 0
	}
	line, convErr := strconv.Atoi(match[1])
	if convErr != nil || line < 1 {
		return 0
	}
	return line
}

func lineOfErrorToken(statement string, err error) int {
	if err == nil {
		return 0
	}
	message := err.Error()
	for _, pattern := range objectPatterns {
		match := pattern.FindStringSubmatch(message)
		if len(match) == 2 {
			return tokenLine(statement, match[1], false)
		}
	}
	match := nearPattern.FindStringSubmatch(message)
	if len(match) == 2 {
		return tokenLine(statement, match[1], true)
	}
	return 0
}

func tokenLine(statement, token string, last bool) int {
	lines := tokenLines(statement, strings.TrimSpace(token))
	if len(lines) == 0 {
		if dot := strings.LastIndex(token, "."); dot >= 0 && dot < len(token)-1 {
			lines = tokenLines(statement, token[dot+1:])
		}
	}
	if len(lines) == 0 {
		return 0
	}
	if last {
		return lines[len(lines)-1]
	}
	return lines[0]
}

func tokenLines(statement, token string) []int {
	want := []rune(strings.ToLower(token))
	if len(want) == 0 {
		return nil
	}
	runes := []rune(statement)
	var lines []int
	line := 1
	for i := 0; i < len(runes); {
		ch := runes[i]
		if sqlscript.CountsAsLineBreak(ch, nextRune(runes, i)) {
			line++
			i++
			continue
		}
		if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				if sqlscript.CountsAsLineBreak(runes[i], nextRune(runes, i)) {
					line++
				}
				i++
			}
			if i+1 < len(runes) {
				i += 2
			} else {
				i = len(runes)
			}
			continue
		}
		if ch == '\'' {
			i = skipQuote(runes, i, '\'', &line)
			continue
		}
		if ch == '"' || ch == '`' || ch == '[' {
			end, inner, startLine := readIdentQuote(runes, i, &line)
			if nameMatches(inner, token) {
				lines = append(lines, startLine)
			}
			i = end
			continue
		}
		if hasFold(runes, i, want) && tokenBoundary(runes, i, i+len(want)) {
			lines = append(lines, line)
			i += len(want)
			continue
		}
		i++
	}
	return lines
}

func nameMatches(inner, token string) bool {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return false
	}
	if strings.EqualFold(inner, strings.TrimSpace(token)) {
		return true
	}
	if dot := strings.LastIndex(token, "."); dot >= 0 && dot < len(token)-1 {
		return strings.EqualFold(inner, token[dot+1:])
	}
	return false
}

func readIdentQuote(runes []rune, i int, line *int) (int, string, int) {
	startLine := *line
	closer := runes[i]
	if closer == '[' {
		closer = ']'
	}
	i++
	var inner []rune
	for i < len(runes) {
		ch := runes[i]
		if sqlscript.CountsAsLineBreak(ch, nextRune(runes, i)) {
			*line++
			inner = append(inner, ch)
			i++
			continue
		}
		if ch == closer {
			if i+1 < len(runes) && runes[i+1] == closer {
				inner = append(inner, closer)
				i += 2
				continue
			}
			return i + 1, string(inner), startLine
		}
		inner = append(inner, ch)
		i++
	}
	return len(runes), string(inner), startLine
}

func skipQuote(runes []rune, i int, closer rune, line *int) int {
	i++
	for i < len(runes) {
		ch := runes[i]
		if sqlscript.CountsAsLineBreak(ch, nextRune(runes, i)) {
			*line++
		}
		if ch == closer {
			if i+1 < len(runes) && runes[i+1] == closer {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(runes)
}

func hasFold(runes []rune, i int, want []rune) bool {
	if i+len(want) > len(runes) {
		return false
	}
	for j, ch := range want {
		if unicode.ToLower(runes[i+j]) != ch {
			return false
		}
	}
	return true
}

func tokenBoundary(runes []rune, start, end int) bool {
	if start > 0 && errorIdentRune(runes[start-1]) {
		return false
	}
	if end < len(runes) && errorIdentRune(runes[end]) {
		return false
	}
	return true
}

func errorIdentRune(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

// lineOfRune 把 1 开始的字符位置换成语句内的行号。位置超出文本时落到最后一行。
func nextRune(runes []rune, i int) rune {
	if i+1 < len(runes) {
		return runes[i+1]
	}
	return 0
}

func lineOfRune(statement string, position int) int {
	if position < 1 {
		return 0
	}
	runes := []rune(statement)
	line := 1
	for i, ch := range runes {
		if i+1 == position {
			return line
		}
		if sqlscript.CountsAsLineBreak(ch, nextRune(runes, i)) {
			line++
		}
	}
	if len(runes) == 0 {
		return 0
	}
	return line
}

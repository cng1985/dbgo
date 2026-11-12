package shared

import (
	"errors"
	"fmt"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
	"github.com/jackc/pgx/v5/pgconn"
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
	return 0
}

// lineOfRune 把 1 开始的字符位置换成语句内的行号。位置超出文本时落到最后一行。
func lineOfRune(statement string, position int) int {
	if position < 1 {
		return 0
	}
	line := 1
	count := 0
	for _, ch := range statement {
		count++
		if count == position {
			return line
		}
		if ch == '\n' {
			line++
		}
	}
	if count == 0 {
		return 0
	}
	return line
}

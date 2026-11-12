package shared

import (
	"errors"
	"testing"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
	"github.com/jackc/pgx/v5/pgconn"
)

type numberedError struct {
	line int32
	msg  string
}

func (e numberedError) Error() string { return e.msg }

func (e numberedError) SQLErrorLineNo() int32 { return e.line }

func TestFailStatementUsesDriverLine(t *testing.T) {
	part := sqlscript.Part{Text: "select\nbad", Line: 4}
	err := failStatement(part, numberedError{line: 2, msg: "Invalid column name"})
	if ErrorLine(err) != 5 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
	if err.Error() != "Invalid column name" {
		t.Fatalf("对外错误被改写: %v", err)
	}
	var numbered numberedError
	if !errors.As(err, &numbered) {
		t.Fatal("行号包装应能解开到驱动错误")
	}
}

func TestFailStatementUsesPostgresPosition(t *testing.T) {
	part := sqlscript.Part{Text: "select\nbad", Line: 4}
	err := failStatement(part, &pgconn.PgError{Message: "syntax error", Position: 8})
	if ErrorLine(err) != 5 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
	message := PublicSQLError(err)
	want := "第 5 行：SQL 执行失败："
	if len(message) < len(want) || message[:len(want)] != want {
		t.Fatalf("提示 = %s", message)
	}
}

func TestFailStatementUsesRewrittenPostgresPosition(t *testing.T) {
	statement := "select ?, ?, ?, ?\nx\ny"
	sent := RewritePlaceholders("postgres", statement)
	position := 0
	for _, ch := range sent {
		position++
		if ch == 'x' {
			break
		}
	}
	part := sqlscript.Part{Text: statement, Line: 1}
	err := failStatement(part, &pgconn.PgError{Message: "syntax error", Position: int32(position)})
	if ErrorLine(err) != 2 {
		t.Fatalf("行号 = %d，位置 = %d，改写 = %q，错误 = %v", ErrorLine(err), position, sent, err)
	}
}

func TestFailStatementKeepsStartLineWithoutDriverLine(t *testing.T) {
	part := sqlscript.Part{Text: "select missing", Line: 3}
	err := failStatement(part, errors.New("no such column: missing"))
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d", ErrorLine(err))
	}
}

func TestLineOfRuneClampsPastEnd(t *testing.T) {
	if got := lineOfRune("select\nbad", 100); got != 2 {
		t.Fatalf("超出文本时应落到最后一行，得到 %d", got)
	}
	if got := lineOfRune("select\nbad", 8); got != 2 {
		t.Fatalf("第 8 个字符应在第 2 行，得到 %d", got)
	}
}

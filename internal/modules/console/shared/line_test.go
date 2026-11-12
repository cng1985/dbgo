package shared

import (
	"errors"
	"strings"
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

func TestFailStatementDoesNotCountLeadingCommentTwice(t *testing.T) {
	parts := sqlscript.SplitParts("select 1;\n-- note\nselect bad")
	if len(parts) != 2 || parts[1].TextLine != 2 || parts[1].Line != 3 {
		t.Fatalf("片段 = %#v", parts)
	}
	err := failStatement(parts[1], numberedError{line: 2, msg: "bad"})
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
	position := 0
	for _, ch := range parts[1].Text {
		position++
		if ch == 'b' {
			break
		}
	}
	pgErr := failStatement(parts[1], &pgconn.PgError{Message: "syntax error", Position: int32(position)})
	if ErrorLine(pgErr) != 3 {
		t.Fatalf("PostgreSQL 行号 = %d，位置 = %d，文本 = %q", ErrorLine(pgErr), position, parts[1].Text)
	}
}

func TestFailStatementFindsChineseColumnLine(t *testing.T) {
	script := strings.Join([]string{
		"SELECT",
		"    es.session_id AS [卡住的会话ID],",
		"    er.blocking_session_id AS [阻塞源头ID],",
		"    er.wait_type AS [等待类型],",
		"    er.wait_time / 1000 AS [等待秒数],",
		"    er.cpu_time / 1000 AS [已耗CPU秒数],",
		"    er.logical_reads AS [逻辑读次数],",
		"",
		"    -- 客户端信息",
		"    ec.client_net_address AS [客户端IP],",
		"    ec.client_tcp_port AS [客户端端口],",
		"    es.host_name AS [客户端机器],",
		"    es.program_name AS [应用名称],",
		"    es.login_name1 AS [登录用户],",
	}, "\n")
	parts := sqlscript.SplitParts(script)
	if len(parts) != 1 {
		t.Fatalf("应是一条语句: %#v", parts)
	}
	want := 0
	for i, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "login_name1") {
			want = i + 1
		}
	}
	if want != 14 {
		t.Fatalf("示例里的字段应在第 14 行，实际 %d", want)
	}
	reported := failStatement(parts[0], sqlServerFailure{text: "列名 'login_name1' 无效。", line: 1})
	if ErrorLine(reported) != want {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(reported), reported)
	}
	if PublicSQLError(reported) != "第 14 行：SQL 执行失败：列名 'login_name1' 无效。" {
		t.Fatalf("提示 = %s", PublicSQLError(reported))
	}
	kept := failStatement(parts[0], sqlServerFailure{text: "列名 'login_name1' 无效。", line: int32(want)})
	if ErrorLine(kept) != want {
		t.Fatalf("服务器给出的行号应保留，行号 = %d", ErrorLine(kept))
	}
}

func TestFailStatementTrustsSQLServerLinePastOne(t *testing.T) {
	part := sqlscript.Part{Text: "select login_name1\nfrom t", TextLine: 5, Line: 5}
	err := failStatement(part, sqlServerFailure{text: "列名 'login_name1' 无效。", line: 2})
	if ErrorLine(err) != 6 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestFailStatementUsesMySQLLine(t *testing.T) {
	part := sqlscript.Part{Text: "select\n1\nform city", TextLine: 4, Line: 4}
	err := failStatement(part, errors.New("Error 1064 (42000): near 'form city' at line 3"))
	if ErrorLine(err) != 6 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestFailStatementSkipsColumnNameInString(t *testing.T) {
	part := sqlscript.Part{Text: "select 'missing_column'\n, missing_column from city", TextLine: 1, Line: 1}
	err := failStatement(part, errors.New("SQL logic error: no such column: missing_column (1)"))
	if ErrorLine(err) != 2 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestFailStatementSkipsColumnNameInComment(t *testing.T) {
	part := sqlscript.Part{Text: "-- missing_column\nselect\n  missing_column\nfrom city", TextLine: 1, Line: 2}
	err := failStatement(part, errors.New("SQL logic error: no such column: missing_column (1)"))
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d，错误 = %v", ErrorLine(err), err)
	}
}

func TestFailStatementKeepsStartLineWithoutDriverLine(t *testing.T) {
	part := sqlscript.Part{Text: "select missing", Line: 3}
	err := failStatement(part, errors.New("no such column: missing"))
	if ErrorLine(err) != 3 {
		t.Fatalf("行号 = %d", ErrorLine(err))
	}
}

func TestLineOfRuneCountsCarriageReturnOnce(t *testing.T) {
	if line := lineOfRune("select\r\nbad", 9); line != 2 {
		t.Fatalf("回车换行后的行号 = %d", line)
	}
	if line := lineOfRune("select\rbad", 8); line != 2 {
		t.Fatalf("单独回车后的行号 = %d", line)
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

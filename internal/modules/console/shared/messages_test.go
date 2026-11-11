package shared

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-sql/sqlexp"
)

func init() {
	sql.Register("printfake", printDriver{})
}

type textMessage string

func (t textMessage) String() string { return string(t) }

type sqlError struct {
	message string
}

func (e sqlError) Error() string { return "mssql: " + e.message }

func (e sqlError) SQLErrorMessage() string { return e.message }

func TestReadOnlyAllowsPrint(t *testing.T) {
	if FirstToken("print '11'") != "print" {
		t.Fatalf("首个词应为 print")
	}
	if !ReadOnlyToken("print") {
		t.Fatal("只读模式应允许 PRINT")
	}
	if ReadOnlyToken("insert") {
		t.Fatal("只读模式不应允许 INSERT")
	}
}

func TestCollectPrintMessage(t *testing.T) {
	sets, err := playMessages(t,
		sqlexp.MsgNotice{Message: textMessage("11")},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNextResultSet{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 {
		t.Fatalf("PRINT 应只有一条结果: %#v", sets)
	}
	if len(sets[0].Columns) != 0 || len(sets[0].Messages) != 1 || sets[0].Messages[0] != "11" {
		t.Fatalf("应收集到 11: %#v", sets[0])
	}
}

func TestCollectPrintAroundSelect(t *testing.T) {
	sets, err := playMessages(t,
		sqlexp.MsgNotice{Message: textMessage("msg1")},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNext{},
		sqlexp.MsgRowsAffected{Count: 1},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNotice{Message: textMessage("msg2")},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNextResultSet{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 3 {
		t.Fatalf("应分成消息、结果集、消息: %#v", sets)
	}
	if sets[0].Messages[0] != "msg1" || len(sets[0].Columns) != 0 {
		t.Fatalf("第一条应是 msg1: %#v", sets[0])
	}
	if len(sets[1].Columns) != 1 || sets[1].Columns[0].Name != "c" || len(sets[1].Rows) != 1 || sets[1].RowsAffected != 1 {
		t.Fatalf("中间应是查询结果: %#v", sets[1])
	}
	if sets[2].Messages[0] != "msg2" {
		t.Fatalf("最后一条应是 msg2: %#v", sets[2])
	}
}

func TestCollectRowsAffectedWithoutGrid(t *testing.T) {
	sets, err := playMessages(t,
		sqlexp.MsgRowsAffected{Count: 3},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNextResultSet{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].RowsAffected != 3 || len(sets[0].Columns) != 0 || len(sets[0].Messages) != 0 {
		t.Fatalf("更新语句应只保留影响行数: %#v", sets)
	}
}

func TestCollectServerErrorUsesMessageText(t *testing.T) {
	_, err := playMessages(t,
		sqlexp.MsgNotice{Message: textMessage("before")},
		sqlexp.MsgError{Error: sqlError{message: "对象名无效"}},
		sqlexp.MsgNextResultSet{},
	)
	if err == nil || err.Error() != "对象名无效" {
		t.Fatalf("应返回服务器消息正文: %v", err)
	}
}

func TestCollectStopsWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := collectServerMessages(ctx, func(context.Context) any {
		t.Fatal("上下文已取消时不应再取消息")
		return nil
	}, func() bool { return false }, func() error { return nil }, nil, "print")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回取消错误: %v", err)
	}
}

func TestCollectSelectKeepsRowCount(t *testing.T) {
	sets, err := playMessages(t,
		sqlexp.MsgNext{},
		sqlexp.MsgRowsAffected{Count: 99},
		sqlexp.MsgNextResultSet{},
		sqlexp.MsgNextResultSet{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].RowsAffected != 1 || len(sets[0].Rows) != 1 {
		t.Fatalf("查询结果应保留读到的行数: %#v", sets)
	}
}

func playMessages(t *testing.T, messages ...any) ([]Result, error) {
	t.Helper()
	totalBoundaries := 0
	for _, message := range messages {
		if _, ok := message.(sqlexp.MsgNextResultSet); ok {
			totalBoundaries++
		}
	}
	index := 0
	boundaries := 0
	return collectServerMessages(context.Background(), func(context.Context) any {
		if index >= len(messages) {
			t.Fatalf("消息已读完，仍在等待")
		}
		message := messages[index]
		index++
		return message
	}, func() bool {
		boundaries++
		return boundaries < totalBoundaries
	}, func() error { return nil }, func() (Result, error) {
		value := "100"
		return Result{
			Columns:      []Column{{Name: "c", Type: "INT"}},
			Rows:         [][]*string{{&value}},
			RowsAffected: 1,
		}, nil
	}, "print")
}

func TestPrintIsNotBlockedAsWrite(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "print.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = Execute(context.Background(), db, "sqlite", "print '11'", ExecOptions{ReadOnly: true})
	if err == nil || strings.Contains(err.Error(), "只读模式") {
		t.Fatalf("只读模式应放行 PRINT，再由数据库判断语法: %v", err)
	}
}

func TestSQLServerMessagesThroughDriver(t *testing.T) {
	db, err := sql.Open("printfake", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	printed, err := runStatement(context.Background(), db, "sqlserver", "print '11'", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(printed) != 1 || len(printed[0].Messages) != 1 || printed[0].Messages[0] != "11" || len(printed[0].Columns) != 0 {
		t.Fatalf("PRINT 应进入输出: %#v", printed)
	}

	selected, err := runStatement(context.Background(), db, "sqlserver", "select 1 as n", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].RowsAffected != 1 || cellText(selected[0], 0, 0) != "1" {
		t.Fatalf("查询仍应返回行: %#v", selected)
	}

	mixed, err := runStatement(context.Background(), db, "sqlserver", "print 'msg1'; select 100 as c; print 'msg2'", 10)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	var sawRow bool
	for _, set := range mixed {
		lines = append(lines, set.Messages...)
		if cellText(set, 0, 0) == "100" {
			sawRow = true
		}
	}
	if strings.Join(lines, "\n") != "msg1\nmsg2" || !sawRow {
		t.Fatalf("同一批应同时有消息和结果集: %#v", mixed)
	}
}

func cellText(result Result, row, column int) string {
	if row >= len(result.Rows) || column >= len(result.Rows[row]) || result.Rows[row][column] == nil {
		return ""
	}
	return *result.Rows[row][column]
}

type printDriver struct{}

func (printDriver) Open(string) (driver.Conn, error) { return &printConn{}, nil }

type printConn struct {
	msg *sqlexp.ReturnMessage
}

func (c *printConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("不支持预处理")
}
func (c *printConn) Close() error              { return nil }
func (c *printConn) Begin() (driver.Tx, error) { return nil, errors.New("不支持事务") }

func (c *printConn) CheckNamedValue(nv *driver.NamedValue) error {
	msg, ok := nv.Value.(*sqlexp.ReturnMessage)
	if !ok {
		return nil
	}
	sqlexp.ReturnMessageInit(msg)
	c.msg = msg
	return driver.ErrRemoveArgument
}

func (c *printConn) QueryContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.msg == nil {
		return nil, errors.New("缺少消息队列")
	}
	rows, messages := scriptFor(query)
	for _, message := range messages {
		if err := sqlexp.ReturnMessageEnqueue(ctx, c.msg, message); err != nil {
			return nil, err
		}
	}
	c.msg = nil
	return rows, nil
}

func scriptFor(query string) (*scriptRows, []any) {
	text := strings.ToLower(query)
	switch {
	case strings.Contains(text, "print") && strings.Contains(text, "select"):
		return &scriptRows{
				successes: 3,
				sets: []scriptSet{{
					cols: []string{"c"},
					rows: [][]driver.Value{{int64(100)}},
					more: true,
				}},
			}, []any{
				sqlexp.MsgNotice{Message: textMessage("msg1")},
				sqlexp.MsgNextResultSet{},
				sqlexp.MsgNext{},
				sqlexp.MsgRowsAffected{Count: 1},
				sqlexp.MsgNextResultSet{},
				sqlexp.MsgNotice{Message: textMessage("msg2")},
				sqlexp.MsgNextResultSet{},
				sqlexp.MsgNextResultSet{},
			}
	case strings.Contains(text, "print"):
		return &scriptRows{successes: 1}, []any{
			sqlexp.MsgNotice{Message: textMessage("11")},
			sqlexp.MsgNextResultSet{},
			sqlexp.MsgNextResultSet{},
		}
	default:
		return &scriptRows{
				active: true,
				sets: []scriptSet{{
					cols: []string{"n"},
					rows: [][]driver.Value{{int64(1)}},
				}},
			}, []any{
				sqlexp.MsgNext{},
				sqlexp.MsgRowsAffected{Count: 5},
				sqlexp.MsgNextResultSet{},
				sqlexp.MsgNextResultSet{},
			}
	}
}

type scriptSet struct {
	cols []string
	rows [][]driver.Value
	more bool
}

type scriptRows struct {
	sets        []scriptSet
	set         int
	row         int
	successes   int
	active      bool
	requestDone bool
}

func (r *scriptRows) Columns() []string {
	if !r.active || r.set >= len(r.sets) {
		return []string{}
	}
	return r.sets[r.set].cols
}

func (r *scriptRows) Close() error { return nil }

func (r *scriptRows) Next(dest []driver.Value) error {
	if !r.active || r.set >= len(r.sets) {
		r.requestDone = true
		return io.EOF
	}
	set := &r.sets[r.set]
	if r.row >= len(set.rows) {
		if !set.more {
			r.requestDone = true
		}
		return io.EOF
	}
	row := set.rows[r.row]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		}
	}
	r.row++
	return nil
}

func (r *scriptRows) HasNextResultSet() bool { return !r.requestDone }

func (r *scriptRows) NextResultSet() error {
	if r.requestDone || r.successes == 0 {
		r.requestDone = true
		return io.EOF
	}
	r.successes--
	if !r.active && len(r.sets) > 0 {
		r.active = true
		r.set = 0
		r.row = 0
		return nil
	}
	if r.active && r.set+1 < len(r.sets) {
		r.set++
		r.row = 0
	}
	return nil
}

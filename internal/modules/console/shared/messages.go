package shared

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-sql/sqlexp"
)

func querySQLServer(ctx context.Context, db queryer, query, command string, maxRows int) ([]Result, error) {
	retmsg := &sqlexp.ReturnMessage{}
	rows, err := db.QueryContext(ctx, query, retmsg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectServerMessages(ctx, func(ctx context.Context) any {
		return retmsg.Message(ctx)
	}, rows.NextResultSet, rows.Err, func() (Result, error) {
		return readMessageRows(rows, command, maxRows)
	}, command)
}

func collectServerMessages(ctx context.Context, nextMessage func(context.Context) any, nextResultSet func() bool, rowsErr func() error, readRows func() (Result, error), command string) ([]Result, error) {
	var sets []Result
	var pending []string
	var open *Result
	var sawCount bool
	var affected int64
	var queryErr error

	flushBoundary := func() {
		if open != nil {
			if len(pending) > 0 {
				open.Messages = append(open.Messages, pending...)
				pending = nil
			}
			sets = append(sets, *open)
			open = nil
			sawCount = false
			affected = 0
			return
		}
		if len(pending) == 0 && !sawCount {
			return
		}
		count := int64(0)
		if sawCount {
			count = affected
		}
		sets = append(sets, Result{
			Command:      command,
			Columns:      []Column{},
			Rows:         [][]*string{},
			RowsAffected: count,
			Messages:     append([]string(nil), pending...),
		})
		pending = nil
		sawCount = false
		affected = 0
	}

	active := true
	for active {
		if err := ctx.Err(); err != nil {
			flushBoundary()
			return sets, err
		}
		switch m := nextMessage(ctx).(type) {
		case sqlexp.MsgNotice:
			pending = append(pending, noticeText(m.Message))
		case sqlexp.MsgError:
			if queryErr == nil {
				queryErr = sqlServerFailure{text: serverMessageText(m.Error), line: sqlServerLineNo(m.Error)}
			}
		case sqlexp.MsgRowsAffected:
			if open != nil {
				break
			}
			if sawCount {
				affected += m.Count
			} else {
				affected = m.Count
				sawCount = true
			}
		case sqlexp.MsgNext:
			flushBoundary()
			if readRows == nil {
				if queryErr == nil {
					queryErr = fmt.Errorf("没有读取结果")
				}
				break
			}
			result, err := readRows()
			if err != nil && queryErr == nil {
				queryErr = err
			}
			result.Command = command
			if len(pending) > 0 {
				result.Messages = append(result.Messages, pending...)
				pending = nil
			}
			open = &result
		case sqlexp.MsgNextResultSet:
			if err := ctx.Err(); err != nil {
				flushBoundary()
				return sets, err
			}
			flushBoundary()
			if nextResultSet != nil && !nextResultSet() {
				if rowsErr != nil {
					if err := rowsErr(); err != nil && queryErr == nil {
						queryErr = err
					}
				}
				active = false
			}
		}
	}
	flushBoundary()
	if len(sets) == 0 {
		sets = append(sets, Result{Command: command, Columns: []Column{}, Rows: [][]*string{}})
	}
	if queryErr != nil {
		return sets, queryErr
	}
	return sets, nil
}

func readMessageRows(rows *sql.Rows, command string, maxRows int) (Result, error) {
	names, err := rows.Columns()
	if err != nil {
		drainRows(rows)
		return Result{Command: command, Columns: []Column{}, Rows: [][]*string{}}, err
	}
	types, _ := rows.ColumnTypes()
	columns := make([]Column, len(names))
	for i, name := range names {
		typeName := ""
		if i < len(types) && types[i] != nil {
			typeName = types[i].DatabaseTypeName()
		}
		columns[i] = Column{Name: name, Type: typeName}
	}
	result := Result{Command: command, Columns: columns, Rows: [][]*string{}}
	for rows.Next() {
		if maxRows > 0 && len(result.Rows) >= maxRows {
			result.Truncated = true
			continue
		}
		raw := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			drainRows(rows)
			return result, err
		}
		line := make([]*string, len(raw))
		for i, value := range raw {
			line[i] = formatCell(value, columns[i].Type)
		}
		result.Rows = append(result.Rows, line)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.RowsAffected = int64(len(result.Rows))
	return result, nil
}

func drainRows(rows *sql.Rows) {
	for rows.Next() {
	}
}

func noticeText(message fmt.Stringer) string {
	if message == nil {
		return ""
	}
	return message.String()
}

// sqlServerFailure 保留服务器给出的正文和行号。正文不再带驱动前缀。
type sqlServerFailure struct {
	text string
	line int32
}

func (e sqlServerFailure) Error() string {
	if e.text == "" {
		return "SQL 执行失败"
	}
	return e.text
}

func (e sqlServerFailure) SQLErrorMessage() string { return e.Error() }

func (e sqlServerFailure) SQLErrorLineNo() int32 { return e.line }

func sqlServerLineNo(err error) int32 {
	var numbered interface{ SQLErrorLineNo() int32 }
	if errors.As(err, &numbered) && numbered != nil {
		if n := numbered.SQLErrorLineNo(); n > 0 {
			return n
		}
	}
	return 0
}

func serverMessageText(err error) string {
	if err == nil {
		return "SQL 执行失败"
	}
	type detailed interface {
		SQLErrorMessage() string
	}
	if item, ok := err.(detailed); ok {
		if text := strings.TrimSpace(item.SQLErrorMessage()); text != "" {
			return text
		}
	}
	text := strings.TrimSpace(strings.TrimPrefix(err.Error(), "mssql: "))
	if text == "" {
		return "SQL 执行失败"
	}
	return text
}

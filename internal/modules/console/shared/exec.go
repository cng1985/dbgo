package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
)

type Column struct {
	Name string
	Type string
}

type Result struct {
	Command      string
	Columns      []Column
	Rows         [][]*string
	RowsAffected int64
	Truncated    bool
}

type ExecOptions struct {
	ReadOnly       bool
	MaxRows        int
	MaxStatements  int
	UseTransaction bool
}

func Execute(ctx context.Context, db *sql.DB, driver, script string, options ExecOptions) ([]Result, error) {
	if db == nil {
		return nil, fmt.Errorf("目标连接不存在")
	}
	statements := sqlscript.Split(script)
	if len(statements) == 0 {
		return nil, fmt.Errorf("SQL 不能为空")
	}
	if options.MaxStatements < 1 {
		options.MaxStatements = 50
	}
	if len(statements) > options.MaxStatements {
		return nil, fmt.Errorf("语句数量超过 %d", options.MaxStatements)
	}
	if options.MaxRows < 1 {
		options.MaxRows = 100
	}
	for _, statement := range statements {
		if Forbidden(statement) {
			return nil, fmt.Errorf("该语句被控制台拒绝")
		}
		if options.ReadOnly && !ReadOnlyToken(FirstToken(statement)) {
			return nil, fmt.Errorf("只读模式只允许查询语句")
		}
	}
	if options.UseTransaction && driver != "mysql" {
		return executeTransactional(ctx, db, driver, statements, options)
	}
	results := make([]Result, 0, len(statements))
	for _, statement := range statements {
		result, err := runStatement(ctx, db, driver, statement, options.MaxRows)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func executeTransactional(ctx context.Context, db *sql.DB, driver string, statements []string, options ExecOptions) ([]Result, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if options.ReadOnly && driver == "postgres" {
		if _, err := tx.ExecContext(ctx, "set transaction read only"); err != nil {
			return nil, err
		}
	}
	results := make([]Result, 0, len(statements))
	for _, statement := range statements {
		result, err := runStatement(ctx, tx, driver, statement, options.MaxRows)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return results, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func runStatement(ctx context.Context, db queryer, driver, statement string, maxRows int) (Result, error) {
	command := FirstToken(statement)
	query := RewritePlaceholders(driver, statement)
	if ReturnsRows(statement) {
		return queryRows(ctx, db, query, command, maxRows)
	}
	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return Result{Command: command}, err
	}
	affected, _ := result.RowsAffected()
	return Result{Command: command, Columns: []Column{}, Rows: [][]*string{}, RowsAffected: affected}, nil
}

func queryRows(ctx context.Context, db queryer, query, command string, maxRows int) (Result, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return Result{Command: command}, err
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return Result{Command: command}, err
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
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		raw := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
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

func formatCell(value any, typeName string) *string {
	if value == nil {
		return nil
	}
	var text string
	switch typed := value.(type) {
	case []byte:
		if binaryType(typeName) {
			text = "0x" + fmtHex(typed)
		} else {
			text = string(typed)
		}
	case time.Time:
		text = typed.UTC().Format(time.RFC3339Nano)
	case int64:
		text = strconv.FormatInt(typed, 10)
	case int32:
		text = strconv.FormatInt(int64(typed), 10)
	case float64:
		text = strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		text = strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case bool:
		text = strconv.FormatBool(typed)
	default:
		text = fmt.Sprint(typed)
	}
	return &text
}

func binaryType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "blob", "bytea", "binary", "varbinary", "image", "raw", "tinyblob", "mediumblob", "longblob":
		return true
	default:
		return false
	}
}

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(value)*2)
	for i, b := range value {
		out[i*2] = digits[b>>4]
		out[i*2+1] = digits[b&0x0f]
	}
	return string(out)
}

func ServerVersion(ctx context.Context, db *sql.DB, driver string) (string, error) {
	query := "select version()"
	switch driver {
	case "sqlite":
		query = "select sqlite_version()"
	case "sqlserver":
		query = "select @@version"
	}
	var version string
	if err := db.QueryRowContext(ctx, query).Scan(&version); err != nil {
		return "", err
	}
	version = strings.TrimSpace(version)
	if len(version) > 300 {
		version = version[:300]
	}
	return version, nil
}

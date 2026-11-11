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
	Messages     []string
	SQL          string
}

type ExecOptions struct {
	ReadOnly       bool
	MaxRows        int
	MaxStatements  int
	UseTransaction bool
	// SQLiteSchema 在同一条连接上执行，并包进事务。执行期间关闭外键强制，避免重建表时级联删除引用行；提交前若出现新的外键错误则回滚。
	SQLiteSchema bool
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
	if options.SQLiteSchema && NormalizeDriver(driver) == "sqlite" {
		return executeSQLiteSchema(ctx, db, statements, options)
	}
	if options.UseTransaction && driver != "mysql" {
		return executeTransactional(ctx, db, driver, statements, options)
	}
	results := make([]Result, 0, len(statements))
	for _, statement := range statements {
		batch, err := runStatement(ctx, db, driver, statement, options.MaxRows)
		if err != nil {
			return results, err
		}
		results = append(results, batch...)
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
		batch, err := runStatement(ctx, tx, driver, statement, options.MaxRows)
		if err != nil {
			return results, err
		}
		results = append(results, batch...)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return results, nil
}

func executeSQLiteSchema(ctx context.Context, db *sql.DB, statements []string, options ExecOptions) ([]Result, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	before, err := foreignKeyCounts(ctx, conn)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return nil, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	results := make([]Result, 0, len(statements))
	for _, statement := range statements {
		batch, err := runStatement(ctx, tx, "sqlite", statement, options.MaxRows)
		if err != nil {
			return results, err
		}
		results = append(results, batch...)
	}
	after, err := foreignKeyCounts(ctx, tx)
	if err != nil {
		return results, err
	}
	if foreignKeysWorse(before, after) {
		return results, fmt.Errorf("更新后外键约束不成立，已回滚")
	}
	if err := tx.Commit(); err != nil {
		return results, err
	}
	committed = true
	return results, nil
}

type fkKey struct {
	table  string
	parent string
	id     int64
}

func foreignKeyCounts(ctx context.Context, query queryer) (map[fkKey]int, error) {
	rows, err := query.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[fkKey]int{}
	for rows.Next() {
		var table, parent string
		var rowid, id int64
		if err := rows.Scan(&table, &rowid, &parent, &id); err != nil {
			return nil, err
		}
		counts[fkKey{table: table, parent: parent, id: id}]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

func foreignKeysWorse(before, after map[fkKey]int) bool {
	for key, count := range after {
		if count > before[key] {
			return true
		}
	}
	return false
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func runStatement(ctx context.Context, db queryer, driver, statement string, maxRows int) ([]Result, error) {
	command := FirstToken(statement)
	query := RewritePlaceholders(driver, statement)
	// SQL Server 的 PRINT 和信息类消息走 TDS INFO。普通 Exec/Query 会丢掉这些标记，必须用消息队列把整句读完。
	if NormalizeDriver(driver) == "sqlserver" {
		results, err := querySQLServer(ctx, db, query, command, maxRows)
		if err != nil {
			return nil, err
		}
		return withSQL(results, statement), nil
	}
	if ReturnsRows(statement) {
		result, err := queryRows(ctx, db, query, command, maxRows)
		if err != nil {
			return nil, err
		}
		return withSQL([]Result{result}, statement), nil
	}
	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	return withSQL([]Result{{Command: command, Columns: []Column{}, Rows: [][]*string{}, RowsAffected: affected}}, statement), nil
}

func withSQL(results []Result, statement string) []Result {
	for i := range results {
		results[i].SQL = statement
	}
	return results
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
	switch baseTypeName(typeName) {
	case "blob", "bytea", "binary", "varbinary", "image", "raw", "tinyblob", "mediumblob", "longblob":
		return true
	default:
		return false
	}
}

func baseTypeName(typeName string) string {
	name := strings.ToLower(strings.TrimSpace(typeName))
	if i := strings.IndexByte(name, '('); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	return name
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

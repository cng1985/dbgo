package shared

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type DumpStats struct {
	Tables     int
	Statements int
	Truncated  bool
}

func DumpSQL(ctx context.Context, db *sql.DB, driver, schema, name string, maxRows int, writer io.Writer) (DumpStats, error) {
	var stats DumpStats
	if maxRows < 1 {
		maxRows = 1000
	}
	if _, err := fmt.Fprintf(writer, "-- dbgo logical backup\n-- driver: %s\n-- name: %s\n-- created_at: %s\n\n", driver, name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return stats, err
	}
	if driver == "mysql" {
		if _, err := io.WriteString(writer, "set session sql_mode = concat(@@sql_mode, ',NO_BACKSLASH_ESCAPES');\n\n"); err != nil {
			return stats, err
		}
	}
	tables, err := listDumpTables(ctx, db, driver, schema)
	if err != nil {
		return stats, err
	}
	if len(tables) > 200 {
		return stats, fmt.Errorf("表数量超过 200，请缩小备份范围")
	}
	for _, table := range tables {
		statements, truncated, err := writeTable(ctx, db, driver, schema, table, maxRows, writer)
		if err != nil {
			return stats, err
		}
		stats.Tables++
		stats.Statements += statements
		if truncated {
			stats.Truncated = true
		}
	}
	return stats, nil
}

func listDumpTables(ctx context.Context, db *sql.DB, driver, schema string) ([]string, error) {
	tables, err := ListTables(ctx, db, driver, schema)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		if strings.EqualFold(table.Type, "VIEW") || strings.EqualFold(table.Type, "view") {
			continue
		}
		names = append(names, table.Name)
	}
	return names, nil
}

func writeTable(ctx context.Context, db *sql.DB, driver, schema, table string, maxRows int, writer io.Writer) (int, bool, error) {
	statements := 0
	ddl, err := tableDDL(ctx, db, driver, schema, table)
	if err != nil {
		return 0, false, err
	}
	if _, err := io.WriteString(writer, ddl+"\n"); err != nil {
		return 0, false, err
	}
	statements++
	columns, err := ListColumns(ctx, db, driver, schema, table)
	if err != nil {
		return statements, false, err
	}
	if len(columns) == 0 {
		return statements, false, nil
	}
	qualified, err := Qualified(driver, schema, table)
	if err != nil {
		return statements, false, err
	}
	quotedColumns := make([]string, len(columns))
	for i, column := range columns {
		quoted, err := QuoteIdent(driver, column.Name)
		if err != nil {
			return statements, false, err
		}
		quotedColumns[i] = quoted
	}
	query := "select " + strings.Join(quotedColumns, ", ") + " from " + qualified
	if len(columns) > 0 && columns[0].Primary {
		query += " order by " + quotedColumns[0]
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return statements, false, err
	}
	defer rows.Close()
	batch := 40
	if len(columns) > 0 && 800/len(columns) < batch {
		batch = 800 / len(columns)
	}
	if batch < 1 {
		batch = 1
	}
	pending := make([]string, 0, batch)
	truncated := false
	written := 0
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		line := "insert into " + qualified + " (" + strings.Join(quotedColumns, ", ") + ") values\n" + strings.Join(pending, ",\n") + ";\n"
		if _, err := io.WriteString(writer, line); err != nil {
			return err
		}
		statements++
		pending = pending[:0]
		return nil
	}
	for rows.Next() {
		if written >= maxRows {
			truncated = true
			break
		}
		raw := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return statements, truncated, err
		}
		values := make([]string, len(columns))
		for i, value := range raw {
			literal, err := literal(driver, value, columns[i].Type)
			if err != nil {
				return statements, truncated, err
			}
			values[i] = literal
		}
		pending = append(pending, "("+strings.Join(values, ", ")+")")
		written++
		if len(pending) >= batch {
			if err := flush(); err != nil {
				return statements, truncated, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return statements, truncated, err
	}
	if err := flush(); err != nil {
		return statements, truncated, err
	}
	if _, err := io.WriteString(writer, "\n"); err != nil {
		return statements, truncated, err
	}
	return statements, truncated, nil
}

func tableDDL(ctx context.Context, db *sql.DB, driver, schema, table string) (string, error) {
	if driver == "sqlite" {
		var ddl string
		err := db.QueryRowContext(ctx, "select sql from sqlite_master where type = 'table' and name = ?", table).Scan(&ddl)
		if err != nil {
			return "", err
		}
		if !strings.HasSuffix(strings.TrimSpace(ddl), ";") {
			ddl += ";"
		}
		return ddl, nil
	}
	columns, err := ListColumns(ctx, db, driver, schema, table)
	if err != nil {
		return "", err
	}
	qualified, err := Qualified(driver, schema, table)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(columns)+1)
	keys := []string{}
	for _, column := range columns {
		quoted, err := QuoteIdent(driver, column.Name)
		if err != nil {
			return "", err
		}
		part := quoted + " " + safeType(column.Type)
		if !column.Nullable {
			part += " not null"
		}
		lines = append(lines, part)
		if column.Primary {
			keys = append(keys, quoted)
		}
	}
	if len(keys) > 0 {
		lines = append(lines, "primary key ("+strings.Join(keys, ", ")+")")
	}
	return "create table " + qualified + " (\n  " + strings.Join(lines, ",\n  ") + "\n);", nil
}

func safeType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return "text"
	}
	for _, ch := range typeName {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == ' ' || ch == '(' || ch == ')' || ch == ',' || ch == '\'' {
			continue
		}
		return "text"
	}
	return typeName
}

func literal(driver string, value any, typeName string) (string, error) {
	if value == nil {
		return "null", nil
	}
	switch typed := value.(type) {
	case bool:
		if driver == "postgres" {
			if typed {
				return "true", nil
			}
			return "false", nil
		}
		if typed {
			return "1", nil
		}
		return "0", nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case time.Time:
		return "'" + typed.UTC().Format(time.RFC3339Nano) + "'", nil
	case []byte:
		if binaryType(typeName) {
			switch driver {
			case "postgres":
				return "'\\x" + fmtHex(typed) + "'", nil
			case "sqlserver":
				return "0x" + fmtHex(typed), nil
			default:
				return "X'" + fmtHex(typed) + "'", nil
			}
		}
		return quoteString(driver, string(typed)), nil
	default:
		return quoteString(driver, fmt.Sprint(typed)), nil
	}
}

func quoteString(driver, value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

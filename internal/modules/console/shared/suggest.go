package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	suggestMaxTables  = 300
	suggestMaxColumns = 6000
)

type SuggestColumn struct {
	Name string
	Type string
}

type SuggestTable struct {
	Schema  string
	Name    string
	Type    string
	Columns []SuggestColumn
}

type SuggestCatalog struct {
	Tables    []SuggestTable
	Truncated bool
}

func ListSuggest(ctx context.Context, db *sql.DB, driver, schema string) (SuggestCatalog, error) {
	switch driver {
	case "sqlite":
		if schema == "" {
			schema = "main"
		}
		return listSuggestSQLite(ctx, db, schema, suggestMaxTables, suggestMaxColumns)
	case "mysql", "postgres", "sqlserver":
		if strings.TrimSpace(schema) == "" {
			return SuggestCatalog{}, fmt.Errorf("缺少模式")
		}
		return listSuggestRemote(ctx, db, driver, schema, suggestMaxTables, suggestMaxColumns)
	default:
		return SuggestCatalog{}, fmt.Errorf("不支持的数据库类型")
	}
}

func listSuggestSQLite(ctx context.Context, db *sql.DB, schema string, maxTables, maxColumns int) (SuggestCatalog, error) {
	tables, err := ListTables(ctx, db, "sqlite", schema)
	if err != nil {
		return SuggestCatalog{}, err
	}
	catalog := SuggestCatalog{Tables: []SuggestTable{}}
	total := 0
	for _, table := range tables {
		if len(catalog.Tables) >= maxTables {
			catalog.Truncated = true
			break
		}
		entry := SuggestTable{
			Schema:  schema,
			Name:    table.Name,
			Type:    normalizeTableType(table.Type),
			Columns: []SuggestColumn{},
		}
		columns, err := ListColumns(ctx, db, "sqlite", schema, table.Name)
		if err != nil {
			return SuggestCatalog{}, err
		}
		for _, column := range columns {
			if total >= maxColumns {
				catalog.Truncated = true
				break
			}
			entry.Columns = append(entry.Columns, SuggestColumn{Name: column.Name, Type: column.Type})
			total++
		}
		catalog.Tables = append(catalog.Tables, entry)
	}
	return catalog, nil
}

const suggestRemoteSQL = `select t.table_name, t.table_type, c.column_name, c.data_type
from information_schema.tables t
left join information_schema.columns c
  on c.table_schema = t.table_schema
 and c.table_name = t.table_name
where t.table_schema = ?
  and upper(t.table_type) in ('BASE TABLE', 'VIEW', 'FOREIGN TABLE')
order by t.table_name, c.ordinal_position`

func suggestRemoteQuery(driver string) string {
	return RewritePlaceholders(driver, suggestRemoteSQL)
}

func listSuggestRemote(ctx context.Context, db *sql.DB, driver, schema string, maxTables, maxColumns int) (SuggestCatalog, error) {
	rows, err := db.QueryContext(ctx, suggestRemoteQuery(driver), schema)
	if err != nil {
		return SuggestCatalog{}, err
	}
	return collectSuggest(rows, schema, maxTables, maxColumns)
}

func collectSuggest(rows *sql.Rows, schema string, maxTables, maxColumns int) (SuggestCatalog, error) {
	defer rows.Close()
	catalog := SuggestCatalog{Tables: []SuggestTable{}}
	index := map[string]int{}
	total := 0
	for rows.Next() {
		var tableName, tableType string
		var columnName, columnType sql.NullString
		if err := rows.Scan(&tableName, &tableType, &columnName, &columnType); err != nil {
			return SuggestCatalog{}, err
		}
		pos, ok := index[tableName]
		if !ok {
			if len(catalog.Tables) >= maxTables {
				catalog.Truncated = true
				break
			}
			catalog.Tables = append(catalog.Tables, SuggestTable{
				Schema:  schema,
				Name:    tableName,
				Type:    normalizeTableType(tableType),
				Columns: []SuggestColumn{},
			})
			pos = len(catalog.Tables) - 1
			index[tableName] = pos
		}
		if !columnName.Valid || strings.TrimSpace(columnName.String) == "" {
			continue
		}
		if total >= maxColumns {
			catalog.Truncated = true
			continue
		}
		catalog.Tables[pos].Columns = append(catalog.Tables[pos].Columns, SuggestColumn{
			Name: columnName.String,
			Type: columnType.String,
		})
		total++
	}
	return catalog, rows.Err()
}

func normalizeTableType(value string) string {
	if strings.Contains(strings.ToUpper(strings.TrimSpace(value)), "VIEW") {
		return "view"
	}
	return "table"
}

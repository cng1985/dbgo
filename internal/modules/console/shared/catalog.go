package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Schema struct {
	Name string
}

type Table struct {
	Schema string
	Name   string
	Type   string
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Default  string
	Primary  bool
	Extra    string
}

type IndexInfo struct {
	Name    string
	Unique  bool
	Columns string
}

func ListSchemas(ctx context.Context, db *sql.DB, driver string) ([]Schema, error) {
	query := ""
	switch driver {
	case "sqlite":
		query = "select name from pragma_database_list order by seq"
	case "mysql":
		query = "select schema_name from information_schema.schemata where schema_name not in ('mysql','information_schema','performance_schema','sys') order by schema_name"
	case "postgres":
		query = "select schema_name from information_schema.schemata where schema_name not in ('pg_catalog','information_schema') and schema_name not like 'pg_%' order by schema_name"
	case "sqlserver":
		query = "select name from sys.schemas where name not in ('sys','guest','INFORMATION_SCHEMA') order by name"
	default:
		return nil, fmt.Errorf("不支持的数据库类型")
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Schema{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, Schema{Name: name})
	}
	return items, rows.Err()
}

func ListTables(ctx context.Context, db *sql.DB, driver, schema string) ([]Table, error) {
	var (
		query string
		args  []any
	)
	switch driver {
	case "sqlite":
		master := "sqlite_master"
		if schema != "" && !strings.EqualFold(schema, "main") {
			quoted, err := QuoteIdent(driver, schema)
			if err != nil {
				return nil, err
			}
			master = quoted + ".sqlite_master"
		}
		query = "select name, type from " + master + " where type in ('table','view') and name not like 'sqlite_%' order by name"
	case "mysql", "postgres", "sqlserver":
		query = RewritePlaceholders(driver, "select table_name, table_type from information_schema.tables where table_schema = ? order by table_name")
		args = []any{schema}
	default:
		return nil, fmt.Errorf("不支持的数据库类型")
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Table{}
	for rows.Next() {
		var item Table
		if err := rows.Scan(&item.Name, &item.Type); err != nil {
			return nil, err
		}
		item.Schema = schema
		items = append(items, item)
	}
	return items, rows.Err()
}

func ListColumns(ctx context.Context, db *sql.DB, driver, schema, table string) ([]ColumnInfo, error) {
	switch driver {
	case "sqlite":
		query := "select name, type, \"notnull\", dflt_value, pk from pragma_table_info(?, ?)"
		schemaName := schema
		if schemaName == "" {
			schemaName = "main"
		}
		rows, err := db.QueryContext(ctx, query, table, schemaName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []ColumnInfo{}
		for rows.Next() {
			var item ColumnInfo
			var notNull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&item.Name, &item.Type, &notNull, &dflt, &pk); err != nil {
				return nil, err
			}
			item.Nullable = notNull == 0
			item.Default = dflt.String
			item.Primary = pk > 0
			items = append(items, item)
		}
		return items, rows.Err()
	default:
		query := RewritePlaceholders(driver, `select column_name, data_type, is_nullable, column_default
			from information_schema.columns
			where table_schema = ? and table_name = ?
			order by ordinal_position`)
		rows, err := db.QueryContext(ctx, query, schema, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []ColumnInfo{}
		for rows.Next() {
			var item ColumnInfo
			var nullable string
			var dflt sql.NullString
			if err := rows.Scan(&item.Name, &item.Type, &nullable, &dflt); err != nil {
				return nil, err
			}
			item.Nullable = strings.EqualFold(nullable, "YES")
			item.Default = dflt.String
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		keys, err := primaryKeys(ctx, db, driver, schema, table)
		if err != nil {
			return nil, err
		}
		keySet := map[string]bool{}
		for _, key := range keys {
			keySet[key] = true
		}
		for i := range items {
			items[i].Primary = keySet[items[i].Name]
		}
		return items, nil
	}
}

func primaryKeys(ctx context.Context, db *sql.DB, driver, schema, table string) ([]string, error) {
	if driver == "sqlite" {
		return nil, nil
	}
	query := RewritePlaceholders(driver, `select column_name
		from information_schema.key_column_usage k
		join information_schema.table_constraints c
		  on c.constraint_name = k.constraint_name
		 and c.table_schema = k.table_schema
		 and c.table_name = k.table_name
		where c.constraint_type = 'PRIMARY KEY' and c.table_schema = ? and c.table_name = ?
		order by k.ordinal_position`)
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		keys = append(keys, name)
	}
	return keys, rows.Err()
}

func ListIndexes(ctx context.Context, db *sql.DB, driver, schema, table string) ([]IndexInfo, error) {
	switch driver {
	case "sqlite":
		rows, err := db.QueryContext(ctx, `select name, "unique" from pragma_index_list(?)`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []IndexInfo{}
		for rows.Next() {
			var item IndexInfo
			var unique int
			if err := rows.Scan(&item.Name, &unique); err != nil {
				return nil, err
			}
			item.Unique = unique == 1
			items = append(items, item)
			if len(items) >= 100 {
				break
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for i := range items {
			columns, err := sqliteIndexColumns(ctx, db, items[i].Name)
			if err != nil {
				return nil, err
			}
			items[i].Columns = strings.Join(columns, ",")
		}
		return items, nil
	case "mysql":
		query := `select index_name, min(non_unique), group_concat(column_name order by seq_in_index)
			from information_schema.statistics
			where table_schema = ? and table_name = ?
			group by index_name
			order by index_name`
		return scanIndexes(ctx, db, driver, query, schema, table, true)
	case "postgres":
		query := RewritePlaceholders(driver, `select indexname, 0, indexdef from pg_indexes where schemaname = ? and tablename = ? order by indexname`)
		return scanIndexes(ctx, db, driver, query, schema, table, false)
	case "sqlserver":
		query := RewritePlaceholders(driver, `select i.name, i.is_unique, c.name
			from sys.indexes i
			join sys.tables t on i.object_id = t.object_id
			join sys.schemas s on t.schema_id = s.schema_id
			join sys.index_columns ic on ic.object_id = i.object_id and ic.index_id = i.index_id
			join sys.columns c on c.object_id = ic.object_id and c.column_id = ic.column_id
			where s.name = ? and t.name = ? and i.name is not null
			order by i.name, ic.key_ordinal`)
		rows, err := db.QueryContext(ctx, query, schema, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		grouped := []IndexInfo{}
		index := map[string]int{}
		for rows.Next() {
			var name, column string
			var unique bool
			if err := rows.Scan(&name, &unique, &column); err != nil {
				return nil, err
			}
			position, ok := index[name]
			if !ok {
				grouped = append(grouped, IndexInfo{Name: name, Unique: unique, Columns: column})
				index[name] = len(grouped) - 1
				continue
			}
			grouped[position].Columns += "," + column
		}
		return grouped, rows.Err()
	default:
		return nil, fmt.Errorf("不支持的数据库类型")
	}
}

func sqliteIndexColumns(ctx context.Context, db *sql.DB, index string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "select name from pragma_index_info(?)", index)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name.Valid {
			columns = append(columns, name.String)
		}
	}
	return columns, rows.Err()
}

func scanIndexes(ctx context.Context, db *sql.DB, driver, query, schema, table string, nonUnique bool) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []IndexInfo{}
	for rows.Next() {
		var item IndexInfo
		var flag int
		if err := rows.Scan(&item.Name, &flag, &item.Columns); err != nil {
			return nil, err
		}
		if nonUnique {
			item.Unique = flag == 0
		} else {
			item.Unique = strings.Contains(strings.ToLower(item.Columns), "unique")
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

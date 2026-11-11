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
	Identity bool
	Extra    string
	Comment  string
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
		return listSQLiteColumns(ctx, db, schema, table)
	case "mysql":
		return listMySQLColumns(ctx, db, schema, table)
	case "postgres":
		return listPostgresColumns(ctx, db, schema, table)
	case "sqlserver":
		return listSQLServerColumns(ctx, db, schema, table)
	default:
		return nil, fmt.Errorf("不支持的数据库类型")
	}
}

func listSQLiteColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	schemaName := schema
	if schemaName == "" {
		schemaName = "main"
	}
	rows, err := db.QueryContext(ctx, `select name, type, "notnull", dflt_value, pk from pragma_table_info(?, ?)`, table, schemaName)
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
		item.Type = strings.TrimSpace(item.Type)
		item.Nullable = notNull == 0
		item.Default = strings.TrimSpace(dflt.String)
		item.Primary = pk > 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sqlText, err := sqliteTableSQL(ctx, db, schemaName, table)
	if err != nil {
		return items, nil
	}
	notes := sqliteColumnNotes(sqlText)
	for i := range items {
		note, ok := noteFor(notes, items[i].Name)
		if !ok {
			continue
		}
		items[i].Comment = note.comment
		items[i].Identity = note.identity
		if note.identity {
			items[i].Extra = "autoincrement"
			if items[i].Primary {
				items[i].Nullable = false
			}
		}
	}
	return items, nil
}

func sqliteTableSQL(ctx context.Context, db *sql.DB, schema, table string) (string, error) {
	master := "sqlite_master"
	if schema != "" && !strings.EqualFold(schema, "main") {
		quoted, err := QuoteIdent("sqlite", schema)
		if err != nil {
			return "", err
		}
		master = quoted + ".sqlite_master"
	}
	var sqlText sql.NullString
	err := db.QueryRowContext(ctx, "select sql from "+master+" where type = 'table' and name = ?", table).Scan(&sqlText)
	if err != nil || !sqlText.Valid {
		return "", err
	}
	return sqlText.String, nil
}

func listMySQLColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	query := RewritePlaceholders("mysql", `select column_name, column_type, is_nullable, column_default, extra, column_comment, column_key
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
		var nullable, extra, comment, key string
		var dflt sql.NullString
		if err := rows.Scan(&item.Name, &item.Type, &nullable, &dflt, &extra, &comment, &key); err != nil {
			return nil, err
		}
		item.Type = strings.TrimSpace(item.Type)
		item.Nullable = strings.EqualFold(nullable, "YES")
		item.Default = strings.TrimSpace(dflt.String)
		item.Extra = strings.TrimSpace(extra)
		item.Comment = strings.TrimSpace(comment)
		item.Primary = strings.EqualFold(key, "PRI")
		item.Identity = strings.Contains(strings.ToLower(item.Extra), "auto_increment")
		items = append(items, item)
	}
	return items, rows.Err()
}

func listPostgresColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	query := RewritePlaceholders("postgres", `select a.attname,
			format_type(a.atttypid, a.atttypmod),
			case when a.attnotnull then 0 else 1 end,
			pg_get_expr(ad.adbin, ad.adrelid),
			a.attidentity,
			col_description(a.attrelid, a.attnum),
			case when exists (
				select 1 from pg_index i
				where i.indrelid = c.oid and i.indisprimary and a.attnum = any (i.indkey)
			) then 1 else 0 end
		from pg_attribute a
		join pg_class c on c.oid = a.attrelid
		join pg_namespace n on n.oid = c.relnamespace
		left join pg_attrdef ad on ad.adrelid = a.attrelid and ad.adnum = a.attnum
		where n.nspname = ? and c.relname = ?
		  and a.attnum > 0 and not a.attisdropped
		  and c.relkind in ('r', 'p', 'v', 'm', 'f')
		order by a.attnum`)
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ColumnInfo{}
	for rows.Next() {
		var item ColumnInfo
		var nullable, primary int
		var dflt, identity, comment sql.NullString
		if err := rows.Scan(&item.Name, &item.Type, &nullable, &dflt, &identity, &comment, &primary); err != nil {
			return nil, err
		}
		item.Type = strings.TrimSpace(item.Type)
		item.Nullable = nullable == 1
		item.Default = strings.TrimSpace(dflt.String)
		item.Comment = strings.TrimSpace(comment.String)
		item.Primary = primary == 1
		switch strings.ToLower(strings.TrimSpace(identity.String)) {
		case "a", "d":
			item.Identity = true
			item.Extra = "identity"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func listSQLServerColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	query := RewritePlaceholders("sqlserver", `select c.name,
			ty.name,
			c.max_length,
			c.precision,
			c.scale,
			c.is_nullable,
			c.is_identity,
			object_definition(c.default_object_id),
			cast(ep.value as nvarchar(max)),
			case when pk.column_id is null then 0 else 1 end
		from sys.columns c
		join sys.objects o on o.object_id = c.object_id
		join sys.schemas s on s.schema_id = o.schema_id
		join sys.types ty on ty.user_type_id = c.user_type_id
		left join sys.extended_properties ep
		  on ep.major_id = c.object_id and ep.minor_id = c.column_id and ep.class = 1 and ep.name = N'MS_Description'
		left join (
			select ic.object_id, ic.column_id
			from sys.indexes i
			join sys.index_columns ic on ic.object_id = i.object_id and ic.index_id = i.index_id
			where i.is_primary_key = 1
		) pk on pk.object_id = c.object_id and pk.column_id = c.column_id
		where s.name = ? and o.name = ? and o.type in ('U', 'V')
		order by c.column_id`)
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ColumnInfo{}
	for rows.Next() {
		var item ColumnInfo
		var typeName string
		var maxLength, precision, scale int64
		var nullable, identity bool
		var dflt, comment sql.NullString
		var primary int
		if err := rows.Scan(&item.Name, &typeName, &maxLength, &precision, &scale, &nullable, &identity, &dflt, &comment, &primary); err != nil {
			return nil, err
		}
		item.Type = formatSQLServerType(typeName, int(maxLength), int(precision), int(scale))
		item.Nullable = nullable
		item.Default = strings.TrimSpace(dflt.String)
		item.Comment = strings.TrimSpace(comment.String)
		item.Primary = primary == 1
		item.Identity = identity
		if identity {
			item.Extra = "identity"
		}
		items = append(items, item)
	}
	return items, rows.Err()
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

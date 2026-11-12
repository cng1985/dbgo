package shared

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrObjectNotFound = errors.New("对象不存在")

type DBObject struct {
	Schema string
	Name   string
	Kind   string
	Detail string
}

func NormalizeKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "all":
		return "all", nil
	case "table", "view", "procedure", "function", "routine":
		return strings.ToLower(strings.TrimSpace(kind)), nil
	default:
		return "", fmt.Errorf("对象类型不受支持")
	}
}

func RoutineSupported(driver string) bool {
	switch NormalizeDriver(driver) {
	case "mysql", "postgres", "sqlserver":
		return true
	default:
		return false
	}
}

func ListObjects(ctx context.Context, db *sql.DB, driver, schema, kind string) ([]DBObject, error) {
	driver = NormalizeDriver(driver)
	kind, err := NormalizeKind(kind)
	if err != nil {
		return nil, err
	}
	var items []DBObject
	if kind == "all" || kind == "table" || kind == "view" {
		tables, err := ListTables(ctx, db, driver, schema)
		if err != nil {
			return nil, err
		}
		for _, table := range tables {
			objectKind := "table"
			if strings.Contains(strings.ToLower(table.Type), "view") {
				objectKind = "view"
			}
			if kind != "all" && kind != objectKind {
				continue
			}
			items = append(items, DBObject{Schema: schema, Name: table.Name, Kind: objectKind, Detail: table.Type})
		}
	}
	if kind == "all" || kind == "routine" || kind == "procedure" || kind == "function" {
		if !RoutineSupported(driver) {
			if items == nil {
				items = []DBObject{}
			}
			return items, nil
		}
		routines, err := listRoutines(ctx, db, driver, schema)
		if err != nil {
			return nil, err
		}
		for _, routine := range routines {
			if kind == "procedure" && routine.Kind != "procedure" {
				continue
			}
			if kind == "function" && routine.Kind != "function" {
				continue
			}
			if kind == "all" && (routine.Kind == "table" || routine.Kind == "view") {
				continue
			}
			items = append(items, routine)
		}
	}
	if items == nil {
		items = []DBObject{}
	}
	return items, nil
}

func listRoutines(ctx context.Context, db *sql.DB, driver, schema string) ([]DBObject, error) {
	var query string
	switch driver {
	case "mysql":
		query = RewritePlaceholders(driver, `select routine_name, lower(routine_type), ''
			from information_schema.routines
			where routine_schema = ?
			order by routine_name`)
	case "postgres":
		query = RewritePlaceholders(driver, `select p.proname,
			case when p.prokind = 'p' then 'procedure' else 'function' end,
			pg_get_function_identity_arguments(p.oid)
			from pg_proc p
			join pg_namespace n on n.oid = p.pronamespace
			where n.nspname = ? and p.prokind in ('f', 'p')
			order by p.proname`)
	case "sqlserver":
		query = RewritePlaceholders(driver, `select o.name,
			case when o.type = 'P' then 'procedure' else 'function' end,
			o.type_desc
			from sys.objects o
			join sys.schemas s on s.schema_id = o.schema_id
			where s.name = ? and o.type in ('P', 'FN', 'IF', 'TF')
			order by o.name`)
	default:
		return []DBObject{}, nil
	}
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DBObject{}
	for rows.Next() {
		var item DBObject
		if err := rows.Scan(&item.Name, &item.Kind, &item.Detail); err != nil {
			return nil, err
		}
		item.Schema = schema
		item.Kind = strings.ToLower(item.Kind)
		items = append(items, item)
	}
	return items, rows.Err()
}

func ObjectDefinition(ctx context.Context, db *sql.DB, driver, schema, name, kind string) (string, string, error) {
	driver = NormalizeDriver(driver)
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "table", "view", "procedure", "function":
	default:
		return "", "", fmt.Errorf("对象类型不受支持")
	}
	if _, err := QuoteIdent(driver, name); err != nil {
		return "", "", fmt.Errorf("对象名无效")
	}
	switch driver {
	case "sqlite":
		return sqliteDefinition(ctx, db, schema, name, kind)
	case "mysql":
		sqlText, err := showCreate(ctx, db, schema, name, kind)
		return sqlText, "", err
	case "postgres":
		return postgresDefinition(ctx, db, schema, name, kind)
	case "sqlserver":
		return sqlServerDefinition(ctx, db, schema, name, kind)
	default:
		return "", "", fmt.Errorf("不支持的数据库类型")
	}
}

func sqliteDefinition(ctx context.Context, db *sql.DB, schema, name, kind string) (string, string, error) {
	if kind == "procedure" || kind == "function" {
		return "", "", fmt.Errorf("SQLite 不支持该对象")
	}
	master := "sqlite_master"
	if schema != "" && !strings.EqualFold(schema, "main") {
		quoted, err := QuoteIdent("sqlite", schema)
		if err != nil {
			return "", "", fmt.Errorf("模式名无效")
		}
		master = quoted + ".sqlite_master"
	}
	var sqlText sql.NullString
	err := db.QueryRowContext(ctx, "select sql from "+master+" where type = ? and name = ?", kind, name).Scan(&sqlText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrObjectNotFound
	}
	if err != nil {
		return "", "", err
	}
	if !sqlText.Valid || strings.TrimSpace(sqlText.String) == "" {
		return "", "", ErrObjectNotFound
	}
	return strings.TrimSpace(sqlText.String), "", nil
}

func showCreate(ctx context.Context, db *sql.DB, schema, name, kind string) (string, error) {
	qualified, err := Qualified("mysql", schema, name)
	if err != nil {
		return "", fmt.Errorf("对象名无效")
	}
	verb := kind
	query := "show create " + verb + " " + qualified
	text, err := longestText(ctx, db, query)
	if err != nil {
		if isMissing(err) {
			return "", ErrObjectNotFound
		}
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", ErrObjectNotFound
	}
	return text, nil
}

func postgresDefinition(ctx context.Context, db *sql.DB, schema, name, kind string) (string, string, error) {
	switch kind {
	case "view":
		var definition sql.NullString
		query := RewritePlaceholders("postgres", "select definition from pg_views where schemaname = ? and viewname = ?")
		err := db.QueryRowContext(ctx, query, schema, name).Scan(&definition)
		if errors.Is(err, sql.ErrNoRows) || !definition.Valid {
			return "", "", ErrObjectNotFound
		}
		if err != nil {
			return "", "", err
		}
		qualified, err := Qualified("postgres", schema, name)
		if err != nil {
			return "", "", fmt.Errorf("对象名无效")
		}
		return "CREATE OR REPLACE VIEW " + qualified + " AS\n" + strings.TrimSpace(definition.String), "", nil
	case "procedure", "function":
		prokind := "f"
		if kind == "procedure" {
			prokind = "p"
		}
		query := RewritePlaceholders("postgres", `select pg_get_functiondef(p.oid)
			from pg_proc p
			join pg_namespace n on n.oid = p.pronamespace
			where n.nspname = ? and p.proname = ? and p.prokind = ?`)
		rows, err := db.QueryContext(ctx, query, schema, name, prokind)
		if err != nil {
			return "", "", err
		}
		defer rows.Close()
		parts := []string{}
		for rows.Next() {
			var text string
			if err := rows.Scan(&text); err != nil {
				return "", "", err
			}
			parts = append(parts, strings.TrimSpace(text))
		}
		if err := rows.Err(); err != nil {
			return "", "", err
		}
		if len(parts) == 0 {
			return "", "", ErrObjectNotFound
		}
		note := ""
		if len(parts) > 1 {
			note = "存在多个重载，已全部列出"
		}
		return strings.Join(parts, "\n\n"), note, nil
	default:
		sqlText, err := reconstructTable(ctx, db, "postgres", schema, name)
		return sqlText, "这是根据字段和索引整理的语句，可能与原始脚本有差别", err
	}
}

func sqlServerDefinition(ctx context.Context, db *sql.DB, schema, name, kind string) (string, string, error) {
	if kind == "table" {
		sqlText, err := reconstructTable(ctx, db, "sqlserver", schema, name)
		return sqlText, "这是根据字段和索引整理的语句，可能与原始脚本有差别", err
	}
	types := []any{schema, name}
	filter := "o.type = 'V'"
	switch kind {
	case "procedure":
		filter = "o.type = 'P'"
	case "function":
		filter = "o.type in ('FN', 'IF', 'TF')"
	}
	query := RewritePlaceholders("sqlserver", `select m.definition
		from sys.sql_modules m
		join sys.objects o on o.object_id = m.object_id
		join sys.schemas s on s.schema_id = o.schema_id
		where s.name = ? and o.name = ? and `+filter)
	text, err := longestText(ctx, db, query, types...)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", "", ErrObjectNotFound
	}
	return text, "", nil
}

func reconstructTable(ctx context.Context, db *sql.DB, driver, schema, name string) (string, error) {
	columns, err := ListColumns(ctx, db, driver, schema, name)
	if err != nil {
		return "", err
	}
	if len(columns) == 0 {
		return "", ErrObjectNotFound
	}
	indexes, err := ListIndexes(ctx, db, driver, schema, name)
	if err != nil {
		return "", err
	}
	drafts := make([]ColumnDraft, 0, len(columns))
	for _, column := range columns {
		drafts = append(drafts, ColumnDraft{
			Name:     column.Name,
			Type:     safeType(column.Type),
			Nullable: column.Nullable,
			Primary:  column.Primary,
			Identity: column.Identity,
			Default:  column.Default,
			Comment:  column.Comment,
		})
	}
	script, err := RenderObject(ObjectDraft{
		Driver:  driver,
		Schema:  schema,
		Kind:    "table",
		Name:    name,
		Columns: drafts,
	})
	if err != nil {
		return "", err
	}
	qualified, err := Qualified(driver, schema, name)
	if err != nil {
		return script, nil
	}
	var builder strings.Builder
	builder.WriteString(script)
	for _, index := range indexes {
		if strings.EqualFold(index.Name, "PRIMARY") || strings.Contains(strings.ToLower(index.Name), "primary") {
			continue
		}
		quoted, err := QuoteIdent(driver, index.Name)
		if err != nil {
			continue
		}
		cols := splitIndexColumns(driver, indexColumnText(index.Columns))
		if len(cols) == 0 {
			continue
		}
		builder.WriteString(";\n")
		if index.Unique {
			builder.WriteString("CREATE UNIQUE INDEX ")
		} else {
			builder.WriteString("CREATE INDEX ")
		}
		builder.WriteString(quoted)
		builder.WriteString(" ON ")
		builder.WriteString(qualified)
		builder.WriteString(" (")
		builder.WriteString(strings.Join(cols, ", "))
		builder.WriteString(")")
	}
	return builder.String(), nil
}

func indexColumnText(columns string) string {
	if start := strings.LastIndex(columns, "("); start >= 0 {
		if end := strings.LastIndex(columns, ")"); end > start {
			return columns[start+1 : end]
		}
	}
	return columns
}

func splitIndexColumns(driver, columns string) []string {
	parts := strings.Split(columns, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if fields := strings.Fields(part); len(fields) > 0 {
			part = fields[0]
		}
		quoted, err := QuoteIdent(driver, part)
		if err != nil {
			continue
		}
		out = append(out, quoted)
	}
	return out
}

func longestText(ctx context.Context, db *sql.DB, query string, args ...any) (string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}
	best := ""
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return "", err
		}
		for _, value := range values {
			if value.Valid && len(value.String) > len(best) {
				best = value.String
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.TrimSpace(best), nil
}

// RoutineMeta 读取存储过程或函数的编号和说明。SQLite 没有这类对象。
// 说明读不到时返回空字符串，不代替定义本身失败。
func RoutineMeta(ctx context.Context, db *sql.DB, driver, schema, name, kind string) (string, string, error) {
	driver = NormalizeDriver(driver)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if driver == "" || driver == "sqlite" || (kind != "procedure" && kind != "function") {
		return "", "", nil
	}
	switch driver {
	case "mysql":
		routineType := "PROCEDURE"
		if kind == "function" {
			routineType = "FUNCTION"
		}
		var comment sql.NullString
		query := RewritePlaceholders(driver, `select routine_comment
			from information_schema.routines
			where routine_schema = ? and routine_name = ? and upper(routine_type) = ?`)
		err := db.QueryRowContext(ctx, query, schema, name, routineType).Scan(&comment)
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		return "", strings.TrimSpace(comment.String), nil
	case "postgres":
		prokind := "f"
		if kind == "procedure" {
			prokind = "p"
		}
		var objectID, comment sql.NullString
		query := RewritePlaceholders(driver, `select p.oid::text, obj_description(p.oid, 'pg_proc')
			from pg_proc p
			join pg_namespace n on n.oid = p.pronamespace
			where n.nspname = ? and p.proname = ? and p.prokind = ?
			limit 1`)
		err := db.QueryRowContext(ctx, query, schema, name, prokind).Scan(&objectID, &comment)
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(objectID.String), strings.TrimSpace(comment.String), nil
	default:
		filter := "o.type = 'P'"
		if kind == "function" {
			filter = "o.type in ('FN', 'IF', 'TF')"
		}
		var objectID, comment sql.NullString
		query := RewritePlaceholders(driver, `select convert(varchar(32), o.object_id), convert(nvarchar(4000), ep.value)
			from sys.objects o
			inner join sys.schemas s on s.schema_id = o.schema_id
			left join sys.extended_properties ep on ep.major_id = o.object_id and ep.minor_id = 0 and ep.name = N'MS_Description'
			where s.name = ? and o.name = ? and `+filter)
		err := db.QueryRowContext(ctx, query, schema, name).Scan(&objectID, &comment)
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(objectID.String), strings.TrimSpace(comment.String), nil
	}
}

func isMissing(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "doesn't exist") || strings.Contains(text, "does not exist") || strings.Contains(text, "unknown table")
}

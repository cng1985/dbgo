package shared

import (
	"fmt"
	"strings"
)

// TableAlter 描述一张已有表要变成的结构。列的 Origin 是修改前的字段名，空字符串表示新增。
type TableAlter struct {
	Driver         string
	Schema         string
	Name           string
	BaselineSchema string
	BaselineName   string
	Baseline       []ColumnDraft
	Columns        []ColumnDraft
}

type keptColumn struct {
	before          ColumnDraft
	after           ColumnDraft
	typeChanged     bool
	nullChanged     bool
	defaultChanged  bool
	identityChanged bool
	commentChanged  bool
	renamed         bool
}

func (item keptColumn) definitionChanged() bool {
	return item.typeChanged || item.nullChanged || item.defaultChanged || item.identityChanged || item.commentChanged
}

type alterPlan struct {
	kept          []keptColumn
	added         []ColumnDraft
	dropped       []ColumnDraft
	pkChanged     bool
	nameChanged   bool
	schemaChanged bool
}

func (plan alterPlan) changed() bool {
	if plan.pkChanged || plan.nameChanged || plan.schemaChanged || len(plan.added) > 0 || len(plan.dropped) > 0 {
		return true
	}
	for _, item := range plan.kept {
		if item.renamed || item.definitionChanged() {
			return true
		}
	}
	return false
}

type nameStep struct {
	from string
	to   string
}

// RenderTableAlter 生成更新表结构的语句，不连接数据库。没有变化时 changed 为 false。
func RenderTableAlter(spec TableAlter) (string, bool, string, error) {
	driver := NormalizeDriver(spec.Driver)
	if driver == "" {
		return "", false, "", fmt.Errorf("数据库类型不受支持")
	}
	spec.Driver = driver
	spec.Name = strings.TrimSpace(spec.Name)
	spec.BaselineName = strings.TrimSpace(spec.BaselineName)
	spec.Schema = strings.TrimSpace(spec.Schema)
	spec.BaselineSchema = strings.TrimSpace(spec.BaselineSchema)
	if spec.BaselineName == "" || spec.Name == "" {
		return "", false, "", fmt.Errorf("表名无效")
	}
	spec.Baseline = normalizeColumns(spec.Baseline)
	spec.Columns = normalizeColumns(spec.Columns)
	plan, err := planTableAlter(spec)
	if err != nil {
		return "", false, "", err
	}
	if !plan.changed() {
		return "", false, "", nil
	}
	var sqlText string
	var note string
	switch driver {
	case "sqlite":
		if sqliteNeedsRebuild(plan) {
			sqlText, err = sqliteRebuild(spec, plan)
			note = "SQLite 会重建这张表，并拷贝能对上的字段。原有索引和触发器不会带到新表。"
		} else {
			sqlText, err = emitIncremental(spec, plan)
		}
	case "sqlserver":
		if sqlserverNeedsRebuild(plan) {
			sqlText, err = sqlserverRebuild(spec, plan)
			note = "SQL Server 不能直接修改自增，语句会新建表并拷贝数据。索引和其它约束不会自动带上。"
		} else {
			sqlText, err = emitIncremental(spec, plan)
		}
	default:
		sqlText, err = emitIncremental(spec, plan)
	}
	if err != nil {
		return "", false, "", err
	}
	if len(plan.dropped) > 0 {
		if note != "" {
			note += "删除的字段不会拷贝。"
		} else {
			note = "删除字段会丢掉这一列已有的数据。"
		}
	}
	return sqlText, true, note, nil
}

func planTableAlter(spec TableAlter) (alterPlan, error) {
	var zero alterPlan
	driver := spec.Driver
	if len(spec.Baseline) == 0 || len(spec.Baseline) > 256 || len(spec.Columns) == 0 || len(spec.Columns) > 256 {
		return zero, fmt.Errorf("表至少要有一列，且不超过 256 列")
	}
	beforeBy := map[string]ColumnDraft{}
	for _, column := range spec.Baseline {
		column = normalizeColumn(column)
		key := foldIdent(driver, column.Name)
		if key == "" {
			return zero, fmt.Errorf("列名无效")
		}
		if _, ok := beforeBy[key]; ok {
			return zero, fmt.Errorf("原字段名重复")
		}
		beforeBy[key] = column
	}
	seenName := map[string]bool{}
	seenOrigin := map[string]bool{}
	var plan alterPlan
	for _, column := range spec.Columns {
		column = normalizeColumn(column)
		nameKey := foldIdent(driver, column.Name)
		if nameKey == "" {
			return zero, fmt.Errorf("列名无效")
		}
		if seenName[nameKey] {
			return zero, fmt.Errorf("列名重复")
		}
		seenName[nameKey] = true
		origin := column.Origin
		if origin == "" {
			if _, ok := beforeBy[nameKey]; ok && !seenOrigin[nameKey] {
				origin = column.Name
			}
		}
		if origin == "" {
			if err := validateNewColumn(column); err != nil {
				return zero, err
			}
			plan.added = append(plan.added, column)
			continue
		}
		originKey := foldIdent(driver, origin)
		if seenOrigin[originKey] {
			return zero, fmt.Errorf("原字段重复")
		}
		seenOrigin[originKey] = true
		before, ok := beforeBy[originKey]
		if !ok {
			return zero, fmt.Errorf("原字段 %s 不存在", origin)
		}
		column.Origin = before.Name
		plan.kept = append(plan.kept, diffKept(driver, before, column))
	}
	for _, before := range beforeBy {
		if !seenOrigin[foldIdent(driver, before.Name)] {
			plan.dropped = append(plan.dropped, before)
		}
	}
	identity := 0
	primary := 0
	for _, column := range spec.Columns {
		column = normalizeColumn(column)
		if column.Identity {
			identity++
		}
		if column.Primary {
			primary++
		}
		if _, err := columnLine(driver, column, false); err != nil {
			return zero, err
		}
	}
	if identity > 1 {
		return zero, fmt.Errorf("一张表只能有一个自增列")
	}
	if driver == "sqlite" && identity == 1 && primary != 1 {
		return zero, fmt.Errorf("SQLite 自增列必须是唯一主键")
	}
	plan.pkChanged = primaryChanged(driver, beforeBy, plan)
	plan.nameChanged = !sameIdent(driver, spec.BaselineName, spec.Name)
	if driver != "sqlite" && spec.BaselineSchema != "" && spec.Schema != "" {
		plan.schemaChanged = !sameIdent(driver, spec.BaselineSchema, spec.Schema)
	}
	return plan, nil
}

func normalizeColumns(columns []ColumnDraft) []ColumnDraft {
	out := make([]ColumnDraft, len(columns))
	for i, column := range columns {
		out[i] = normalizeColumn(column)
	}
	return out
}

func normalizeColumn(column ColumnDraft) ColumnDraft {
	column.Origin = strings.TrimSpace(column.Origin)
	column.Name = strings.TrimSpace(column.Name)
	column.Type = strings.TrimSpace(column.Type)
	column.Default = strings.TrimSpace(column.Default)
	column.Comment = strings.TrimSpace(column.Comment)
	return column
}

func diffKept(driver string, before, after ColumnDraft) keptColumn {
	item := keptColumn{before: before, after: after}
	item.typeChanged = !sameType(before.Type, after.Type)
	item.nullChanged = notNull(before) != notNull(after)
	item.defaultChanged = !sameDefault(before.Default, after.Default)
	item.identityChanged = before.Identity != after.Identity
	item.commentChanged = before.Comment != after.Comment
	item.renamed = !sameIdent(driver, before.Name, after.Name)
	return item
}

func primaryChanged(driver string, beforeBy map[string]ColumnDraft, plan alterPlan) bool {
	before := map[string]bool{}
	for _, column := range beforeBy {
		if column.Primary {
			before[foldIdent(driver, column.Name)] = true
		}
	}
	after := map[string]bool{}
	for _, item := range plan.kept {
		if item.after.Primary {
			after[foldIdent(driver, item.before.Name)] = true
		}
	}
	for _, column := range plan.added {
		if column.Primary {
			return true
		}
	}
	if len(before) != len(after) {
		return true
	}
	for key := range before {
		if !after[key] {
			return true
		}
	}
	return false
}

func validateNewColumn(column ColumnDraft) error {
	if column.Identity || !notNull(column) || !blankDefault(column.Default) {
		return nil
	}
	return fmt.Errorf("新增非空字段 %s 需要默认值", column.Name)
}

func sqliteNeedsRebuild(plan alterPlan) bool {
	if plan.pkChanged {
		return true
	}
	for _, item := range plan.kept {
		if item.definitionChanged() {
			return true
		}
	}
	for _, column := range plan.added {
		if !sqliteAddOK(column) {
			return true
		}
	}
	for _, column := range plan.dropped {
		if column.Primary || column.Identity {
			return true
		}
	}
	return false
}

func sqliteAddOK(column ColumnDraft) bool {
	if column.Primary || column.Identity || strings.TrimSpace(column.Comment) != "" {
		return false
	}
	if strings.Contains(column.Default, "(") {
		return false
	}
	if notNull(column) {
		def, ok, err := formatDefault("sqlite", column.Default)
		if err != nil || !ok {
			return false
		}
		switch strings.ToUpper(def) {
		case "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "NULL":
			return false
		}
	}
	return true
}

func sqlserverNeedsRebuild(plan alterPlan) bool {
	for _, item := range plan.kept {
		if item.identityChanged {
			return true
		}
	}
	return false
}

func emitIncremental(spec TableAlter, plan alterPlan) (string, error) {
	if spec.Driver == "sqlite" {
		for _, item := range plan.kept {
			if item.definitionChanged() {
				return "", fmt.Errorf("SQLite 需要重建表才能修改字段")
			}
		}
	}
	if spec.Driver == "sqlserver" {
		for _, item := range plan.kept {
			if item.identityChanged {
				return "", fmt.Errorf("SQL Server 需要重建表才能修改自增")
			}
		}
	}
	table, err := alterQualified(spec.Driver, spec.BaselineSchema, spec.BaselineName)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	lines := []string{}
	rebind := rebindPrimary(spec.Driver, plan)
	if rebind && hadPrimary(spec.Baseline) {
		drop, err := dropPrimarySQL(spec, table)
		if err != nil {
			return "", err
		}
		if drop != "" {
			lines = append(lines, drop)
		}
	}
	for _, column := range orderedDrops(spec.Driver, spec.Baseline, plan.dropped) {
		quoted, err := QuoteIdent(spec.Driver, column.Name)
		if err != nil {
			return "", fmt.Errorf("列名无效")
		}
		lines = append(lines, "ALTER TABLE "+table+" DROP COLUMN "+quoted)
	}
	steps, err := renameSteps(spec.Driver, plan.kept)
	if err != nil {
		return "", err
	}
	for _, step := range steps {
		sqlText, err := renameColumnSQL(spec, table, step.from, step.to)
		if err != nil {
			return "", err
		}
		lines = append(lines, sqlText)
	}
	for _, item := range plan.kept {
		parts, err := modifyColumnSQL(spec, table, item)
		if err != nil {
			return "", err
		}
		lines = append(lines, parts...)
	}
	for _, column := range plan.added {
		specSQL, err := columnLine(spec.Driver, column, false)
		if err != nil {
			return "", err
		}
		verb := "ADD COLUMN "
		if spec.Driver == "sqlserver" {
			verb = "ADD "
		}
		lines = append(lines, "ALTER TABLE "+table+" "+verb+specSQL)
		if strings.TrimSpace(column.Comment) != "" && (spec.Driver == "postgres" || spec.Driver == "sqlserver") {
			comment, err := columnCommentSQL(spec.Driver, spec.BaselineSchema, spec.BaselineName, column.Name, column.Comment)
			if err != nil {
				return "", err
			}
			lines = append(lines, comment)
		}
	}
	if rebind && hasPrimary(spec.Columns) {
		add, err := addPrimarySQL(spec, table)
		if err != nil {
			return "", err
		}
		lines = append(lines, add)
	}
	move, err := moveTableSQL(spec, plan)
	if err != nil {
		return "", err
	}
	if move != "" {
		lines = append(lines, move)
	}
	return joinSQL(lines), nil
}

func rebindPrimary(driver string, plan alterPlan) bool {
	if plan.pkChanged {
		return true
	}
	if driver != "mysql" && driver != "sqlserver" {
		return false
	}
	for _, item := range plan.kept {
		if item.before.Primary && (item.typeChanged || item.nullChanged || item.identityChanged) {
			return true
		}
	}
	return false
}

func sqliteRebuild(spec TableAlter, plan alterPlan) (string, error) {
	temp, err := temporaryTable(spec.BaselineName, spec.Name)
	if err != nil {
		return "", err
	}
	create, err := renderTable(ObjectDraft{Driver: "sqlite", Kind: "table", Name: temp, Columns: spec.Columns})
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(create))
	builder.WriteString(";\n")
	if len(plan.kept) > 0 {
		insert, err := copyRowsSQL("sqlite", "", temp, spec.BaselineSchema, spec.BaselineName, plan.kept)
		if err != nil {
			return "", err
		}
		builder.WriteString(insert)
		builder.WriteString(";\n")
	}
	oldName, err := QuoteIdent("sqlite", spec.BaselineName)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	tempName, err := QuoteIdent("sqlite", temp)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	finalName, err := QuoteIdent("sqlite", spec.Name)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	builder.WriteString("DROP TABLE ")
	builder.WriteString(oldName)
	builder.WriteString(";\nALTER TABLE ")
	builder.WriteString(tempName)
	builder.WriteString(" RENAME TO ")
	builder.WriteString(finalName)
	return builder.String(), nil
}

func sqlserverRebuild(spec TableAlter, plan alterPlan) (string, error) {
	temp, err := temporaryTable(spec.BaselineName, spec.Name)
	if err != nil {
		return "", err
	}
	schema := physicalSchema("sqlserver", spec.BaselineSchema)
	create, err := renderTable(ObjectDraft{Driver: "sqlserver", Schema: schema, Kind: "table", Name: temp, Columns: spec.Columns})
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(create))
	if !strings.HasSuffix(strings.TrimSpace(create), ";") {
		builder.WriteString(";")
	}
	builder.WriteByte('\n')
	identityInsert := false
	for _, item := range plan.kept {
		if item.after.Identity {
			identityInsert = true
			break
		}
	}
	tempQualified, err := Qualified("sqlserver", schema, temp)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	if len(plan.kept) > 0 {
		if identityInsert {
			builder.WriteString("SET IDENTITY_INSERT ")
			builder.WriteString(tempQualified)
			builder.WriteString(" ON;\n")
		}
		insert, err := copyRowsSQL("sqlserver", schema, temp, spec.BaselineSchema, spec.BaselineName, plan.kept)
		if err != nil {
			return "", err
		}
		builder.WriteString(insert)
		builder.WriteString(";\n")
		if identityInsert {
			builder.WriteString("SET IDENTITY_INSERT ")
			builder.WriteString(tempQualified)
			builder.WriteString(" OFF;\n")
		}
	}
	oldQualified, err := alterQualified("sqlserver", spec.BaselineSchema, spec.BaselineName)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	builder.WriteString("DROP TABLE ")
	builder.WriteString(oldQualified)
	builder.WriteString(";\n")
	finalSchema := schema
	if spec.Schema != "" && !sameIdent("sqlserver", spec.Schema, schema) {
		target, err := QuoteIdent("sqlserver", spec.Schema)
		if err != nil {
			return "", fmt.Errorf("模式名无效")
		}
		source, err := QuoteIdent("sqlserver", schema)
		if err != nil {
			return "", fmt.Errorf("模式名无效")
		}
		tempIdent, err := QuoteIdent("sqlserver", temp)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		builder.WriteString("ALTER SCHEMA ")
		builder.WriteString(target)
		builder.WriteString(" TRANSFER ")
		builder.WriteString(source)
		builder.WriteByte('.')
		builder.WriteString(tempIdent)
		builder.WriteString(";\n")
		finalSchema = spec.Schema
	}
	builder.WriteString("EXEC sp_rename ")
	builder.WriteString(quoteLiteral("sqlserver", finalSchema+"."+temp))
	builder.WriteString(", ")
	builder.WriteString(quoteLiteral("sqlserver", spec.Name))
	builder.WriteString(", N'OBJECT'")
	return builder.String(), nil
}

func copyRowsSQL(driver, targetSchema, target, sourceSchema, source string, kept []keptColumn) (string, error) {
	targets := make([]string, 0, len(kept))
	sources := make([]string, 0, len(kept))
	for _, item := range kept {
		quotedTarget, err := QuoteIdent(driver, item.after.Name)
		if err != nil {
			return "", fmt.Errorf("列名无效")
		}
		quotedSource, err := QuoteIdent(driver, item.before.Name)
		if err != nil {
			return "", fmt.Errorf("列名无效")
		}
		targets = append(targets, quotedTarget)
		sources = append(sources, quotedSource)
	}
	targetName, err := Qualified(driver, physicalSchema(driver, targetSchema), target)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	sourceName, err := alterQualified(driver, sourceSchema, source)
	if err != nil {
		return "", fmt.Errorf("表名无效")
	}
	return "INSERT INTO " + targetName + " (" + strings.Join(targets, ", ") + ")\nSELECT " + strings.Join(sources, ", ") + " FROM " + sourceName, nil
}

func modifyColumnSQL(spec TableAlter, table string, item keptColumn) ([]string, error) {
	switch spec.Driver {
	case "mysql":
		if !item.typeChanged && !item.nullChanged && !item.defaultChanged && !item.identityChanged && !item.commentChanged {
			return nil, nil
		}
		body, err := columnLine(spec.Driver, item.after, false)
		if err != nil {
			return nil, err
		}
		return []string{"ALTER TABLE " + table + " MODIFY COLUMN " + body}, nil
	case "postgres":
		return postgresAlterColumn(table, item)
	case "sqlserver":
		return sqlserverAlterColumn(spec, table, item)
	default:
		return nil, nil
	}
}

func postgresAlterColumn(table string, item keptColumn) ([]string, error) {
	quoted, err := QuoteIdent("postgres", item.after.Name)
	if err != nil {
		return nil, fmt.Errorf("列名无效")
	}
	parts := []string{}
	if item.typeChanged {
		typ, err := checkedType(item.after.Type)
		if err != nil {
			return nil, err
		}
		parts = append(parts, "ALTER COLUMN "+quoted+" TYPE "+typ+" USING "+quoted+"::"+typ)
	}
	if item.nullChanged {
		if notNull(item.after) {
			parts = append(parts, "ALTER COLUMN "+quoted+" SET NOT NULL")
		} else {
			parts = append(parts, "ALTER COLUMN "+quoted+" DROP NOT NULL")
		}
	}
	if item.defaultChanged {
		if blankDefault(item.after.Default) {
			parts = append(parts, "ALTER COLUMN "+quoted+" DROP DEFAULT")
		} else {
			def, ok, err := formatDefault("postgres", item.after.Default)
			if err != nil || !ok {
				return nil, fmt.Errorf("默认值无效")
			}
			parts = append(parts, "ALTER COLUMN "+quoted+" SET DEFAULT "+def)
		}
	}
	if item.identityChanged {
		if item.after.Identity {
			parts = append(parts, "ALTER COLUMN "+quoted+" ADD GENERATED BY DEFAULT AS IDENTITY")
		} else {
			parts = append(parts, "ALTER COLUMN "+quoted+" DROP IDENTITY IF EXISTS")
		}
	}
	lines := []string{}
	if len(parts) > 0 {
		lines = append(lines, "ALTER TABLE "+table+" "+strings.Join(parts, ", "))
	}
	if item.commentChanged {
		comment, err := commentOnQualified("postgres", table, item.after.Name, item.after.Comment)
		if err != nil {
			return nil, err
		}
		lines = append(lines, comment)
	}
	return lines, nil
}

func sqlserverAlterColumn(spec TableAlter, table string, item keptColumn) ([]string, error) {
	lines := []string{}
	if item.typeChanged || item.nullChanged {
		quoted, err := QuoteIdent("sqlserver", item.after.Name)
		if err != nil {
			return nil, fmt.Errorf("列名无效")
		}
		typ, err := checkedType(item.after.Type)
		if err != nil {
			return nil, err
		}
		nullSQL := " NULL"
		if notNull(item.after) {
			nullSQL = " NOT NULL"
		}
		lines = append(lines, "ALTER TABLE "+table+" ALTER COLUMN "+quoted+" "+typ+nullSQL)
	}
	if item.defaultChanged {
		if !blankDefault(item.before.Default) {
			lines = append(lines, sqlserverDropDefault(spec.BaselineSchema, spec.BaselineName, item.after.Name))
		}
		if !blankDefault(item.after.Default) {
			def, ok, err := formatDefault("sqlserver", item.after.Default)
			if err != nil || !ok {
				return nil, fmt.Errorf("默认值无效")
			}
			quoted, err := QuoteIdent("sqlserver", item.after.Name)
			if err != nil {
				return nil, fmt.Errorf("列名无效")
			}
			lines = append(lines, "ALTER TABLE "+table+" ADD DEFAULT "+def+" FOR "+quoted)
		}
	}
	if item.commentChanged {
		comment, err := columnCommentSQL("sqlserver", spec.BaselineSchema, spec.BaselineName, item.after.Name, item.after.Comment)
		if err != nil {
			return nil, err
		}
		lines = append(lines, comment)
	}
	return lines, nil
}

func renameSteps(driver string, kept []keptColumn) ([]nameStep, error) {
	pending := []nameStep{}
	occupied := map[string]bool{}
	finals := map[string]bool{}
	for _, item := range kept {
		occupied[foldIdent(driver, item.before.Name)] = true
		finals[foldIdent(driver, item.after.Name)] = true
		if item.renamed {
			pending = append(pending, nameStep{from: item.before.Name, to: item.after.Name})
		}
	}
	steps := []nameStep{}
	for guard := 0; len(pending) > 0; guard++ {
		if guard > 10000 {
			return nil, fmt.Errorf("无法安排字段重命名")
		}
		next := make([]nameStep, 0, len(pending))
		progressed := false
		for _, item := range pending {
			if occupied[foldIdent(driver, item.to)] && !sameIdent(driver, item.from, item.to) {
				next = append(next, item)
				continue
			}
			steps = append(steps, item)
			delete(occupied, foldIdent(driver, item.from))
			occupied[foldIdent(driver, item.to)] = true
			progressed = true
		}
		pending = next
		if len(pending) == 0 || progressed {
			continue
		}
		temp, err := freshColumn(driver, occupied, finals)
		if err != nil {
			return nil, err
		}
		steps = append(steps, nameStep{from: pending[0].from, to: temp})
		delete(occupied, foldIdent(driver, pending[0].from))
		occupied[foldIdent(driver, temp)] = true
		pending[0].from = temp
	}
	return steps, nil
}

func freshColumn(driver string, occupied, finals map[string]bool) (string, error) {
	for i := 1; i < 1000; i++ {
		name := fmt.Sprintf("__dbgo_col_%d", i)
		key := foldIdent(driver, name)
		if !occupied[key] && !finals[key] {
			return name, nil
		}
	}
	return "", fmt.Errorf("无法生成临时列名")
}

func renameColumnSQL(spec TableAlter, table, from, to string) (string, error) {
	fromName, err := QuoteIdent(spec.Driver, from)
	if err != nil {
		return "", fmt.Errorf("列名无效")
	}
	toName, err := QuoteIdent(spec.Driver, to)
	if err != nil {
		return "", fmt.Errorf("列名无效")
	}
	if spec.Driver == "sqlserver" {
		schema := physicalSchema(spec.Driver, spec.BaselineSchema)
		objectName := schema + "." + spec.BaselineName + "." + from
		return "EXEC sp_rename " + quoteLiteral(spec.Driver, objectName) + ", " + quoteLiteral(spec.Driver, to) + ", N'COLUMN'", nil
	}
	return "ALTER TABLE " + table + " RENAME COLUMN " + fromName + " TO " + toName, nil
}

func dropPrimarySQL(spec TableAlter, table string) (string, error) {
	switch spec.Driver {
	case "mysql":
		return "ALTER TABLE " + table + " DROP PRIMARY KEY", nil
	case "postgres":
		return postgresDropPrimary(spec), nil
	case "sqlserver":
		return sqlserverDropPrimary(spec), nil
	default:
		return "", nil
	}
}

func postgresDropPrimary(spec TableAlter) string {
	schemaExpr := "current_schema()"
	schemaLiteral := "current_schema()"
	if spec.BaselineSchema != "" {
		quoted := quoteLiteral("postgres", spec.BaselineSchema)
		schemaExpr = quoted
		schemaLiteral = quoted
	}
	tableLiteral := quoteLiteral("postgres", spec.BaselineName)
	return "DO $dbgo$\nDECLARE cname text;\nBEGIN\n  SELECT con.conname INTO cname\n" +
		"  FROM pg_constraint con\n  JOIN pg_class rel ON rel.oid = con.conrelid\n" +
		"  JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace\n" +
		"  WHERE con.contype = 'p' AND rel.relname = " + tableLiteral + " AND nsp.nspname = " + schemaExpr + ";\n" +
		"  IF cname IS NOT NULL THEN\n" +
		"    EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT %I', " + schemaLiteral + ", " + tableLiteral + ", cname);\n" +
		"  END IF;\nEND\n$dbgo$"
}

func sqlserverDropPrimary(spec TableAlter) string {
	schema := quoteLiteral("sqlserver", physicalSchema("sqlserver", spec.BaselineSchema))
	table := quoteLiteral("sqlserver", spec.BaselineName)
	return "BEGIN\n  DECLARE @dbgo_sql nvarchar(max);\n" +
		"  SET @dbgo_sql = (\n" +
		"    SELECT N'ALTER TABLE ' + QUOTENAME(s.name) + N'.' + QUOTENAME(tb.name) + N' DROP CONSTRAINT ' + QUOTENAME(kc.name)\n" +
		"    FROM sys.key_constraints kc\n" +
		"    INNER JOIN sys.tables tb ON tb.object_id = kc.parent_object_id\n" +
		"    INNER JOIN sys.schemas s ON s.schema_id = tb.schema_id\n" +
		"    WHERE kc.type = 'PK' AND s.name = " + schema + " AND tb.name = " + table + "\n" +
		"  );\n" +
		"  IF @dbgo_sql IS NOT NULL EXEC sp_executesql @dbgo_sql;\nEND"
}

func addPrimarySQL(spec TableAlter, table string) (string, error) {
	names := []string{}
	for _, column := range spec.Columns {
		column = normalizeColumn(column)
		if !column.Primary {
			continue
		}
		quoted, err := QuoteIdent(spec.Driver, column.Name)
		if err != nil {
			return "", fmt.Errorf("列名无效")
		}
		names = append(names, quoted)
	}
	if len(names) == 0 {
		return "", nil
	}
	return "ALTER TABLE " + table + " ADD PRIMARY KEY (" + strings.Join(names, ", ") + ")", nil
}

func columnCommentSQL(driver, schema, table, column, comment string) (string, error) {
	switch driver {
	case "postgres":
		qualified, err := Qualified(driver, schema, table)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		return commentOnQualified(driver, qualified, column, comment)
	case "sqlserver":
		text, err := normalizeComment(comment)
		if err != nil {
			return "", err
		}
		return sqlserverComment(schema, table, column, text), nil
	default:
		return "", nil
	}
}

func commentOnQualified(driver, qualifiedTable, column, comment string) (string, error) {
	comment, err := normalizeComment(comment)
	if err != nil {
		return "", err
	}
	quoted, err := QuoteIdent(driver, column)
	if err != nil {
		return "", fmt.Errorf("列名无效")
	}
	value := "NULL"
	if comment != "" {
		value = quoteLiteral(driver, comment)
	}
	return "COMMENT ON COLUMN " + qualifiedTable + "." + quoted + " IS " + value, nil
}

func sqlserverComment(schema, table, column, comment string) string {
	schemaName := physicalSchema("sqlserver", schema)
	schemaLit := quoteLiteral("sqlserver", schemaName)
	tableLit := quoteLiteral("sqlserver", table)
	columnLit := quoteLiteral("sqlserver", column)
	levels := "@level0type = N'SCHEMA', @level0name = " + schemaLit +
		", @level1type = N'TABLE', @level1name = " + tableLit +
		", @level2type = N'COLUMN', @level2name = " + columnLit
	exists := "EXISTS (\n    SELECT 1\n    FROM sys.extended_properties ep\n" +
		"    INNER JOIN sys.tables tb ON tb.object_id = ep.major_id\n" +
		"    INNER JOIN sys.schemas s ON s.schema_id = tb.schema_id\n" +
		"    INNER JOIN sys.columns c ON c.object_id = tb.object_id AND c.column_id = ep.minor_id\n" +
		"    WHERE ep.name = N'MS_Description' AND ep.class = 1 AND s.name = " + schemaLit +
		" AND tb.name = " + tableLit + " AND c.name = " + columnLit + "\n  )"
	if comment == "" {
		return "BEGIN\n  IF " + exists + "\n    EXEC sp_dropextendedproperty @name = N'MS_Description', " + levels + ";\nEND"
	}
	value := quoteLiteral("sqlserver", comment)
	return "BEGIN\n  IF " + exists + "\n" +
		"    EXEC sp_updateextendedproperty @name = N'MS_Description', @value = " + value + ", " + levels + ";\n" +
		"  ELSE\n" +
		"    EXEC sp_addextendedproperty @name = N'MS_Description', @value = " + value + ", " + levels + ";\nEND"
}

func sqlserverDropDefault(schema, table, column string) string {
	schemaLit := quoteLiteral("sqlserver", physicalSchema("sqlserver", schema))
	tableLit := quoteLiteral("sqlserver", table)
	columnLit := quoteLiteral("sqlserver", column)
	qualified, _ := alterQualified("sqlserver", schema, table)
	return "BEGIN\n  DECLARE @dbgo_df nvarchar(128);\n  DECLARE @dbgo_sql nvarchar(max);\n" +
		"  SET @dbgo_df = (\n" +
		"    SELECT dc.name\n" +
		"    FROM sys.default_constraints dc\n" +
		"    INNER JOIN sys.columns c ON c.object_id = dc.parent_object_id AND c.column_id = dc.parent_column_id\n" +
		"    INNER JOIN sys.tables tb ON tb.object_id = c.object_id\n" +
		"    INNER JOIN sys.schemas s ON s.schema_id = tb.schema_id\n" +
		"    WHERE s.name = " + schemaLit + " AND tb.name = " + tableLit + " AND c.name = " + columnLit + "\n" +
		"  );\n" +
		"  IF @dbgo_df IS NOT NULL\n" +
		"  BEGIN\n" +
		"    SET @dbgo_sql = N'ALTER TABLE " + strings.ReplaceAll(qualified, "'", "''") + " DROP CONSTRAINT ' + QUOTENAME(@dbgo_df);\n" +
		"    EXEC sp_executesql @dbgo_sql;\n" +
		"  END\nEND"
}

func moveTableSQL(spec TableAlter, plan alterPlan) (string, error) {
	if !plan.nameChanged && !plan.schemaChanged {
		return "", nil
	}
	switch spec.Driver {
	case "sqlite":
		if !plan.nameChanged {
			return "", nil
		}
		from, err := QuoteIdent(spec.Driver, spec.BaselineName)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		to, err := QuoteIdent(spec.Driver, spec.Name)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		return "ALTER TABLE " + from + " RENAME TO " + to, nil
	case "mysql":
		from, err := Qualified(spec.Driver, spec.BaselineSchema, spec.BaselineName)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		to, err := Qualified(spec.Driver, spec.Schema, spec.Name)
		if err != nil {
			return "", fmt.Errorf("表名无效")
		}
		return "RENAME TABLE " + from + " TO " + to, nil
	case "postgres":
		lines := []string{}
		schema := spec.BaselineSchema
		name := spec.BaselineName
		if plan.schemaChanged {
			table, err := Qualified(spec.Driver, schema, name)
			if err != nil {
				return "", fmt.Errorf("表名无效")
			}
			target, err := QuoteIdent(spec.Driver, spec.Schema)
			if err != nil {
				return "", fmt.Errorf("模式名无效")
			}
			lines = append(lines, "ALTER TABLE "+table+" SET SCHEMA "+target)
			schema = spec.Schema
		}
		if plan.nameChanged {
			table, err := Qualified(spec.Driver, schema, name)
			if err != nil {
				return "", fmt.Errorf("表名无效")
			}
			target, err := QuoteIdent(spec.Driver, spec.Name)
			if err != nil {
				return "", fmt.Errorf("表名无效")
			}
			lines = append(lines, "ALTER TABLE "+table+" RENAME TO "+target)
		}
		return joinSQL(lines), nil
	default:
		lines := []string{}
		schema := physicalSchema(spec.Driver, spec.BaselineSchema)
		name := spec.BaselineName
		if plan.schemaChanged {
			target, err := QuoteIdent(spec.Driver, spec.Schema)
			if err != nil {
				return "", fmt.Errorf("模式名无效")
			}
			source, err := QuoteIdent(spec.Driver, schema)
			if err != nil {
				return "", fmt.Errorf("模式名无效")
			}
			objectName, err := QuoteIdent(spec.Driver, name)
			if err != nil {
				return "", fmt.Errorf("表名无效")
			}
			lines = append(lines, "ALTER SCHEMA "+target+" TRANSFER "+source+"."+objectName)
			schema = spec.Schema
		}
		if plan.nameChanged {
			lines = append(lines, "EXEC sp_rename "+quoteLiteral(spec.Driver, schema+"."+name)+", "+quoteLiteral(spec.Driver, spec.Name)+", N'OBJECT'")
		}
		return joinSQL(lines), nil
	}
}

func orderedDrops(driver string, baseline, dropped []ColumnDraft) []ColumnDraft {
	gone := map[string]ColumnDraft{}
	for _, column := range dropped {
		gone[foldIdent(driver, column.Name)] = column
	}
	ordered := make([]ColumnDraft, 0, len(dropped))
	seen := map[string]bool{}
	for _, column := range baseline {
		key := foldIdent(driver, column.Name)
		item, ok := gone[key]
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		ordered = append(ordered, item)
	}
	return ordered
}

func temporaryTable(current, next string) (string, error) {
	for i := 1; i < 100; i++ {
		name := fmt.Sprintf("__dbgo_new_%d", i)
		if !strings.EqualFold(name, current) && name != next {
			return name, nil
		}
	}
	return "", fmt.Errorf("无法生成临时表名")
}

func alterQualified(driver, schema, name string) (string, error) {
	return Qualified(driver, physicalSchema(driver, schema), name)
}

func physicalSchema(driver, schema string) string {
	schema = strings.TrimSpace(schema)
	if driver == "sqlserver" && schema == "" {
		return "dbo"
	}
	return schema
}

func hadPrimary(columns []ColumnDraft) bool {
	for _, column := range columns {
		if column.Primary {
			return true
		}
	}
	return false
}

func hasPrimary(columns []ColumnDraft) bool {
	for _, column := range columns {
		if normalizeColumn(column).Primary {
			return true
		}
	}
	return false
}

func notNull(column ColumnDraft) bool {
	return !column.Nullable || column.Primary
}

func blankDefault(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.EqualFold(value, "NULL")
}

func sameDefault(before, after string) bool {
	return strings.TrimSpace(before) == strings.TrimSpace(after)
}

func sameType(before, after string) bool {
	return strings.EqualFold(collapseSpace(before), collapseSpace(after))
}

func collapseSpace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func sameIdent(driver, before, after string) bool {
	if driver == "postgres" {
		return before == after
	}
	return strings.EqualFold(before, after)
}

func foldIdent(driver, name string) string {
	name = strings.TrimSpace(name)
	if driver == "postgres" {
		return name
	}
	return strings.ToLower(name)
}

func joinSQL(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, ";\n")
}

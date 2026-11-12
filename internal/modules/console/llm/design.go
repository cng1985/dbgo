package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
)

type tableDraft struct {
	Summary string        `json:"summary"`
	Name    string        `json:"name"`
	Comment string        `json:"comment"`
	Notes   []string      `json:"notes"`
	Columns []draftColumn `json:"columns"`
}

type draftColumn struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Nullable optionalBool `json:"nullable"`
	Primary  optionalBool `json:"primary"`
	Identity optionalBool `json:"identity"`
	Default  string       `json:"default"`
	Comment  string       `json:"comment"`
}

type optionalBool struct {
	Value bool
	Set   bool
}

func (b *optionalBool) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.ToLower(strings.TrimSpace(string(data))), "\"")
	b.Set = true
	b.Value = text == "true" || text == "1" || text == "yes"
	return nil
}

func designMessages(driver, schema, prompt, tables string) []Message {
	var user strings.Builder
	user.WriteString("数据库类型：")
	user.WriteString(driver)
	user.WriteByte('\n')
	if schema != "" {
		user.WriteString("当前模式：")
		user.WriteString(schema)
		user.WriteByte('\n')
	}
	if tables != "" {
		user.WriteString("已有表，仅供参考命名，不要修改这些表：\n")
		user.WriteString(tables)
		user.WriteByte('\n')
	}
	user.WriteString("建表描述：\n")
	user.WriteString(prompt)
	return []Message{
		{Role: "system", Content: "你是数据库表结构设计助手。只设计一张新表，只输出一个 JSON 对象，不要使用 Markdown，不要输出 SQL。字段为 summary（中文说明）、name（英文蛇形表名）、comment（表说明）、notes（字符串数组）和 columns（字段数组）。每个字段包含 name、type、nullable、primary、identity、default、comment。type 必须从给定列表中原样选择。必须有一个主键。自增列最多一个，且必须是主键。不要发明外键约束。可用类型：" + strings.Join(columnTypesFor(driver), "、")},
		{Role: "user", Content: user.String()},
	}
}

func parseDesign(content string) (tableDraft, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return tableDraft{}, fmt.Errorf("模型没有返回可识别的表结构")
	}
	var parsed tableDraft
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil {
		return tableDraft{}, fmt.Errorf("模型没有返回可识别的表结构")
	}
	return parsed, nil
}

func normalizeDesign(driver string, raw tableDraft, existing []string) (DesignTableResponse, error) {
	driver = shared.NormalizeDriver(driver)
	if driver == "" {
		return DesignTableResponse{}, fmt.Errorf("当前连接的数据库类型无法建表")
	}
	name, ok := cleanIdent(raw.Name)
	if !ok {
		return DesignTableResponse{}, fmt.Errorf("模型没有给出可用的表名")
	}
	notes := make([]string, 0, 8)
	for _, item := range raw.Notes {
		item = cleanComment(item)
		if item == "" {
			continue
		}
		notes = append(notes, item)
		if len(notes) == 6 {
			break
		}
	}
	type pendingColumn struct {
		name     string
		typ      string
		nullable bool
		primary  bool
		identity bool
		def      string
		comment  string
	}
	pending := make([]pendingColumn, 0, len(raw.Columns))
	seen := map[string]struct{}{}
	for _, item := range raw.Columns {
		if len(pending) >= 40 {
			notes = append(notes, "只保留了前 40 个字段。")
			break
		}
		columnName, valid := cleanIdent(item.Name)
		if !valid {
			label := cleanComment(item.Name)
			if label == "" {
				notes = append(notes, "有一个字段没有名称，已忽略。")
			} else {
				notes = append(notes, "字段名 "+label+" 无效，已忽略。")
			}
			continue
		}
		key := strings.ToLower(columnName)
		if _, exists := seen[key]; exists {
			notes = append(notes, "字段 "+columnName+" 与已有字段重复，已忽略。")
			continue
		}
		seen[key] = struct{}{}
		mapped, recognized := matchDesignType(driver, item.Type)
		if !recognized {
			shown := cleanComment(item.Type)
			if shown == "" {
				notes = append(notes, "字段 "+columnName+" 没有类型，已使用 "+mapped+"。")
			} else {
				notes = append(notes, "字段 "+columnName+" 的类型 "+shown+" 已改成 "+mapped+"。")
			}
		}
		nullable := true
		if item.Nullable.Set {
			nullable = item.Nullable.Value
		}
		pending = append(pending, pendingColumn{
			name:     columnName,
			typ:      mapped,
			nullable: nullable,
			primary:  item.Primary.Value,
			identity: item.Identity.Value,
			def:      strings.TrimSpace(item.Default),
			comment:  cleanComment(item.Comment),
		})
	}
	if len(pending) == 0 {
		return DesignTableResponse{}, fmt.Errorf("模型没有给出可用的字段")
	}
	identityAt := -1
	for i := range pending {
		if !pending[i].identity {
			continue
		}
		if identityAt >= 0 {
			pending[i].identity = false
			notes = append(notes, "字段 "+pending[i].name+" 的自增已去掉，只保留一个自增列。")
			continue
		}
		nextType := identityType(driver, pending[i].typ)
		if nextType != pending[i].typ {
			notes = append(notes, "自增字段 "+pending[i].name+" 已改成 "+nextType+"。")
			pending[i].typ = nextType
		}
		pending[i].primary = true
		pending[i].nullable = false
		identityAt = i
	}
	if driver == "sqlite" && identityAt >= 0 {
		for i := range pending {
			if i != identityAt && pending[i].primary {
				pending[i].primary = false
				notes = append(notes, "SQLite 自增列必须是唯一主键，已取消字段 "+pending[i].name+" 的主键。")
			}
		}
	}
	hasPrimary := false
	for _, column := range pending {
		if column.primary {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary {
		pending[0].primary = true
		pending[0].nullable = false
		notes = append(notes, "已把 "+pending[0].name+" 设为主键。")
	}
	columns := make([]DesignColumn, 0, len(pending))
	for _, column := range pending {
		if column.primary {
			column.nullable = false
		}
		if column.identity && column.def != "" {
			column.def = ""
			notes = append(notes, "自增字段 "+column.name+" 不需要默认值，已去掉。")
		}
		if column.def != "" {
			cleaned, keep := cleanDefault(column.def)
			if !keep {
				notes = append(notes, "字段 "+column.name+" 的默认值无法使用，已去掉。")
				column.def = ""
			} else if cleaned != column.def {
				notes = append(notes, "字段 "+column.name+" 的默认值已改成 "+cleaned+"。")
				column.def = cleaned
			}
		}
		columns = append(columns, DesignColumn{
			Name:     column.name,
			Type:     column.typ,
			Nullable: column.nullable,
			Primary:  column.primary,
			Identity: column.identity,
			Default:  column.def,
			Comment:  column.comment,
		})
	}
	if len(notes) > 11 {
		notes = notes[:11]
	}
	if nameTaken(existing, name) {
		notes = append(notes, "已有同名表，执行前需要勾选「替换已有对象」，否则建表会失败。")
	}
	summary := cleanComment(raw.Summary)
	if summary == "" {
		summary = "已根据描述整理出表结构，确认后再执行。"
	}
	return DesignTableResponse{
		Summary: summary,
		Name:    name,
		Comment: cleanComment(raw.Comment),
		Columns: columns,
		Notes:   notes,
	}, nil
}

func columnTypesFor(driver string) []string {
	switch shared.NormalizeDriver(driver) {
	case "mysql":
		return []string{
			"tinyint", "smallint", "int", "bigint", "decimal(10,2)", "float", "double", "bit(1)", "boolean",
			"char(32)", "varchar(64)", "varchar(255)", "text", "mediumtext", "longtext", "blob",
			"date", "time", "datetime", "timestamp", "year", "json",
		}
	case "postgres":
		return []string{
			"smallint", "integer", "bigint", "numeric", "real", "double precision", "boolean",
			"char(32)", "varchar(255)", "text", "bytea", "date", "time", "timestamp", "timestamptz",
			"json", "jsonb", "uuid",
		}
	case "sqlserver":
		return []string{
			"tinyint", "smallint", "int", "bigint", "bit", "decimal(18,2)", "float", "real", "money",
			"char(32)", "varchar(255)", "nchar(32)", "nvarchar(255)", "text", "ntext",
			"date", "time", "datetime", "datetime2", "datetimeoffset", "uniqueidentifier", "varbinary(max)", "xml",
		}
	default:
		return []string{"integer", "real", "text", "blob", "numeric"}
	}
}

func defaultColumnType(driver string) string {
	switch shared.NormalizeDriver(driver) {
	case "mysql", "sqlserver":
		return "int"
	default:
		return "integer"
	}
}

func matchDesignType(driver, raw string) (string, bool) {
	options := columnTypesFor(driver)
	raw = strings.TrimSpace(raw)
	if found, ok := findType(options, raw); ok {
		return found, true
	}
	base := typeBase(raw)
	if found, ok := findType(options, base); ok {
		return found, true
	}
	for _, candidate := range typeAliases[base] {
		if found, ok := findType(options, candidate); ok {
			return found, false
		}
	}
	return defaultColumnType(driver), false
}

func findType(options []string, want string) (string, bool) {
	if want == "" {
		return "", false
	}
	for _, item := range options {
		if strings.EqualFold(item, want) {
			return item, true
		}
	}
	return "", false
}

func typeBase(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if index := strings.IndexByte(raw, '('); index >= 0 {
		raw = strings.TrimSpace(raw[:index])
	}
	return raw
}

func identityType(driver, current string) string {
	if shared.NormalizeDriver(driver) == "sqlite" {
		return "integer"
	}
	switch typeBase(current) {
	case "tinyint", "smallint", "int", "integer", "bigint":
		return current
	}
	if shared.NormalizeDriver(driver) == "postgres" {
		return "bigint"
	}
	return "bigint"
}

func cleanIdent(name string) (string, bool) {
	name = strings.Join(strings.Fields(strings.TrimSpace(name)), "_")
	if _, err := shared.QuoteIdent("sqlite", name); err != nil {
		return "", false
	}
	return name, true
}

func cleanComment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return value
}

func cleanDefault(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	if utf8.RuneCountInString(value) > 80 || strings.ContainsAny(value, ";\n\r") || strings.Contains(value, "--") || strings.Contains(value, "/*") {
		return "", false
	}
	switch strings.ToUpper(value) {
	case "NULL", "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "TRUE", "FALSE":
		return strings.ToUpper(value), true
	case "NOW()", "GETDATE()", "SYSDATE", "CURRENT_TIMESTAMP()":
		return "CURRENT_TIMESTAMP", true
	}
	if strings.ContainsAny(value, "()") {
		return "", false
	}
	return value, true
}

func nameTaken(existing []string, name string) bool {
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item), name) {
			return true
		}
	}
	return false
}

var typeAliases = map[string][]string{
	"int":              {"int", "integer", "bigint"},
	"integer":          {"integer", "int", "bigint"},
	"int4":             {"integer", "int", "bigint"},
	"int8":             {"bigint", "integer", "int"},
	"bigint":           {"bigint", "integer", "int"},
	"bigserial":        {"bigint", "integer", "int"},
	"serial":           {"integer", "int", "bigint"},
	"smallint":         {"smallint", "integer", "int"},
	"int2":             {"smallint", "integer", "int"},
	"tinyint":          {"tinyint", "smallint", "int"},
	"bool":             {"boolean", "bit", "bit(1)", "integer"},
	"boolean":          {"boolean", "bit", "bit(1)", "integer"},
	"bit":              {"bit", "bit(1)", "boolean", "integer"},
	"text":             {"text", "varchar(255)", "nvarchar(255)"},
	"string":           {"varchar(255)", "nvarchar(255)", "text"},
	"varchar":          {"varchar(255)", "varchar(64)", "nvarchar(255)", "text"},
	"nvarchar":         {"nvarchar(255)", "varchar(255)", "text"},
	"char":             {"char(32)", "varchar(255)", "text"},
	"nchar":            {"nchar(32)", "nvarchar(255)", "char(32)", "text"},
	"clob":             {"text", "varchar(255)", "nvarchar(255)"},
	"decimal":          {"decimal(10,2)", "decimal(18,2)", "numeric"},
	"numeric":          {"numeric", "decimal(10,2)", "decimal(18,2)"},
	"money":            {"money", "decimal(18,2)", "decimal(10,2)", "numeric"},
	"float":            {"float", "real", "double precision", "double"},
	"double":           {"double", "double precision", "float", "real"},
	"real":             {"real", "float", "double precision"},
	"double precision": {"double precision", "double", "float", "real"},
	"date":             {"date", "text"},
	"time":             {"time", "text"},
	"datetime":         {"datetime", "timestamp", "datetime2", "text"},
	"datetime2":        {"datetime2", "datetime", "timestamp"},
	"timestamp":        {"timestamp", "datetime", "timestamptz", "datetime2", "text"},
	"timestamptz":      {"timestamptz", "timestamp", "datetime"},
	"datetimeoffset":   {"datetimeoffset", "datetime2", "timestamp"},
	"json":             {"json", "jsonb", "text"},
	"jsonb":            {"jsonb", "json", "text"},
	"uuid":             {"uuid", "uniqueidentifier", "varchar(255)", "text"},
	"uniqueidentifier": {"uniqueidentifier", "uuid", "varchar(255)", "text"},
	"blob":             {"blob", "bytea", "varbinary(max)"},
	"bytea":            {"bytea", "blob", "varbinary(max)"},
	"varbinary":        {"varbinary(max)", "blob", "bytea"},
	"binary":           {"varbinary(max)", "blob", "bytea"},
	"xml":              {"xml", "text"},
	"year":             {"year", "int", "integer"},
}

package llm

import (
	"encoding/json"
	"strings"
)

type advice struct {
	Summary string   `json:"summary"`
	SQL     string   `json:"sql"`
	Notes   []string `json:"notes"`
}

func optimizeMessages(driver, schema, sqlText, tables string) []Message {
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
		user.WriteString("可用结构：\n")
		user.WriteString(tables)
		user.WriteByte('\n')
	} else {
		user.WriteString("没有读到表结构，只根据语句本身给出建议。\n")
	}
	user.WriteString("原始 SQL：\n")
	user.WriteString(sqlText)
	return []Message{
		{Role: "system", Content: "你是数据库性能顾问。只输出一个 JSON 对象，不要使用 Markdown。字段为 summary（中文说明）、sql（优化后的单条 SQL，必须沿用已给出的表和字段）和 notes（字符串数组，列出风险或索引建议）。不要发明不存在的表。如果原语句已经合适，sql 可以保持原样并在 summary 里说明。"},
		{Role: "user", Content: user.String()},
	}
}

func parseAdvice(content string) (advice, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}
	var parsed advice
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return advice{Summary: clip(content), Notes: []string{}}, err
	}
	parsed.Summary = strings.TrimSpace(parsed.Summary)
	parsed.SQL = strings.TrimSpace(parsed.SQL)
	if parsed.Notes == nil {
		parsed.Notes = []string{}
	}
	return parsed, nil
}

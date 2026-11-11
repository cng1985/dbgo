package etl

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

type Spec struct {
	SourceSQL string
	Driver    string
	Schema    string
	Table     string
	Mode      string
	Mappings  []ColumnMap
	MaxRows   int
	BatchSize int
}

type Outcome struct {
	Columns     []string
	Rows        [][]*string
	RowsRead    int64
	RowsWritten int64
	Truncated   bool
}

// Transfer 先把抽取结果读完，再在目标库的一个事务里写入。失败时目标表回到执行前。
func Transfer(ctx context.Context, source, target *sql.DB, spec Spec) (Outcome, error) {
	var outcome Outcome
	limit := spec.MaxRows
	if limit <= 0 {
		limit = maxExtractRows
	}
	columns, values, extra, err := readRows(ctx, source, spec.SourceSQL, limit, true)
	if err != nil {
		return outcome, err
	}
	if extra {
		return outcome, apperrors.Invalid(fmt.Sprintf("抽取结果超过 %d 行，已停止且没有写入", limit))
	}
	outcome.Columns = columns
	outcome.RowsRead = int64(len(values))
	plan, err := buildPlan(columns, spec.Mappings)
	if err != nil {
		return outcome, err
	}
	qualified, err := shared.Qualified(shared.NormalizeDriver(spec.Driver), strings.TrimSpace(spec.Schema), strings.TrimSpace(spec.Table))
	if err != nil {
		return outcome, apperrors.Invalid("目标表名无效")
	}
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return outcome, apperrors.Business(codeETL, apperrors.KindUnavailable, shared.PublicError("打开目标事务失败", "", err), err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if spec.Mode == "replace" {
		if _, err := tx.ExecContext(ctx, "delete from "+qualified); err != nil {
			return outcome, apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("清空目标表失败", "", err), err)
		}
	}
	written, err := insertRows(ctx, tx, spec.Driver, qualified, plan, values, spec.BatchSize)
	if err != nil {
		return outcome, err
	}
	if err := tx.Commit(); err != nil {
		return outcome, apperrors.Business(codeETL, apperrors.KindUnavailable, shared.PublicError("提交目标事务失败", "", err), err)
	}
	committed = true
	outcome.RowsWritten = written
	return outcome, nil
}

func Preview(ctx context.Context, source *sql.DB, sqlText string, limit int) (Outcome, error) {
	var outcome Outcome
	if limit <= 0 {
		limit = previewRows
	}
	columns, values, extra, err := readRows(ctx, source, sqlText, limit, false)
	if err != nil {
		return outcome, err
	}
	rows := make([][]*string, len(values))
	for i, line := range values {
		rows[i] = make([]*string, len(line))
		for j, cell := range line {
			rows[i][j] = cellText(cell)
		}
	}
	outcome.Columns = columns
	outcome.Rows = rows
	outcome.RowsRead = int64(len(rows))
	outcome.Truncated = extra
	return outcome, nil
}

type columnPlan struct {
	quoted  []string
	indexes []int
}

func buildPlan(columns []string, mappings []ColumnMap) (columnPlan, error) {
	if len(columns) == 0 {
		return columnPlan{}, apperrors.Invalid("抽取语句没有返回字段")
	}
	exact := map[string]int{}
	exactCount := map[string]int{}
	folded := map[string][]int{}
	for i, name := range columns {
		exactCount[name]++
		if _, ok := exact[name]; !ok {
			exact[name] = i
		}
		key := strings.ToLower(name)
		folded[key] = append(folded[key], i)
	}
	if len(mappings) == 0 {
		mappings = make([]ColumnMap, 0, len(columns))
		for _, name := range columns {
			if exactCount[name] > 1 {
				return columnPlan{}, apperrors.Invalid("抽取结果里字段 " + name + " 重复")
			}
			mappings = append(mappings, ColumnMap{Source: name, Target: name})
		}
	}
	plan := columnPlan{quoted: make([]string, 0, len(mappings)), indexes: make([]int, 0, len(mappings))}
	for _, item := range mappings {
		index, err := resolveColumn(item.Source, exact, exactCount, folded)
		if err != nil {
			return columnPlan{}, err
		}
		plan.indexes = append(plan.indexes, index)
		plan.quoted = append(plan.quoted, item.Target)
	}
	return plan, nil
}

func resolveColumn(source string, exact map[string]int, exactCount map[string]int, folded map[string][]int) (int, error) {
	if exactCount[source] > 1 {
		return 0, apperrors.Invalid("字段 " + source + " 与多个结果列同名")
	}
	if exactCount[source] == 1 {
		return exact[source], nil
	}
	matches := folded[strings.ToLower(source)]
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return 0, apperrors.Invalid("抽取结果里没有字段 " + source)
	}
	return 0, apperrors.Invalid("字段 " + source + " 与多个只差大小写的结果列对应，请写成和结果一致的名字")
}

func readRows(ctx context.Context, db *sql.DB, query string, limit int, strict bool) ([]string, [][]any, bool, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, false, apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("抽取失败", "", err), err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("读取字段失败", "", err), err)
	}
	if len(columns) == 0 {
		return nil, nil, false, apperrors.Invalid("抽取语句没有返回字段")
	}
	capacity := limit
	if capacity < 0 {
		capacity = 0
	}
	read := make([][]any, 0, capacity)
	extra := false
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, nil, false, err
		}
		if len(read) >= limit {
			extra = true
			if strict {
				break
			}
			break
		}
		raw := make([]any, len(columns))
		holders := make([]any, len(columns))
		for i := range raw {
			holders[i] = &raw[i]
		}
		if err := rows.Scan(holders...); err != nil {
			return nil, nil, false, apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("读取抽取结果失败", "", err), err)
		}
		line := make([]any, len(columns))
		for i, value := range raw {
			line[i] = copyCell(value)
		}
		read = append(read, line)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("读取抽取结果失败", "", err), err)
	}
	return columns, read, extra, nil
}

func copyCell(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []byte:
		cloned := make([]byte, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return typed
	}
}

func cellText(value any) *string {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []byte:
		text := string(typed)
		return &text
	case string:
		text := typed
		return &text
	case time.Time:
		text := typed.UTC().Format(time.RFC3339Nano)
		return &text
	default:
		text := fmt.Sprint(typed)
		return &text
	}
}

func insertRows(ctx context.Context, tx *sql.Tx, driver, table string, plan columnPlan, rows [][]any, batchSize int) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	quoted := make([]string, len(plan.quoted))
	normalized := shared.NormalizeDriver(driver)
	for i, name := range plan.quoted {
		text, err := shared.QuoteIdent(normalized, name)
		if err != nil {
			return 0, apperrors.Invalid("目标字段 " + name + " 无效")
		}
		quoted[i] = text
	}
	if batchSize <= 0 {
		batchSize = insertBatch
	}
	if width := len(quoted); width > 0 {
		limit := 2000 / width
		if limit < 1 {
			limit = 1
		}
		if batchSize > limit {
			batchSize = limit
		}
	}
	var written int64
	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		if err := insertBatchRows(ctx, tx, normalized, table, quoted, plan.indexes, rows[start:end]); err != nil {
			return written, err
		}
		written += int64(end - start)
	}
	return written, nil
}

func insertBatchRows(ctx context.Context, tx *sql.Tx, driver, table string, columns []string, indexes []int, rows [][]any) error {
	var builder strings.Builder
	builder.WriteString("insert into ")
	builder.WriteString(table)
	builder.WriteString(" (")
	builder.WriteString(strings.Join(columns, ", "))
	builder.WriteString(") values ")
	args := make([]any, 0, len(rows)*len(indexes))
	for i, row := range rows {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteByte('(')
		for j, index := range indexes {
			if j > 0 {
				builder.WriteString(", ")
			}
			builder.WriteByte('?')
			if index < 0 || index >= len(row) {
				args = append(args, nil)
				continue
			}
			args = append(args, row[index])
		}
		builder.WriteByte(')')
	}
	query := shared.RewritePlaceholders(driver, builder.String())
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return apperrors.Business(codeETL, apperrors.KindInvalid, shared.PublicError("写入目标表失败", "", err), err)
	}
	return nil
}

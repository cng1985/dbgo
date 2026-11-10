package domain

import (
	"strings"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
	MaxOffset       = 100000
)

// NormalizePage 把未填写的页码补成默认值。JSON 数字无法区分省略和显式 0，
// 因此 0 与负数都按未填写处理；超过上限的 page_size 视为非法。
func NormalizePage(page, pageSize int) (int, int, error) {
	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		return 0, 0, apperrors.Invalid("每页数量不能超过 200")
	}
	if int64(page-1) > int64(MaxOffset)/int64(pageSize) {
		return 0, 0, apperrors.Invalid("分页范围超出限制")
	}
	return page, pageSize, nil
}

// OrderBy 只接受白名单字段。非主键排序追加 id，保证分页顺序稳定。
func OrderBy(field, direction string, allowed map[string]string, fallback string) (string, error) {
	key := field
	if strings.TrimSpace(key) == "" {
		key = fallback
	}
	column, ok := allowed[key]
	if !ok || column == "" {
		return "", apperrors.Invalid("排序字段不在允许范围内")
	}
	dir := strings.ToLower(strings.TrimSpace(direction))
	switch dir {
	case "", "desc":
		dir = "desc"
	case "asc":
	default:
		return "", apperrors.Invalid("排序方向只能是 asc 或 desc")
	}
	if column == "id" {
		return "id " + dir, nil
	}
	return column + " " + dir + ", id desc", nil
}

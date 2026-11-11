package snippet

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

const snippetDocumentFormat = "dbgo-snippets"

type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *SnippetRequest) (response.Response[SnippetResponse], error) {
	var zero response.Response[SnippetResponse]
	scope, err := r.authorize(ctx, "snippet:write")
	if err != nil {
		return zero, err
	}
	if err := response.RejectClientID(requestID(request)); err != nil {
		return zero, err
	}
	normalized, err := normalizeSnippet(request)
	if err != nil {
		return zero, err
	}
	total, err := r.repo.Count(ctx, scope.ActorID)
	if err != nil {
		return zero, err
	}
	if total >= 500 {
		return zero, apperrors.Invalid("代码段已有 500 个，请先清理后再新建")
	}
	now := time.Now().UTC()
	entity := &Snippet{
		Name:        normalized.Name,
		Description: normalized.Description,
		Body:        normalized.Body,
		Driver:      normalized.Driver,
		Version:     1,
		CreatedBy:   scope.ActorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return r.repo.Create(ctx, entity)
}

func (r *Resource) Update(ctx context.Context, request *SnippetRequest) (response.Response[SnippetResponse], error) {
	var zero response.Response[SnippetResponse]
	scope, err := r.authorize(ctx, "snippet:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 || request.Version <= 0 {
		return zero, apperrors.Invalid("修改代码段需要有效主键和版本")
	}
	normalized, err := normalizeSnippet(request)
	if err != nil {
		return zero, err
	}
	fields := map[string]any{
		"name":        normalized.Name,
		"description": normalized.Description,
		"body":        normalized.Body,
		"driver":      normalized.Driver,
	}
	return r.repo.Update(ctx, scope.ActorID, request, fields)
}

func (r *Resource) One(ctx context.Context, request *SnippetRequest) (response.Response[SnippetResponse], error) {
	var zero response.Response[SnippetResponse]
	scope, err := r.authorize(ctx, "snippet:read")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	entity, err := r.repo.One(ctx, scope.ActorID, request.ID)
	if err != nil {
		return zero, err
	}
	return response.OK(toResponse(entity)), nil
}

func (r *Resource) Delete(ctx context.Context, request *SnippetDeleteRequest) (response.Response[SnippetResponse], error) {
	var zero response.Response[SnippetResponse]
	scope, err := r.authorize(ctx, "snippet:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	return r.repo.Delete(ctx, scope.ActorID, request.ID)
}

func (r *Resource) Search(ctx context.Context, search *SnippetSearch) (response.PageResponse[SnippetSimple], error) {
	var zero response.PageResponse[SnippetSimple]
	scope, err := r.authorize(ctx, "snippet:read")
	if err != nil {
		return zero, err
	}
	if search == nil {
		search = &SnippetSearch{}
	}
	return r.repo.Search(ctx, scope.ActorID, search)
}

func (r *Resource) Export(ctx context.Context, request *SnippetExportRequest) (response.Response[SnippetDocument], error) {
	var zero response.Response[SnippetDocument]
	scope, err := r.authorize(ctx, "snippet:read")
	if err != nil {
		return zero, err
	}
	if request == nil {
		request = &SnippetExportRequest{}
	}
	ids, err := parseIDs(request.IDs)
	if err != nil {
		return zero, err
	}
	if len(ids) == 0 {
		total, err := r.repo.Count(ctx, scope.ActorID)
		if err != nil {
			return zero, err
		}
		if total > 500 {
			return zero, apperrors.Invalid("代码段超过 500 个，请按主键分批导出")
		}
	}
	entities, err := r.repo.List(ctx, scope.ActorID, ids)
	if err != nil {
		return zero, err
	}
	if len(ids) > 0 && len(entities) != len(uniqueIDs(ids)) {
		return zero, apperrors.NotFound("有代码段不存在")
	}
	items := make([]SnippetPortable, 0, len(entities))
	for i := range entities {
		items = append(items, SnippetPortable{
			Name:        entities[i].Name,
			Description: entities[i].Description,
			Body:        entities[i].Body,
			Driver:      entities[i].Driver,
		})
	}
	return response.OK(SnippetDocument{
		Format:     snippetDocumentFormat,
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Snippets:   items,
	}), nil
}

func (r *Resource) Import(ctx context.Context, request *SnippetImportRequest) (response.Response[SnippetImportResponse], error) {
	var zero response.Response[SnippetImportResponse]
	scope, err := r.authorize(ctx, "snippet:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.Document == nil {
		return zero, apperrors.Invalid("请提供代码段文件")
	}
	if request.Document.Format != snippetDocumentFormat || request.Document.Version != 1 {
		return zero, apperrors.Invalid("代码段文件格式不受支持")
	}
	if len(request.Document.Snippets) == 0 {
		return zero, apperrors.Invalid("代码段文件里没有代码段")
	}
	if len(request.Document.Snippets) > 200 {
		return zero, apperrors.Invalid("一次最多导入 200 个代码段")
	}
	mode := strings.ToLower(strings.TrimSpace(request.OnConflict))
	if mode == "" {
		mode = "rename"
	}
	switch mode {
	case "skip", "rename", "error":
	default:
		return zero, apperrors.Invalid("重名策略只支持 skip、rename、error")
	}
	existing, err := r.repo.List(ctx, scope.ActorID, nil)
	if err != nil {
		return zero, err
	}
	if int64(len(existing)) >= 500 {
		return zero, apperrors.Invalid("代码段已有 500 个，请先清理后再导入")
	}
	used := map[string]bool{}
	for i := range existing {
		used[existing[i].Name] = true
	}
	now := time.Now().UTC()
	entities := make([]*Snippet, 0, len(request.Document.Snippets))
	skipped := 0
	for _, item := range request.Document.Snippets {
		normalized, err := normalizeSnippet(&SnippetRequest{
			Name:        item.Name,
			Description: item.Description,
			Body:        item.Body,
			Driver:      item.Driver,
		})
		if err != nil {
			return zero, err
		}
		name := normalized.Name
		if used[name] {
			switch mode {
			case "skip":
				skipped++
				continue
			case "error":
				return zero, apperrors.Conflict("代码段名称已存在：" + name)
			default:
				name = uniqueName(name, used)
			}
		}
		if name == "" {
			return zero, apperrors.Invalid("代码段名称无效")
		}
		used[name] = true
		entities = append(entities, &Snippet{
			Name:        name,
			Description: normalized.Description,
			Body:        normalized.Body,
			Driver:      normalized.Driver,
			Version:     1,
			CreatedBy:   scope.ActorID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	if len(entities) == 0 {
		return response.OK(SnippetImportResponse{Created: 0, Skipped: skipped, Items: []SnippetSimple{}}), nil
	}
	if int64(len(existing)+len(entities)) > 500 {
		return zero, apperrors.Invalid("导入后代码段会超过 500 个")
	}
	if err := r.repo.CreateMany(ctx, entities); err != nil {
		return zero, err
	}
	items := make([]SnippetSimple, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toSimple(entity))
	}
	return response.OK(SnippetImportResponse{Created: len(entities), Skipped: skipped, Items: items}), nil
}

type normalizedSnippet struct {
	Name        string
	Description string
	Body        string
	Driver      string
}

func normalizeSnippet(request *SnippetRequest) (normalizedSnippet, error) {
	var zero normalizedSnippet
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	name, err := response.TrimRequired(request.Name, "代码段名称", 80)
	if err != nil {
		return zero, err
	}
	description, err := response.TrimLimit(request.Description, "说明", 500)
	if err != nil {
		return zero, err
	}
	body := strings.TrimSpace(request.Body)
	if body == "" {
		return zero, apperrors.Invalid("代码段内容不能为空")
	}
	if len(body) > 100000 {
		return zero, apperrors.Invalid("代码段内容过长")
	}
	driver := strings.TrimSpace(request.Driver)
	if driver != "" {
		driver = shared.NormalizeDriver(driver)
		if driver == "" {
			return zero, apperrors.Invalid("数据库类型只支持 sqlite、mysql、postgres、sqlserver")
		}
	}
	return normalizedSnippet{Name: name, Description: description, Body: body, Driver: driver}, nil
}

func (r *Resource) authorize(ctx context.Context, permission string) (domain.Scope, error) {
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return domain.Scope{}, err
	}
	if err := scope.Allow(permission); err != nil {
		return domain.Scope{}, err
	}
	return scope, nil
}

func requestID(request *SnippetRequest) int64 {
	if request == nil {
		return 0
	}
	return request.ID
}

func parseIDs(raw []string) ([]int64, error) {
	if len(raw) > 500 {
		return nil, apperrors.Invalid("一次最多导出 500 个代码段")
	}
	ids := make([]int64, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, apperrors.Invalid("代码段主键无效")
		}
		id, err := strconv.ParseInt(item, 10, 64)
		if err != nil || id <= 0 {
			return nil, apperrors.Invalid("代码段主键无效")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func uniqueName(base string, used map[string]bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "导入代码段"
	}
	if !used[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		suffix := fmt.Sprintf(" (%d)", i)
		candidate := base + suffix
		if len([]rune(candidate)) > 80 {
			keep := 80 - len([]rune(suffix))
			if keep < 1 {
				keep = 1
			}
			runes := []rune(base)
			if keep > len(runes) {
				keep = len(runes)
			}
			candidate = string(runes[:keep]) + suffix
		}
		if !used[candidate] {
			return candidate
		}
	}
	return ""
}

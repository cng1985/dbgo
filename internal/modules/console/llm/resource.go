package llm

import (
	"context"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
	"github.com/cng1985/dbgo/internal/platform/secret"
)

type Resource struct {
	repo   *Repository
	box    *secret.Box
	opener shared.Opener
}

func NewResource(repo *Repository, box *secret.Box, opener shared.Opener) *Resource {
	return &Resource{repo: repo, box: box, opener: opener}
}

func (r *Resource) Create(ctx context.Context, request *ProfileRequest) (response.Response[ProfileResponse], error) {
	var zero response.Response[ProfileResponse]
	scope, err := r.authorize(ctx, "llm:write")
	if err != nil {
		return zero, err
	}
	if err := response.RejectClientID(requestID(request)); err != nil {
		return zero, err
	}
	normalized, err := normalizeProfile(request, false)
	if err != nil {
		return zero, err
	}
	cipher, err := r.box.Encrypt(normalized.APIKey)
	if err != nil {
		return zero, apperrors.Internal(err)
	}
	now := time.Now().UTC()
	entity := &Profile{
		Name:         normalized.Name,
		BaseURL:      normalized.BaseURL,
		APIKeyCipher: cipher,
		Model:        normalized.Model,
		Temperature:  normalized.Temperature,
		MaxTokens:    normalized.MaxTokens,
		IsDefault:    normalized.IsDefault,
		Remark:       normalized.Remark,
		Version:      1,
		CreatedBy:    scope.ActorID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return r.repo.Create(ctx, entity)
}

func (r *Resource) Update(ctx context.Context, request *ProfileRequest) (response.Response[ProfileResponse], error) {
	var zero response.Response[ProfileResponse]
	if _, err := r.authorize(ctx, "llm:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 || request.Version <= 0 {
		return zero, apperrors.Invalid("修改模型需要有效主键和版本")
	}
	current, err := r.repo.One(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	keepKey := strings.TrimSpace(request.APIKey) == ""
	normalized, err := normalizeProfile(request, keepKey)
	if err != nil {
		return zero, err
	}
	cipher := current.APIKeyCipher
	if !keepKey {
		cipher, err = r.box.Encrypt(normalized.APIKey)
		if err != nil {
			return zero, apperrors.Internal(err)
		}
	}
	fields := map[string]any{
		"name":           normalized.Name,
		"base_url":       normalized.BaseURL,
		"api_key_cipher": cipher,
		"model":          normalized.Model,
		"temperature":    normalized.Temperature,
		"max_tokens":     normalized.MaxTokens,
		"is_default":     normalized.IsDefault,
		"remark":         normalized.Remark,
	}
	return r.repo.Update(ctx, request.ID, request.Version, fields, normalized.IsDefault)
}

func (r *Resource) One(ctx context.Context, request *ProfileRequest) (response.Response[ProfileResponse], error) {
	var zero response.Response[ProfileResponse]
	if _, err := r.authorize(ctx, "llm:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	entity, err := r.repo.One(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	return response.OK(toResponse(entity)), nil
}

func (r *Resource) Delete(ctx context.Context, request *ProfileDeleteRequest) (response.Response[ProfileResponse], error) {
	var zero response.Response[ProfileResponse]
	if _, err := r.authorize(ctx, "llm:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	return r.repo.Delete(ctx, request.ID)
}

func (r *Resource) Search(ctx context.Context, search *ProfileSearch) (response.PageResponse[ProfileSimple], error) {
	var zero response.PageResponse[ProfileSimple]
	if _, err := r.authorize(ctx, "llm:read"); err != nil {
		return zero, err
	}
	if search == nil {
		search = &ProfileSearch{}
	}
	return r.repo.Search(ctx, search)
}

func (r *Resource) Test(ctx context.Context, request *ProfileRequest) (response.Response[TestResponse], error) {
	var zero response.Response[TestResponse]
	if _, err := r.authorize(ctx, "llm:read"); err != nil {
		return zero, err
	}
	endpoint, _, err := r.endpoint(ctx, requestID(request))
	if err != nil {
		return zero, err
	}
	started := time.Now()
	detail, err := Probe(ctx, endpoint)
	if err != nil {
		return zero, apperrors.Business(10005, apperrors.KindUnavailable, "模型服务不可用："+clip(err.Error()), err)
	}
	return response.OK(TestResponse{OK: true, Detail: detail, LatencyMS: time.Since(started).Milliseconds()}), nil
}

func (r *Resource) Optimize(ctx context.Context, request *OptimizeRequest) (response.Response[OptimizeResponse], error) {
	var zero response.Response[OptimizeResponse]
	if _, err := r.authorize(ctx, "llm:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择连接")
	}
	sqlText := strings.TrimSpace(request.SQL)
	if sqlText == "" {
		return zero, apperrors.Invalid("SQL 不能为空")
	}
	if len([]rune(sqlText)) > 20000 {
		return zero, apperrors.TooLarge("SQL 超过优化长度限制")
	}
	target, err := r.opener.Open(ctx, request.ConnectionID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	schema := strings.TrimSpace(request.Schema)
	if schema == "" {
		schema = target.Schema
	}
	tables := ""
	catalog, err := shared.ListSuggest(ctx, target.DB, target.Driver, schema)
	if err != nil {
		tables = ""
	} else {
		tables = schemaBrief(catalog)
	}
	endpoint, profile, err := r.endpoint(ctx, request.ProfileID)
	if err != nil {
		return zero, err
	}
	content, err := Complete(ctx, endpoint, optimizeMessages(target.Driver, schema, sqlText, tables))
	if err != nil {
		return zero, apperrors.Business(10005, apperrors.KindUnavailable, "模型优化失败："+clip(err.Error()), err)
	}
	parsed, parseErr := parseAdvice(content)
	if parseErr != nil && parsed.Summary == "" {
		parsed.Summary = "模型返回的内容无法解析为优化建议"
	}
	if parsed.Notes == nil {
		parsed.Notes = []string{}
	}
	return response.OK(OptimizeResponse{
		ProfileName: profile.Name,
		Model:       profile.Model,
		Summary:     parsed.Summary,
		SQL:         parsed.SQL,
		Notes:       parsed.Notes,
	}), nil
}

func (r *Resource) endpoint(ctx context.Context, id int64) (Endpoint, *Profile, error) {
	var entity *Profile
	var err error
	if id > 0 {
		entity, err = r.repo.One(ctx, id)
	} else {
		entity, err = r.repo.FindDefault(ctx)
	}
	if err != nil {
		return Endpoint{}, nil, err
	}
	key, err := r.box.Decrypt(entity.APIKeyCipher)
	if err != nil {
		return Endpoint{}, nil, apperrors.Unavailable("模型密钥无法解密", err)
	}
	return Endpoint{
		BaseURL:     entity.BaseURL,
		APIKey:      key,
		Model:       entity.Model,
		Temperature: entity.Temperature,
		MaxTokens:   entity.MaxTokens,
	}, entity, nil
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

type normalizedProfile struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	IsDefault   bool
	Remark      string
}

func normalizeProfile(request *ProfileRequest, keepKey bool) (normalizedProfile, error) {
	var zero normalizedProfile
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	name, err := response.TrimRequired(request.Name, "名称", 80)
	if err != nil {
		return zero, err
	}
	baseURL, err := NormalizeBaseURL(request.BaseURL)
	if err != nil {
		return zero, err
	}
	model, err := response.TrimRequired(request.Model, "模型", 120)
	if err != nil {
		return zero, err
	}
	remark, err := response.TrimLimit(request.Remark, "备注", 500)
	if err != nil {
		return zero, err
	}
	temperature := request.Temperature
	if temperature == 0 && request.MaxTokens == 0 && request.Name != "" {
		temperature = 0.2
	}
	if temperature < 0 || temperature > 2 {
		return zero, apperrors.Invalid("温度需要在 0 到 2 之间")
	}
	maxTokens := request.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1200
	}
	if maxTokens < 1 || maxTokens > 8192 {
		return zero, apperrors.Invalid("最大令牌需要在 1 到 8192 之间")
	}
	apiKey := ""
	if !keepKey {
		apiKey = strings.TrimSpace(request.APIKey)
		if len(apiKey) > 4000 {
			return zero, apperrors.Invalid("密钥过长")
		}
	}
	return normalizedProfile{
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		IsDefault:   request.IsDefault,
		Remark:      remark,
	}, nil
}

func requestID(request *ProfileRequest) int64 {
	if request == nil {
		return 0
	}
	return request.ID
}

func schemaBrief(catalog shared.SuggestCatalog) string {
	var builder strings.Builder
	count := 0
	for _, table := range catalog.Tables {
		if count >= 40 {
			builder.WriteString("…\n")
			break
		}
		count++
		builder.WriteString(table.Name)
		builder.WriteByte('(')
		limit := len(table.Columns)
		if limit > 12 {
			limit = 12
		}
		for i := 0; i < limit; i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(table.Columns[i].Name)
			if table.Columns[i].Type != "" {
				builder.WriteByte(' ')
				builder.WriteString(table.Columns[i].Type)
			}
		}
		builder.WriteString(")\n")
	}
	return builder.String()
}

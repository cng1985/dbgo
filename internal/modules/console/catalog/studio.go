package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

func (r *Resource) Objects(ctx context.Context, search *CatalogSearch) (response.Response[CatalogObjectsResponse], error) {
	var zero response.Response[CatalogObjectsResponse]
	target, schema, err := r.openSchema(ctx, search)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	kind := ""
	if search != nil {
		kind = search.Kind
	}
	items, err := shared.ListObjects(ctx, target.DB, target.Driver, schema, kind)
	if err != nil {
		return zero, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取对象失败", "", err), err)
	}
	objects := make([]CatalogObject, 0, len(items))
	for _, item := range items {
		objects = append(objects, CatalogObject{Schema: item.Schema, Name: item.Name, Kind: item.Kind, Detail: item.Detail})
	}
	supported := shared.RoutineSupported(target.Driver)
	if kind == "table" || kind == "view" {
		supported = true
	}
	return response.OK(CatalogObjectsResponse{Driver: target.Driver, Schema: schema, Objects: objects, Supported: supported}), nil
}

func (r *Resource) Definition(ctx context.Context, search *CatalogSearch) (response.Response[CatalogDefinitionResponse], error) {
	var zero response.Response[CatalogDefinitionResponse]
	target, schema, err := r.openSchema(ctx, search)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	if search == nil || strings.TrimSpace(search.Name) == "" {
		target.Close()
		return zero, apperrors.Invalid("请选择对象")
	}
	sqlText, note, err := shared.ObjectDefinition(ctx, target.DB, target.Driver, schema, search.Name, search.Kind)
	if err != nil {
		if errors.Is(err, shared.ErrObjectNotFound) {
			return zero, apperrors.NotFound("对象不存在")
		}
		return zero, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取定义失败", "", err), err)
	}
	return response.OK(CatalogDefinitionResponse{
		Driver:   target.Driver,
		Schema:   schema,
		Name:     search.Name,
		Kind:     strings.ToLower(strings.TrimSpace(search.Kind)),
		SQL:      sqlText,
		Editable: true,
		Note:     note,
	}), nil
}

func (r *Resource) Preview(ctx context.Context, request *CatalogPreviewRequest) (response.Response[CatalogPreviewResponse], error) {
	var zero response.Response[CatalogPreviewResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("catalog:read"); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	sqlText, err := shared.RenderObject(draftFrom(request))
	if err != nil {
		return zero, apperrors.Invalid(err.Error())
	}
	return response.OK(CatalogPreviewResponse{Driver: shared.NormalizeDriver(request.Driver), SQL: sqlText}), nil
}

func (r *Resource) Apply(ctx context.Context, request *CatalogApplyRequest) (response.Response[CatalogApplyResponse], error) {
	var zero response.Response[CatalogApplyResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("catalog:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择连接")
	}
	sqlText := strings.TrimSpace(request.SQL)
	if sqlText == "" {
		return zero, apperrors.Invalid("SQL 不能为空")
	}
	if r.cfg != nil && len(sqlText) > r.cfg.Query.MaxSQLBytes {
		return zero, apperrors.TooLarge("SQL 超过长度限制")
	}
	target, err := r.opener.Open(ctx, request.ConnectionID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	started := time.Now()
	results, err := shared.Execute(ctx, target.DB, target.Driver, sqlText, shared.ExecOptions{
		ReadOnly:      false,
		MaxRows:       20,
		MaxStatements: 30,
	})
	if err != nil {
		return zero, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("执行对象语句失败", "", err), err)
	}
	rowCount := int64(0)
	for _, item := range results {
		rowCount += item.RowsAffected
	}
	return response.OK(CatalogApplyResponse{
		Driver:     target.Driver,
		Statements: len(results),
		DurationMS: time.Since(started).Milliseconds(),
		RowCount:   rowCount,
	}), nil
}

func draftFrom(request *CatalogPreviewRequest) shared.ObjectDraft {
	columns := make([]shared.ColumnDraft, 0, len(request.Columns))
	for _, column := range request.Columns {
		columns = append(columns, shared.ColumnDraft{
			Name:     column.Name,
			Type:     column.Type,
			Nullable: column.Nullable,
			Primary:  column.Primary,
			Identity: column.Identity,
			Default:  column.Default,
		})
	}
	params := make([]shared.ParamDraft, 0, len(request.Params))
	for _, param := range request.Params {
		params = append(params, shared.ParamDraft{Name: param.Name, Type: param.Type, Mode: param.Mode})
	}
	return shared.ObjectDraft{
		Driver:    request.Driver,
		Schema:    request.Schema,
		Kind:      request.Kind,
		Name:      request.Name,
		Replace:   request.Replace,
		Columns:   columns,
		SelectSQL: request.SelectSQL,
		Params:    params,
		Returns:   request.Returns,
		Language:  request.Language,
		Body:      request.Body,
	}
}

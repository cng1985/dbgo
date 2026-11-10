package catalog

import (
	"context"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Resource struct {
	opener shared.Opener
	cfg    *config.Config
}

func NewResource(opener shared.Opener, cfg *config.Config) *Resource {
	return &Resource{opener: opener, cfg: cfg}
}

func (r *Resource) Schemas(ctx context.Context, search *CatalogSearch) (response.Response[CatalogResponse], error) {
	target, err := r.open(ctx, search)
	if err != nil {
		return response.Response[CatalogResponse]{}, err
	}
	defer target.Close()
	items, err := shared.ListSchemas(ctx, target.DB, target.Driver)
	if err != nil {
		return response.Response[CatalogResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取模式失败", "", err), err)
	}
	schemas := make([]CatalogSchema, 0, len(items))
	for _, item := range items {
		schemas = append(schemas, CatalogSchema{Name: item.Name})
	}
	return response.OK(CatalogResponse{Driver: target.Driver, Schemas: schemas, Tables: []CatalogTable{}, Columns: []CatalogColumn{}, Indexes: []CatalogIndex{}}), nil
}

func (r *Resource) Tables(ctx context.Context, search *CatalogSearch) (response.Response[CatalogResponse], error) {
	target, schema, err := r.openSchema(ctx, search)
	if err != nil {
		return response.Response[CatalogResponse]{}, err
	}
	defer target.Close()
	items, err := shared.ListTables(ctx, target.DB, target.Driver, schema)
	if err != nil {
		return response.Response[CatalogResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取表失败", "", err), err)
	}
	tables := make([]CatalogTable, 0, len(items))
	for _, item := range items {
		tables = append(tables, CatalogTable{Schema: item.Schema, Name: item.Name, Type: item.Type})
	}
	return response.OK(CatalogResponse{Driver: target.Driver, Schemas: []CatalogSchema{}, Tables: tables, Columns: []CatalogColumn{}, Indexes: []CatalogIndex{}}), nil
}

func (r *Resource) Columns(ctx context.Context, search *CatalogSearch) (response.Response[CatalogResponse], error) {
	target, schema, table, err := r.openTable(ctx, search)
	if err != nil {
		return response.Response[CatalogResponse]{}, err
	}
	defer target.Close()
	items, err := shared.ListColumns(ctx, target.DB, target.Driver, schema, table)
	if err != nil {
		return response.Response[CatalogResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取字段失败", "", err), err)
	}
	columns := make([]CatalogColumn, 0, len(items))
	for _, item := range items {
		columns = append(columns, CatalogColumn{Name: item.Name, Type: item.Type, Nullable: item.Nullable, Default: item.Default, Primary: item.Primary, Extra: item.Extra})
	}
	return response.OK(CatalogResponse{Driver: target.Driver, Schemas: []CatalogSchema{}, Tables: []CatalogTable{}, Columns: columns, Indexes: []CatalogIndex{}}), nil
}

func (r *Resource) Suggest(ctx context.Context, search *CatalogSearch) (response.Response[CatalogSuggestResponse], error) {
	target, schema, err := r.openSchema(ctx, search)
	if err != nil {
		return response.Response[CatalogSuggestResponse]{}, err
	}
	defer target.Close()
	schemaItems, err := shared.ListSchemas(ctx, target.DB, target.Driver)
	if err != nil {
		return response.Response[CatalogSuggestResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取模式失败", "", err), err)
	}
	catalog, err := shared.ListSuggest(ctx, target.DB, target.Driver, schema)
	if err != nil {
		return response.Response[CatalogSuggestResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取结构提示失败", "", err), err)
	}
	schemas := make([]CatalogSchema, 0, len(schemaItems))
	for _, item := range schemaItems {
		schemas = append(schemas, CatalogSchema{Name: item.Name})
	}
	tables := make([]CatalogSuggestTable, 0, len(catalog.Tables))
	for _, table := range catalog.Tables {
		columns := make([]CatalogSuggestColumn, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, CatalogSuggestColumn{Name: column.Name, Type: column.Type})
		}
		tables = append(tables, CatalogSuggestTable{
			Schema:  table.Schema,
			Name:    table.Name,
			Type:    table.Type,
			Columns: columns,
		})
	}
	return response.OK(CatalogSuggestResponse{
		Driver:    target.Driver,
		Schema:    schema,
		Schemas:   schemas,
		Tables:    tables,
		Truncated: catalog.Truncated,
	}), nil
}

func (r *Resource) Indexes(ctx context.Context, search *CatalogSearch) (response.Response[CatalogResponse], error) {
	target, schema, table, err := r.openTable(ctx, search)
	if err != nil {
		return response.Response[CatalogResponse]{}, err
	}
	defer target.Close()
	items, err := shared.ListIndexes(ctx, target.DB, target.Driver, schema, table)
	if err != nil {
		return response.Response[CatalogResponse]{}, apperrors.Business(10002, apperrors.KindInvalid, shared.PublicError("读取索引失败", "", err), err)
	}
	indexes := make([]CatalogIndex, 0, len(items))
	for _, item := range items {
		indexes = append(indexes, CatalogIndex{Name: item.Name, Unique: item.Unique, Columns: item.Columns})
	}
	return response.OK(CatalogResponse{Driver: target.Driver, Schemas: []CatalogSchema{}, Tables: []CatalogTable{}, Columns: []CatalogColumn{}, Indexes: indexes}), nil
}

func (r *Resource) open(ctx context.Context, search *CatalogSearch) (*shared.Target, error) {
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := scope.Allow("catalog:read"); err != nil {
		return nil, err
	}
	if search == nil || search.ConnectionID <= 0 {
		return nil, apperrors.Invalid("请选择连接")
	}
	return r.opener.Open(ctx, search.ConnectionID)
}

func (r *Resource) openSchema(ctx context.Context, search *CatalogSearch) (*shared.Target, string, error) {
	target, err := r.open(ctx, search)
	if err != nil {
		return nil, "", err
	}
	schema := search.Schema
	if schema == "" {
		schema = target.Schema
	}
	if _, err := shared.QuoteIdent(target.Driver, schema); err != nil {
		target.Close()
		return nil, "", apperrors.Invalid("模式名无效")
	}
	return target, schema, nil
}

func (r *Resource) openTable(ctx context.Context, search *CatalogSearch) (*shared.Target, string, string, error) {
	target, schema, err := r.openSchema(ctx, search)
	if err != nil {
		return nil, "", "", err
	}
	if search.Table == "" {
		target.Close()
		return nil, "", "", apperrors.Invalid("请选择表")
	}
	if _, err := shared.QuoteIdent(target.Driver, search.Table); err != nil {
		target.Close()
		return nil, "", "", apperrors.Invalid("表名无效")
	}
	return target, schema, search.Table, nil
}

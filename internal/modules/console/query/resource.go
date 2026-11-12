package query

import (
	"context"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Resource struct {
	repo   *Repository
	opener shared.Opener
	cfg    *config.Config
	log    *logging.Logger
}

func NewResource(repo *Repository, opener shared.Opener, cfg *config.Config, log *logging.Logger) *Resource {
	return &Resource{repo: repo, opener: opener, cfg: cfg, log: log}
}

func (r *Resource) Execute(ctx context.Context, request *QueryExecuteRequest) (response.Response[QueryResponse], error) {
	var zero response.Response[QueryResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("query:execute"); err != nil {
		return zero, err
	}
	if request == nil || request.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择连接")
	}
	// 保留原文里的空行，只把回车统一成换行。行号按编辑器里的文本计算。
	sqlText := shared.NormalizeNewlines(request.SQL)
	if strings.TrimSpace(sqlText) == "" {
		return zero, apperrors.Invalid("SQL 不能为空")
	}
	schema := strings.TrimSpace(request.Schema)
	if _, err := shared.SchemaSwitchSQL("mysql", schema); err != nil {
		return zero, apperrors.Invalid("数据库名无效")
	}
	if len(sqlText) > r.cfg.Query.MaxSQLBytes {
		return zero, apperrors.TooLarge("SQL 超过长度限制")
	}
	readOnly := true
	if request.ReadOnly != nil {
		readOnly = *request.ReadOnly
	}
	maxRows := request.MaxRows
	if maxRows <= 0 {
		maxRows = 100
	}
	if maxRows > r.cfg.Query.MaxRows {
		return zero, apperrors.Invalid("返回行数超过限制")
	}
	timeout := 30 * time.Second
	if request.TimeoutSeconds > 0 {
		timeout = time.Duration(request.TimeoutSeconds) * time.Second
	}
	if timeout > r.cfg.Server.LongRequestTimeout {
		timeout = r.cfg.Server.LongRequestTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target, err := r.opener.Open(execCtx, request.ConnectionID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	started := time.Now()
	results, execErr := shared.Execute(execCtx, target.DB, target.Driver, sqlText, shared.ExecOptions{
		ReadOnly:      readOnly,
		MaxRows:       maxRows,
		MaxStatements: 50,
		Schema:        schema,
	})
	duration := time.Since(started).Milliseconds()
	status := "success"
	message := ""
	rowCount := int64(0)
	for _, item := range results {
		rowCount += item.RowsAffected
	}
	if execErr != nil {
		status = "failed"
		message = shared.PublicSQLError(execErr)
		if len(message) > 2000 {
			message = message[:2000]
		}
	}
	history := &QueryHistory{
		ConnectionID:   request.ConnectionID,
		ConnectionName: target.Name,
		ActorID:        scope.ActorID,
		SQLText:        sqlText,
		SchemaName:     schema,
		Status:         status,
		RowCount:       rowCount,
		DurationMS:     duration,
		ErrorMessage:   message,
		ReadOnly:       readOnly,
		CreatedAt:      time.Now().UTC(),
	}
	historyID, historyErr := r.repo.insert(ctx, history)
	if historyErr != nil {
		r.log.Error("查询历史写入失败", "error", historyErr.Error(), "connection_id", request.ConnectionID)
	}
	if execErr != nil {
		return zero, apperrors.Business(10002, apperrors.KindInvalid, message, execErr)
	}
	statements := make([]QueryStatement, 0, len(results))
	for _, item := range results {
		columns := make([]QueryColumn, 0, len(item.Columns))
		for _, column := range item.Columns {
			columns = append(columns, QueryColumn{Name: column.Name, Type: column.Type})
		}
		rows := item.Rows
		if rows == nil {
			rows = [][]*string{}
		}
		statements = append(statements, QueryStatement{
			Command:      item.Command,
			Columns:      columns,
			Rows:         rows,
			RowsAffected: item.RowsAffected,
			Truncated:    item.Truncated,
			Messages:     item.Messages,
			SQL:          item.SQL,
		})
	}
	return response.OK(QueryResponse{
		ID:             historyID,
		ConnectionID:   request.ConnectionID,
		ConnectionName: target.Name,
		SQL:            sqlText,
		Schema:         schema,
		Status:         status,
		ReadOnly:       readOnly,
		DurationMS:     duration,
		RowCount:       rowCount,
		Statements:     statements,
		CreatedAt:      history.CreatedAt,
	}), nil
}

func (r *Resource) One(ctx context.Context, request *QueryOneRequest) (response.Response[QueryResponse], error) {
	var zero response.Response[QueryResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("query:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	request.ActorID = scope.ActorID
	return r.repo.One(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *QuerySearch) (response.PageResponse[QuerySimple], error) {
	var zero response.PageResponse[QuerySimple]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("query:read"); err != nil {
		return zero, err
	}
	if search == nil {
		search = &QuerySearch{}
	}
	if search.Status != "" && search.Status != "success" && search.Status != "failed" {
		return zero, apperrors.Invalid("查询状态不受支持")
	}
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	search.ActorID = scope.ActorID
	return r.repo.Search(ctx, search)
}

package etl

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type ConnectionReader interface {
	Describe(ctx context.Context, id int64) (string, string, error)
}

type Resource struct {
	repo    *Repository
	opener  shared.Opener
	names   ConnectionReader
	log     *logging.Logger
	mu      sync.Mutex
	running map[int64]context.CancelFunc
}

func NewResource(repo *Repository, opener shared.Opener, names ConnectionReader, log *logging.Logger) *Resource {
	return &Resource{repo: repo, opener: opener, names: names, log: log, running: map[int64]context.CancelFunc{}}
}

func (r *Resource) Create(ctx context.Context, request *TaskRequest) (response.Response[TaskResponse], error) {
	var zero response.Response[TaskResponse]
	scope, err := r.authorize(ctx, "etl:write")
	if err != nil {
		return zero, err
	}
	if err := response.RejectClientID(requestID(request)); err != nil {
		return zero, err
	}
	normalized, err := r.prepare(ctx, request)
	if err != nil {
		return zero, err
	}
	total, err := r.repo.Count(ctx, scope.ActorID)
	if err != nil {
		return zero, err
	}
	if total >= maxTasks {
		return zero, apperrors.Invalid("任务已有 200 个，请先清理后再新建")
	}
	now := time.Now().UTC()
	entity := &Task{
		Name:                 normalized.name,
		SourceConnectionID:   request.SourceConnectionID,
		SourceConnectionName: normalized.sourceName,
		SourceSQL:            normalized.sql,
		TargetConnectionID:   request.TargetConnectionID,
		TargetConnectionName: normalized.targetName,
		TargetSchema:         normalized.schema,
		TargetTable:          normalized.table,
		WriteMode:            normalized.mode,
		Mappings:             normalized.mappings,
		Version:              1,
		CreatedBy:            scope.ActorID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	return r.repo.Create(ctx, entity)
}

func (r *Resource) Update(ctx context.Context, request *TaskRequest) (response.Response[TaskResponse], error) {
	var zero response.Response[TaskResponse]
	scope, err := r.authorize(ctx, "etl:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 || request.Version <= 0 {
		return zero, apperrors.Invalid("修改任务需要有效主键和版本")
	}
	if r.isRunning(request.ID) {
		return zero, apperrors.Conflict("任务正在执行")
	}
	if _, err := r.repo.One(ctx, scope.ActorID, request.ID); err != nil {
		return zero, err
	}
	normalized, err := r.prepare(ctx, request)
	if err != nil {
		return zero, err
	}
	fields := map[string]any{
		"name":                   normalized.name,
		"source_connection_id":   request.SourceConnectionID,
		"source_connection_name": normalized.sourceName,
		"source_sql":             normalized.sql,
		"target_connection_id":   request.TargetConnectionID,
		"target_connection_name": normalized.targetName,
		"target_schema":          normalized.schema,
		"target_table":           normalized.table,
		"write_mode":             normalized.mode,
		"mappings":               normalized.mappings,
	}
	return r.repo.Update(ctx, scope.ActorID, request, fields)
}

func (r *Resource) One(ctx context.Context, request *TaskActionRequest) (response.Response[TaskResponse], error) {
	var zero response.Response[TaskResponse]
	scope, err := r.authorize(ctx, "etl:read")
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
	mappings, err := decodeMappings(entity.Mappings)
	if err != nil {
		return zero, apperrors.Invalid("字段对应已损坏")
	}
	return response.OK(toResponse(entity, mappings)), nil
}

func (r *Resource) Search(ctx context.Context, search *TaskSearch) (response.PageResponse[TaskSimple], error) {
	var zero response.PageResponse[TaskSimple]
	scope, err := r.authorize(ctx, "etl:read")
	if err != nil {
		return zero, err
	}
	if search == nil {
		search = &TaskSearch{}
	}
	return r.repo.Search(ctx, scope.ActorID, search)
}

func (r *Resource) Delete(ctx context.Context, request *TaskActionRequest) (response.Response[TaskResponse], error) {
	var zero response.Response[TaskResponse]
	scope, err := r.authorize(ctx, "etl:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	if r.isRunning(request.ID) {
		return zero, apperrors.Conflict("任务正在执行")
	}
	return r.repo.Delete(ctx, scope.ActorID, request.ID)
}

func (r *Resource) Preview(ctx context.Context, request *PreviewRequest) (response.Response[PreviewResponse], error) {
	var zero response.Response[PreviewResponse]
	if _, err := r.authorize(ctx, "etl:read"); err != nil {
		return zero, err
	}
	if request == nil || request.SourceConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择来源连接")
	}
	statement, err := ValidateExtract(request.SourceSQL)
	if err != nil {
		return zero, err
	}
	source, err := r.opener.Open(ctx, request.SourceConnectionID)
	if err != nil {
		return zero, err
	}
	defer source.Close()
	outcome, err := Preview(ctx, source.DB, statement, previewRows)
	if err != nil {
		return zero, err
	}
	if outcome.Columns == nil {
		outcome.Columns = []string{}
	}
	if outcome.Rows == nil {
		outcome.Rows = [][]*string{}
	}
	return response.OK(PreviewResponse{Columns: outcome.Columns, Rows: outcome.Rows, Truncated: outcome.Truncated}), nil
}

func (r *Resource) Run(ctx context.Context, request *TaskActionRequest) (response.Response[RunResponse], error) {
	var zero response.Response[RunResponse]
	scope, err := r.authorize(ctx, "etl:run")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	task, err := r.repo.One(ctx, scope.ActorID, request.ID)
	if err != nil {
		return zero, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	if !r.track(task.ID, cancel) {
		cancel()
		return zero, apperrors.Conflict("任务正在执行")
	}
	defer func() {
		r.untrack(task.ID)
		cancel()
	}()
	if _, err := r.repo.FailStale(runCtx, scope.ActorID, task.ID, "上次执行中断"); err != nil {
		return zero, err
	}
	mappings, err := decodeMappings(task.Mappings)
	if err != nil {
		return zero, apperrors.Invalid("字段对应已损坏")
	}
	statement, err := ValidateExtract(task.SourceSQL)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC()
	run := &Run{
		TaskID:    task.ID,
		Status:    "running",
		StartedAt: now,
		CreatedBy: scope.ActorID,
	}
	if err := r.repo.CreateRun(runCtx, run); err != nil {
		return zero, err
	}
	outcome, runErr := r.execute(runCtx, task, statement, mappings)
	finished := time.Now().UTC()
	status := "succeeded"
	message := ""
	if runErr != nil {
		status = "failed"
		message = failureText(runCtx, runErr)
		r.log.Error("etl 执行失败", "error", message, "task_id", task.ID)
	}
	stored := &Run{
		ID:           run.ID,
		TaskID:       task.ID,
		Status:       status,
		RowsRead:     outcome.RowsRead,
		RowsWritten:  outcome.RowsWritten,
		ErrorMessage: message,
		StartedAt:    run.StartedAt,
		FinishedAt:   &finished,
		CreatedBy:    scope.ActorID,
	}
	if err := r.finish(ctx, scope.ActorID, stored); err != nil {
		r.log.Error("etl 执行记录更新失败", "error", err.Error(), "run_id", run.ID)
		if runErr != nil {
			return zero, runErr
		}
		return zero, apperrors.Business(codeETL, apperrors.KindUnavailable, "数据已经写入，但执行记录没有更新", err)
	}
	if runErr != nil {
		return zero, runErr
	}
	return response.OK(toRun(stored)), nil
}

func (r *Resource) Abort(ctx context.Context, request *TaskActionRequest) (response.Response[ActionResponse], error) {
	var zero response.Response[ActionResponse]
	scope, err := r.authorize(ctx, "etl:run")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	if _, err := r.repo.One(ctx, scope.ActorID, request.ID); err != nil {
		return zero, err
	}
	if cancel, ok := r.cancel(request.ID); ok {
		cancel()
		return response.OK(ActionResponse{Status: "aborting"}), nil
	}
	affected, err := r.repo.FailStale(ctx, scope.ActorID, request.ID, "执行已中断")
	if err != nil {
		return zero, err
	}
	if affected == 0 {
		return zero, apperrors.Invalid("这个任务没有正在执行")
	}
	return response.OK(ActionResponse{Status: "failed"}), nil
}

func (r *Resource) Runs(ctx context.Context, search *RunSearch) (response.PageResponse[RunResponse], error) {
	var zero response.PageResponse[RunResponse]
	scope, err := r.authorize(ctx, "etl:read")
	if err != nil {
		return zero, err
	}
	if search == nil || search.TaskID <= 0 {
		return zero, apperrors.Invalid("请选择任务")
	}
	if _, err := r.repo.One(ctx, scope.ActorID, search.TaskID); err != nil {
		return zero, err
	}
	return r.repo.SearchRuns(ctx, scope.ActorID, search.TaskID, search.Page, search.PageSize)
}

func (r *Resource) execute(ctx context.Context, task *Task, statement string, mappings []ColumnMap) (Outcome, error) {
	var outcome Outcome
	source, err := r.opener.Open(ctx, task.SourceConnectionID)
	if err != nil {
		return outcome, err
	}
	defer source.Close()
	target, err := r.opener.Open(ctx, task.TargetConnectionID)
	if err != nil {
		return outcome, err
	}
	defer target.Close()
	if err := checkTable(target.Driver, task.TargetSchema, task.TargetTable); err != nil {
		return outcome, err
	}
	return Transfer(ctx, source.DB, target.DB, Spec{
		SourceSQL: statement,
		Driver:    target.Driver,
		Schema:    task.TargetSchema,
		Table:     task.TargetTable,
		Mode:      task.WriteMode,
		Mappings:  mappings,
		MaxRows:   maxExtractRows,
		BatchSize: insertBatch,
	})
}

type prepared struct {
	name       string
	sql        string
	mode       string
	schema     string
	table      string
	mappings   string
	sourceName string
	targetName string
}

func (r *Resource) prepare(ctx context.Context, request *TaskRequest) (prepared, error) {
	var zero prepared
	if request == nil {
		return zero, apperrors.Invalid("请填写任务")
	}
	name, err := cleanName(request.Name)
	if err != nil {
		return zero, err
	}
	statement, err := ValidateExtract(request.SourceSQL)
	if err != nil {
		return zero, err
	}
	mode, err := cleanMode(request.WriteMode)
	if err != nil {
		return zero, err
	}
	if request.SourceConnectionID <= 0 || request.TargetConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择来源和目标连接")
	}
	_, rawMappings, err := normalizeMappings(request.Mappings)
	if err != nil {
		return zero, err
	}
	sourceName, _, err := r.names.Describe(ctx, request.SourceConnectionID)
	if err != nil {
		return zero, err
	}
	targetName, targetDriver, err := r.names.Describe(ctx, request.TargetConnectionID)
	if err != nil {
		return zero, err
	}
	schema := strings.TrimSpace(request.TargetSchema)
	table := strings.TrimSpace(request.TargetTable)
	if err := checkTable(targetDriver, schema, table); err != nil {
		return zero, err
	}
	return prepared{
		name:       name,
		sql:        statement,
		mode:       mode,
		schema:     schema,
		table:      table,
		mappings:   rawMappings,
		sourceName: sourceName,
		targetName: targetName,
	}, nil
}

func (r *Resource) finish(parent context.Context, ownerID int64, run *Run) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	return r.repo.FinishRun(ctx, ownerID, run.ID, run.Status, run.RowsRead, run.RowsWritten, run.ErrorMessage, *run.FinishedAt)
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

func (r *Resource) track(id int64, cancel context.CancelFunc) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.running[id]; ok {
		return false
	}
	r.running[id] = cancel
	return true
}

func (r *Resource) untrack(id int64) {
	r.mu.Lock()
	delete(r.running, id)
	r.mu.Unlock()
}

func (r *Resource) isRunning(id int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.running[id]
	return ok
}

func (r *Resource) cancel(id int64) (context.CancelFunc, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancel, ok := r.running[id]
	return cancel, ok
}

func requestID(request *TaskRequest) int64 {
	if request == nil {
		return 0
	}
	return request.ID
}

func failureText(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "已中止"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "执行超时"
	}
	if app, ok := apperrors.As(err); ok && app.Message != "" {
		return app.Message
	}
	return "执行失败"
}

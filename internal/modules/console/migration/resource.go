package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type ConnectionReader interface {
	Describe(ctx context.Context, id int64) (string, string, error)
}

type Resource struct {
	repo   *Repository
	opener shared.Opener
	names  ConnectionReader
	cfg    *config.Config
}

func NewResource(repo *Repository, opener shared.Opener, names ConnectionReader, cfg *config.Config) *Resource {
	return &Resource{repo: repo, opener: opener, names: names, cfg: cfg}
}

func (r *Resource) Create(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	scope, err := r.authorize(ctx, "migration:write")
	if err != nil {
		return zero, err
	}
	if err := response.RejectClientID(idOf(request)); err != nil {
		return zero, err
	}
	normalized, err := r.normalize(request)
	if err != nil {
		return zero, err
	}
	name, _, err := r.names.Describe(ctx, request.ConnectionID)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC()
	entity := &DataMigration{
		ConnectionID:   request.ConnectionID,
		ConnectionName: name,
		VersionNo:      normalized.VersionNo,
		Name:           normalized.Name,
		UpSQL:          normalized.UpSQL,
		DownSQL:        normalized.DownSQL,
		Checksum:       checksum(normalized.UpSQL, normalized.DownSQL),
		Status:         "draft",
		CreatedBy:      scope.ActorID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return r.repo.Create(ctx, entity)
}

func (r *Resource) Update(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	current, err := r.repo.find(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, current.ConnectionID); err != nil {
		return zero, err
	}
	request.ConnectionID = current.ConnectionID
	request.VersionNo = current.VersionNo
	normalized, err := r.normalize(request)
	if err != nil {
		return zero, err
	}
	current.Name = normalized.Name
	current.UpSQL = normalized.UpSQL
	current.DownSQL = normalized.DownSQL
	current.Checksum = checksum(normalized.UpSQL, normalized.DownSQL)
	current.UpdatedAt = time.Now().UTC()
	return r.repo.Update(ctx, current)
}

func (r *Resource) One(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	result, err := r.repo.One(ctx, request)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, result.Data.ConnectionID); err != nil {
		return zero, err
	}
	return result, nil
}

func (r *Resource) Delete(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	current, err := r.repo.find(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, current.ConnectionID); err != nil {
		return zero, err
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *MigrationSearch) (response.PageResponse[MigrationSimple], error) {
	var zero response.PageResponse[MigrationSimple]
	scope, err := r.authorize(ctx, "migration:read")
	if err != nil {
		return zero, err
	}
	if search == nil {
		search = &MigrationSearch{}
	}
	if search.Status != "" && !validStatus(search.Status) {
		return zero, apperrors.Invalid("迁移状态不受支持")
	}
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	search.OwnerID = scope.ActorID
	return r.repo.Search(ctx, search)
}

func (r *Resource) Apply(ctx context.Context, request *MigrationActionRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:write"); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	if request.ID > 0 {
		return r.applyOne(ctx, request.ID)
	}
	if request.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请指定迁移或连接")
	}
	if err := r.requireConnection(ctx, request.ConnectionID); err != nil {
		return zero, err
	}
	items, err := r.repo.listStatus(ctx, request.ConnectionID, "draft")
	if err != nil {
		return zero, err
	}
	if len(items) == 0 {
		return zero, apperrors.NotFound("没有待执行的迁移")
	}
	var last response.Response[MigrationResponse]
	for _, item := range items {
		last, err = r.applyOne(ctx, item.ID)
		if err != nil {
			return zero, err
		}
	}
	return last, nil
}

func (r *Resource) Rollback(ctx context.Context, request *MigrationActionRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("请指定要回滚的迁移")
	}
	current, err := r.repo.find(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, current.ConnectionID); err != nil {
		return zero, err
	}
	if strings.TrimSpace(current.DownSQL) == "" {
		return zero, apperrors.Invalid("没有回滚脚本")
	}
	claimed, err := r.repo.claimLatestRollback(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	execErr := r.runSQL(ctx, claimed.ConnectionID, claimed.DownSQL)
	if execErr != nil {
		message := shared.PublicError("回滚失败", "", execErr)
		_, _ = r.repo.finish(ctx, claimed.ID, "reverting", "applied", claimed.AppliedAt, trim(message))
		return zero, apperrors.Business(10003, apperrors.KindInvalid, message, execErr)
	}
	entity, err := r.repo.finish(ctx, claimed.ID, "reverting", "draft", nil, "")
	if err != nil {
		return zero, apperrors.Business(10003, apperrors.KindUnavailable, "回滚语句已执行，但迁移状态未能改回草稿", err)
	}
	return response.OK(*MigrationEntityToResponse(entity)), nil
}

func (r *Resource) Resolve(ctx context.Context, request *MigrationActionRequest) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	if _, err := r.authorize(ctx, "migration:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	current, err := r.repo.find(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, current.ConnectionID); err != nil {
		return zero, err
	}
	switch request.Action {
	case "abort":
		switch current.Status {
		case "applying":
			entity, err := r.repo.finish(ctx, current.ID, "applying", "failed", nil, "执行中断后已中止")
			if err != nil {
				return zero, err
			}
			return response.OK(*MigrationEntityToResponse(entity)), nil
		case "reverting":
			entity, err := r.repo.finish(ctx, current.ID, "reverting", "applied", current.AppliedAt, "回滚中断后已恢复为已执行")
			if err != nil {
				return zero, err
			}
			return response.OK(*MigrationEntityToResponse(entity)), nil
		default:
			return zero, apperrors.Conflict("当前状态不能中止")
		}
	case "confirm":
		switch current.Status {
		case "applying":
			now := time.Now().UTC()
			entity, err := r.repo.finish(ctx, current.ID, "applying", "applied", &now, "")
			if err != nil {
				return zero, err
			}
			return response.OK(*MigrationEntityToResponse(entity)), nil
		case "reverting":
			entity, err := r.repo.finish(ctx, current.ID, "reverting", "draft", nil, "")
			if err != nil {
				return zero, err
			}
			return response.OK(*MigrationEntityToResponse(entity)), nil
		default:
			return zero, apperrors.Conflict("当前状态不能确认")
		}
	default:
		return zero, apperrors.Invalid("动作只支持 abort 或 confirm")
	}
}

func (r *Resource) applyOne(ctx context.Context, id int64) (response.Response[MigrationResponse], error) {
	var zero response.Response[MigrationResponse]
	current, err := r.repo.find(ctx, id)
	if err != nil {
		return zero, err
	}
	if err := r.requireMigrationConnection(ctx, current.ConnectionID); err != nil {
		return zero, err
	}
	claimed, err := r.repo.claim(ctx, id, "draft", "applying")
	if err != nil {
		return zero, err
	}
	execErr := r.runSQL(ctx, claimed.ConnectionID, claimed.UpSQL)
	if execErr != nil {
		message := shared.PublicError("迁移执行失败", "", execErr)
		_, _ = r.repo.finish(ctx, claimed.ID, "applying", "failed", nil, trim(message))
		return zero, apperrors.Business(10003, apperrors.KindInvalid, message, execErr)
	}
	now := time.Now().UTC()
	entity, err := r.repo.finish(ctx, claimed.ID, "applying", "applied", &now, "")
	if err != nil {
		return zero, apperrors.Business(10003, apperrors.KindUnavailable, "目标库已执行，但迁移状态未能标记为已完成，请核对后确认", err)
	}
	return response.OK(*MigrationEntityToResponse(entity)), nil
}

func (r *Resource) runSQL(ctx context.Context, connectionID int64, script string) error {
	target, err := r.opener.Open(ctx, connectionID)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = shared.Execute(ctx, target.DB, target.Driver, script, shared.ExecOptions{
		ReadOnly:       false,
		MaxRows:        1,
		MaxStatements:  500,
		UseTransaction: true,
	})
	return err
}

func (r *Resource) normalize(request *MigrationRequest) (*MigrationRequest, error) {
	if request == nil || request.ConnectionID <= 0 {
		return nil, apperrors.Invalid("请选择连接")
	}
	if request.VersionNo <= 0 {
		return nil, apperrors.Invalid("迁移版本必须是正整数")
	}
	name, err := response.TrimRequired(request.Name, "迁移名称", 120)
	if err != nil {
		return nil, err
	}
	up := strings.TrimSpace(request.UpSQL)
	down := strings.TrimSpace(request.DownSQL)
	if up == "" {
		return nil, apperrors.Invalid("升级脚本不能为空")
	}
	if len(up) > r.cfg.Query.MaxSQLBytes || len(down) > r.cfg.Query.MaxSQLBytes {
		return nil, apperrors.TooLarge("迁移脚本超过长度限制")
	}
	return &MigrationRequest{ConnectionID: request.ConnectionID, VersionNo: request.VersionNo, Name: name, UpSQL: up, DownSQL: down}, nil
}

func (r *Resource) requireConnection(ctx context.Context, connectionID int64) error {
	_, _, err := r.names.Describe(ctx, connectionID)
	return err
}

func (r *Resource) requireMigrationConnection(ctx context.Context, connectionID int64) error {
	if err := r.requireConnection(ctx, connectionID); err != nil {
		if appErr, ok := apperrors.As(err); ok && appErr.Kind == apperrors.KindNotFound {
			return apperrors.NotFound("迁移不存在")
		}
		return err
	}
	return nil
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

func idOf(request *MigrationRequest) int64 {
	if request == nil {
		return 0
	}
	return request.ID
}

func checksum(up, down string) string {
	sum := sha256.Sum256([]byte(up + "\n---\n" + down))
	return hex.EncodeToString(sum[:])
}

func trim(message string) string {
	if len(message) > 2000 {
		return message[:2000]
	}
	return message
}

func validStatus(status string) bool {
	switch status {
	case "draft", "applying", "applied", "failed", "reverting":
		return true
	default:
		return false
	}
}

package connection

import (
	"context"
	"time"

	"github.com/cng1985/dbgo/internal/modules/console/shared"
	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
	"github.com/cng1985/dbgo/internal/platform/secret"
)

type Resource struct {
	repo   *Repository
	box    *secret.Box
	cfg    *config.Config
	opener *TargetOpener
}

func NewResource(repo *Repository, box *secret.Box, cfg *config.Config, opener *TargetOpener) *Resource {
	return &Resource{repo: repo, box: box, cfg: cfg, opener: opener}
}

func (r *Resource) Create(ctx context.Context, request *ConnectionRequest) (response.Response[ConnectionResponse], error) {
	var zero response.Response[ConnectionResponse]
	scope, err := r.authorize(ctx, "connection:write")
	if err != nil {
		return zero, err
	}
	if err := response.RejectClientID(requestID(request)); err != nil {
		return zero, err
	}
	normalized, err := normalizeConnection(r.cfg, request, false)
	if err != nil {
		return zero, err
	}
	cipher, err := r.box.Encrypt(normalized.Password)
	if err != nil {
		return zero, apperrors.Internal(err)
	}
	now := time.Now().UTC()
	entity := &Connection{
		Name:           normalized.Name,
		Driver:         normalized.Driver,
		Host:           normalized.Host,
		Port:           normalized.Port,
		DatabaseName:   normalized.DatabaseName,
		Username:       normalized.Username,
		PasswordCipher: cipher,
		SQLitePath:     normalized.SQLitePath,
		SSLMode:        normalized.SSLMode,
		Remark:         normalized.Remark,
		Version:        1,
		CreatedBy:      scope.ActorID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return r.repo.Create(ctx, entity)
}

func (r *Resource) Update(ctx context.Context, request *ConnectionRequest) (response.Response[ConnectionResponse], error) {
	var zero response.Response[ConnectionResponse]
	if _, err := r.authorize(ctx, "connection:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 || request.Version <= 0 {
		return zero, apperrors.Invalid("修改连接需要有效主键和版本")
	}
	current, err := r.repo.find(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	keepPassword := request.Password == ""
	normalized, err := normalizeConnection(r.cfg, request, keepPassword)
	if err != nil {
		return zero, err
	}
	cipher := current.PasswordCipher
	if !keepPassword {
		cipher, err = r.box.Encrypt(normalized.Password)
		if err != nil {
			return zero, apperrors.Internal(err)
		}
	}
	fields := map[string]any{
		"name":            normalized.Name,
		"driver":          normalized.Driver,
		"host":            normalized.Host,
		"port":            normalized.Port,
		"database_name":   normalized.DatabaseName,
		"username":        normalized.Username,
		"password_cipher": cipher,
		"sqlite_path":     normalized.SQLitePath,
		"ssl_mode":        normalized.SSLMode,
		"remark":          normalized.Remark,
	}
	return r.repo.Update(ctx, request, fields)
}

func (r *Resource) One(ctx context.Context, request *ConnectionRequest) (response.Response[ConnectionResponse], error) {
	var zero response.Response[ConnectionResponse]
	if _, err := r.authorize(ctx, "connection:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	return r.repo.One(ctx, request)
}

func (r *Resource) Delete(ctx context.Context, request *ConnectionDeleteRequest) (response.Response[ConnectionResponse], error) {
	var zero response.Response[ConnectionResponse]
	if _, err := r.authorize(ctx, "connection:write"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *ConnectionSearch) (response.PageResponse[ConnectionSimple], error) {
	var zero response.PageResponse[ConnectionSimple]
	if _, err := r.authorize(ctx, "connection:read"); err != nil {
		return zero, err
	}
	if search == nil {
		search = &ConnectionSearch{}
	}
	if search.Driver != "" {
		search.Driver = shared.NormalizeDriver(search.Driver)
		if search.Driver == "" {
			return zero, apperrors.Invalid("数据库类型不受支持")
		}
	}
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return zero, err
	}
	search.Page = page
	search.PageSize = pageSize
	return r.repo.Search(ctx, search)
}

func (r *Resource) Describe(ctx context.Context, id int64) (string, string, error) {
	result, err := r.One(ctx, &ConnectionRequest{ID: id})
	if err != nil {
		return "", "", err
	}
	return result.Data.Name, result.Data.Driver, nil
}

func (r *Resource) Test(ctx context.Context, request *ConnectionRequest) (response.Response[ConnectionResponse], error) {
	var zero response.Response[ConnectionResponse]
	if _, err := r.authorize(ctx, "connection:read"); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	started := time.Now()
	target, err := r.opener.Open(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	version, err := shared.ServerVersion(ctx, target.DB, target.Driver)
	if err != nil {
		return zero, apperrors.Business(10001, apperrors.KindUnavailable, "读取数据库版本失败", err)
	}
	result, err := r.repo.One(ctx, request)
	if err != nil {
		return zero, err
	}
	reachable := true
	result.Data.Reachable = &reachable
	result.Data.ServerVersion = version
	result.Data.LatencyMS = time.Since(started).Milliseconds()
	return result, nil
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

func requestID(request *ConnectionRequest) int64 {
	if request == nil {
		return 0
	}
	return request.ID
}

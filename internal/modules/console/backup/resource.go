package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func (r *Resource) Create(ctx context.Context, request *BackupRequest) (response.Response[BackupResponse], error) {
	var zero response.Response[BackupResponse]
	scope, err := r.authorize(ctx, "backup:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ConnectionID <= 0 {
		return zero, apperrors.Invalid("请选择连接")
	}
	name, err := response.TrimRequired(request.Name, "备份名称", 120)
	if err != nil {
		return zero, err
	}
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		format = "sql"
	}
	if format != "sql" && format != "sqlite" {
		return zero, apperrors.Invalid("备份格式只支持 sql 或 sqlite")
	}
	connectionName, driver, err := r.names.Describe(ctx, request.ConnectionID)
	if err != nil {
		return zero, err
	}
	if format == "sqlite" && driver != "sqlite" {
		return zero, apperrors.Invalid("只有 sqlite 连接可以创建二进制副本")
	}
	created, err := r.repo.Create(ctx, &DataBackup{
		ConnectionID:   request.ConnectionID,
		ConnectionName: connectionName,
		Driver:         driver,
		Name:           name,
		Format:         format,
		Status:         "running",
		CreatedBy:      scope.ActorID,
	})
	if err != nil {
		return zero, err
	}
	extension := "sql"
	if format == "sqlite" {
		extension = "db"
	}
	key := fmt.Sprintf("backups/%d.%s", created.Data.ID, extension)
	path, err := r.safePath(key)
	if err != nil {
		_, _ = r.fail(ctx, created.Data.ID, "备份路径无效")
		return zero, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		_, _ = r.fail(ctx, created.Data.ID, "无法创建备份目录")
		return zero, apperrors.Internal(err)
	}
	stats, dumpErr := r.writeBackup(ctx, request.ConnectionID, driver, connectionName, format, path)
	if dumpErr != nil {
		_ = os.Remove(path)
		message := shared.PublicError("备份失败", "", dumpErr)
		_, _ = r.fail(ctx, created.Data.ID, trim(message))
		return zero, apperrors.Business(10004, apperrors.KindInvalid, message, dumpErr)
	}
	info, err := os.Stat(path)
	if err != nil {
		return zero, apperrors.Internal(err)
	}
	sum, err := fileChecksum(path)
	if err != nil {
		return zero, apperrors.Internal(err)
	}
	return r.repo.Finish(ctx, &BackupRequest{ID: created.Data.ID}, map[string]any{
		"storage_key":     key,
		"size_bytes":      info.Size(),
		"checksum":        sum,
		"status":          "ready",
		"truncated":       stats.Truncated,
		"statement_count": stats.Statements,
		"error_message":   "",
	})
}

func (r *Resource) writeBackup(ctx context.Context, connectionID int64, driver, name, format, path string) (shared.DumpStats, error) {
	target, err := r.opener.Open(ctx, connectionID)
	if err != nil {
		return shared.DumpStats{}, err
	}
	defer target.Close()
	if format == "sqlite" {
		escaped := strings.ReplaceAll(path, "'", "''")
		if strings.Contains(path, "'") {
			return shared.DumpStats{}, fmt.Errorf("备份路径不能包含引号")
		}
		if _, err := target.DB.ExecContext(ctx, "vacuum into '"+escaped+"'"); err != nil {
			return shared.DumpStats{}, err
		}
		return shared.DumpStats{Tables: 1, Statements: 1}, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return shared.DumpStats{}, err
	}
	defer file.Close()
	writer := &limitedWriter{writer: file, max: r.cfg.Backup.MaxBytes}
	stats, err := shared.DumpSQL(ctx, target.DB, driver, target.Schema, name, r.cfg.Backup.MaxRows, writer)
	if err != nil {
		return stats, err
	}
	if err := file.Sync(); err != nil {
		return stats, err
	}
	return stats, nil
}

func (r *Resource) One(ctx context.Context, request *BackupRequest) (response.Response[BackupResponse], error) {
	var zero response.Response[BackupResponse]
	scope, err := r.authorize(ctx, "backup:read")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	return r.repo.One(ctx, scope.ActorID, request)
}

func (r *Resource) Search(ctx context.Context, search *BackupSearch) (response.PageResponse[BackupSimple], error) {
	var zero response.PageResponse[BackupSimple]
	scope, err := r.authorize(ctx, "backup:read")
	if err != nil {
		return zero, err
	}
	if search == nil {
		search = &BackupSearch{}
	}
	if search.Status != "" && search.Status != "ready" && search.Status != "failed" && search.Status != "running" {
		return zero, apperrors.Invalid("备份状态不受支持")
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

func (r *Resource) Delete(ctx context.Context, request *BackupRequest) (response.Response[BackupResponse], error) {
	var zero response.Response[BackupResponse]
	scope, err := r.authorize(ctx, "backup:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	current, err := r.repo.find(ctx, scope.ActorID, request.ID)
	if err != nil {
		return zero, err
	}
	result, err := r.repo.Delete(ctx, scope.ActorID, request)
	if err != nil {
		return zero, err
	}
	if current.StorageKey != "" {
		if path, pathErr := r.safePath(current.StorageKey); pathErr == nil {
			_ = os.Remove(path)
		}
	}
	return result, nil
}

func (r *Resource) Restore(ctx context.Context, request *BackupRequest) (response.Response[BackupResponse], error) {
	var zero response.Response[BackupResponse]
	scope, err := r.authorize(ctx, "backup:write")
	if err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperrors.Invalid("主键无效")
	}
	current, err := r.repo.find(ctx, scope.ActorID, request.ID)
	if err != nil {
		return zero, err
	}
	if current.Status != "ready" || current.Format != "sql" {
		return zero, apperrors.Conflict("只有已完成的 SQL 备份可以恢复")
	}
	path, err := r.safePath(current.StorageKey)
	if err != nil {
		return zero, err
	}
	script, err := os.ReadFile(path)
	if err != nil {
		return zero, apperrors.NotFound("备份文件不存在")
	}
	if int64(len(script)) > r.cfg.Backup.MaxBytes {
		return zero, apperrors.TooLarge("备份文件超过限制")
	}
	target, err := r.opener.Open(ctx, current.ConnectionID)
	if err != nil {
		return zero, err
	}
	defer target.Close()
	results, err := shared.Execute(ctx, target.DB, target.Driver, string(script), shared.ExecOptions{
		ReadOnly:       false,
		MaxRows:        1,
		MaxStatements:  100000,
		UseTransaction: target.Driver != "mysql",
	})
	if err != nil {
		return zero, apperrors.Business(10004, apperrors.KindInvalid, shared.PublicError("恢复失败", "", err), err)
	}
	result := BackupEntityToResponse(current)
	result.Detail = fmt.Sprintf("已执行 %d 条语句", len(results))
	result.StatementCount = int64(len(results))
	return response.OK(*result), nil
}

func (r *Resource) Download(ctx context.Context, request *BackupRequest) (response.Response[BackupResponse], error) {
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	result, err := r.One(ctx, request)
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	if result.Data.Status != "ready" {
		return response.Response[BackupResponse]{}, apperrors.Conflict("备份尚未完成")
	}
	current, err := r.repo.find(ctx, scope.ActorID, request.ID)
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	path, err := r.safePath(current.StorageKey)
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	if _, err := os.Stat(path); err != nil {
		return response.Response[BackupResponse]{}, apperrors.NotFound("备份文件不存在")
	}
	result.Data.LocalPath = path
	return result, nil
}

func (r *Resource) fail(ctx context.Context, id int64, message string) (response.Response[BackupResponse], error) {
	return r.repo.Finish(ctx, &BackupRequest{ID: id}, map[string]any{
		"status":        "failed",
		"error_message": trim(message),
	})
}

func (r *Resource) safePath(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") {
		return "", apperrors.Invalid("备份路径无效")
	}
	root := filepath.Join(r.cfg.App.DataDir, "backups")
	path := filepath.Clean(filepath.Join(r.cfg.App.DataDir, key))
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", apperrors.Invalid("备份路径无效")
	}
	return path, nil
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

func fileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func trim(message string) string {
	if len(message) > 2000 {
		return message[:2000]
	}
	return message
}

type limitedWriter struct {
	writer io.Writer
	max    int64
	used   int64
}

func (w *limitedWriter) Write(payload []byte) (int, error) {
	if w.max > 0 && w.used+int64(len(payload)) > w.max {
		return 0, errors.New("备份超过大小限制")
	}
	count, err := w.writer.Write(payload)
	w.used += int64(count)
	return count, err
}

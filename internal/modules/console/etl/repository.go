package etl

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cng1985/dbgo/internal/platform/database"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(handle *database.Handle) *Repository {
	return &Repository{db: handle.Gorm}
}

func (r *Repository) Create(ctx context.Context, entity *Task) (response.Response[TaskResponse], error) {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if database.IsUnique(err) {
			return response.Response[TaskResponse]{}, apperrors.Conflict("任务名称已存在")
		}
		return response.Response[TaskResponse]{}, err
	}
	mappings, err := decodeMappings(entity.Mappings)
	if err != nil {
		return response.Response[TaskResponse]{}, apperrors.Invalid("字段对应已损坏")
	}
	return response.OK(toResponse(entity, mappings)), nil
}

func (r *Repository) Update(ctx context.Context, ownerID int64, request *TaskRequest, fields map[string]any) (response.Response[TaskResponse], error) {
	var result response.Response[TaskResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findTaskTx(tx, ownerID, request.ID)
		if err != nil {
			return err
		}
		if entity.Version != request.Version {
			return apperrors.Conflict("任务已被其他人修改")
		}
		fields["version"] = entity.Version + 1
		fields["updated_at"] = time.Now().UTC()
		updated := tx.Model(&Task{}).Where("id = ? and created_by = ? and version = ?", request.ID, ownerID, request.Version).Updates(fields)
		if updated.Error != nil {
			if database.IsUnique(updated.Error) {
				return apperrors.Conflict("任务名称已存在")
			}
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("任务已被其他人修改")
		}
		entity, err = findTaskTx(tx, ownerID, request.ID)
		if err != nil {
			return err
		}
		mappings, err := decodeMappings(entity.Mappings)
		if err != nil {
			return apperrors.Invalid("字段对应已损坏")
		}
		result = response.OK(toResponse(entity, mappings))
		return nil
	})
	if err != nil {
		return response.Response[TaskResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, ownerID, id int64) (*Task, error) {
	return r.find(ctx, ownerID, id)
}

func (r *Repository) Delete(ctx context.Context, ownerID, id int64) (response.Response[TaskResponse], error) {
	var result response.Response[TaskResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findTaskTx(tx, ownerID, id)
		if err != nil {
			return err
		}
		if err := tx.Where("task_id = ? and created_by = ?", id, ownerID).Delete(&Run{}).Error; err != nil {
			return err
		}
		deleted := tx.Where("id = ? and created_by = ?", id, ownerID).Delete(&Task{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.NotFound("任务不存在")
		}
		mappings, err := decodeMappings(entity.Mappings)
		if err != nil {
			mappings = []ColumnMap{}
		}
		result = response.OK(toResponse(entity, mappings))
		return nil
	})
	if err != nil {
		return response.Response[TaskResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, ownerID int64, search *TaskSearch) (response.PageResponse[TaskSimple], error) {
	var result response.PageResponse[TaskSimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	orderBy, err := domain.OrderBy(search.Sort, search.Direction, map[string]string{
		"id":         "id",
		"name":       "name",
		"updated_at": "updated_at",
	}, "updated_at")
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Task{}).Where("created_by = ?", ownerID)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			like := "%" + escapeLike(keyword) + "%"
			query = query.Where("name like ? escape '\\'", like)
		}
		return query
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []Task
	if err := makeQuery().Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	ids := make([]int64, 0, len(entities))
	for i := range entities {
		ids = append(ids, entities[i].ID)
	}
	latest, err := r.latestRuns(ctx, ownerID, ids)
	if err != nil {
		return result, err
	}
	items := make([]TaskSimple, 0, len(entities))
	for i := range entities {
		run := latest[entities[i].ID]
		items = append(items, toSimple(&entities[i], run))
	}
	return response.PageResponse[TaskSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) Count(ctx context.Context, ownerID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Task{}).Where("created_by = ?", ownerID).Count(&total).Error
	return total, err
}

func (r *Repository) CreateRun(ctx context.Context, entity *Run) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *Repository) FinishRun(ctx context.Context, ownerID, runID int64, status string, read, written int64, message string, finished time.Time) error {
	updated := r.db.WithContext(ctx).Model(&Run{}).
		Where("id = ? and created_by = ? and status = ?", runID, ownerID, "running").
		Updates(map[string]any{
			"status":        status,
			"rows_read":     read,
			"rows_written":  written,
			"error_message": trimMessage(message),
			"finished_at":   finished,
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return apperrors.Conflict("执行记录已经结束")
	}
	return nil
}

func (r *Repository) FailStale(ctx context.Context, ownerID, taskID int64, message string) (int64, error) {
	updated := r.db.WithContext(ctx).Model(&Run{}).
		Where("created_by = ? and task_id = ? and status = ?", ownerID, taskID, "running").
		Updates(map[string]any{
			"status":        "failed",
			"error_message": trimMessage(message),
			"finished_at":   time.Now().UTC(),
		})
	return updated.RowsAffected, updated.Error
}

func (r *Repository) SearchRuns(ctx context.Context, ownerID, taskID int64, page, pageSize int) (response.PageResponse[RunResponse], error) {
	var result response.PageResponse[RunResponse]
	page, pageSize, err := domain.NormalizePage(page, pageSize)
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&Run{}).Where("created_by = ? and task_id = ?", ownerID, taskID)
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []Run
	if err := makeQuery().Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]RunResponse, 0, len(entities))
	for i := range entities {
		items = append(items, toRun(&entities[i]))
	}
	return response.PageResponse[RunResponse]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) find(ctx context.Context, ownerID, id int64) (*Task, error) {
	var entity Task
	err := r.db.WithContext(ctx).Where("id = ? and created_by = ?", id, ownerID).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("任务不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) latestRuns(ctx context.Context, ownerID int64, taskIDs []int64) (map[int64]*Run, error) {
	found := map[int64]*Run{}
	if len(taskIDs) == 0 {
		return found, nil
	}
	var picked []struct {
		ID int64 `gorm:"column:id"`
	}
	err := r.db.WithContext(ctx).Model(&Run{}).
		Select("max(id) as id").
		Where("created_by = ? and task_id in ?", ownerID, taskIDs).
		Group("task_id").
		Scan(&picked).Error
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(picked))
	for _, item := range picked {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return found, nil
	}
	var runs []Run
	if err := r.db.WithContext(ctx).Where("id in ?", ids).Find(&runs).Error; err != nil {
		return nil, err
	}
	for i := range runs {
		found[runs[i].TaskID] = &runs[i]
	}
	return found, nil
}

func findTaskTx(tx *gorm.DB, ownerID, id int64) (*Task, error) {
	var entity Task
	err := tx.Where("id = ? and created_by = ?", id, ownerID).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("任务不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func trimMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 2000 {
		message = message[:2000]
	}
	return message
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

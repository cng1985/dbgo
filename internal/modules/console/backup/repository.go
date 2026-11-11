package backup

import (
	"context"
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

func (r *Repository) Create(ctx context.Context, entity *DataBackup) (response.Response[BackupResponse], error) {
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return response.Response[BackupResponse]{}, err
	}
	return response.OK(*BackupEntityToResponse(entity)), nil
}

func (r *Repository) Finish(ctx context.Context, request *BackupRequest, fields map[string]any) (response.Response[BackupResponse], error) {
	var result response.Response[BackupResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&DataBackup{}).Where("id = ? and status = ?", request.ID, "running").Updates(fields)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("备份状态已变化")
		}
		var entity DataBackup
		if err := tx.Where("id = ?", request.ID).Take(&entity).Error; err != nil {
			return err
		}
		result = response.OK(*BackupEntityToResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, ownerID int64, request *BackupRequest) (response.Response[BackupResponse], error) {
	entity, err := r.find(ctx, ownerID, request.ID)
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	return response.OK(*BackupEntityToResponse(entity)), nil
}

func (r *Repository) Delete(ctx context.Context, ownerID int64, request *BackupRequest) (response.Response[BackupResponse], error) {
	var result response.Response[BackupResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity DataBackup
		if err := tx.Where("id = ? and created_by = ?", request.ID, ownerID).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("备份不存在")
			}
			return err
		}
		if entity.Status == "running" {
			return apperrors.Conflict("备份仍在进行，不能删除")
		}
		deleted := tx.Where("id = ? and status <> ?", entity.ID, "running").Delete(&DataBackup{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.Conflict("备份状态已变化")
		}
		result = response.OK(*BackupEntityToResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[BackupResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, search *BackupSearch) (response.PageResponse[BackupSimple], error) {
	var result response.PageResponse[BackupSimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&DataBackup{}).Where("created_by = ?", search.OwnerID)
		if search.ConnectionID > 0 {
			query = query.Where("connection_id = ?", search.ConnectionID)
		}
		if search.Status != "" {
			query = query.Where("status = ?", search.Status)
		}
		return query
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []DataBackup
	offset := (page - 1) * pageSize
	if err := makeQuery().Order("id desc").Offset(offset).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]BackupSimple, 0, len(entities))
	for i := range entities {
		items = append(items, *BackupEntityToSimple(&entities[i]))
	}
	return response.PageResponse[BackupSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) find(ctx context.Context, ownerID, id int64) (*DataBackup, error) {
	var entity DataBackup
	err := r.db.WithContext(ctx).Where("id = ? and created_by = ?", id, ownerID).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("备份不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

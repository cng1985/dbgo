package migration

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

func (r *Repository) Create(ctx context.Context, entity *DataMigration) (response.Response[MigrationResponse], error) {
	var result response.Response[MigrationResponse]
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if database.IsUnique(err) {
			return result, apperrors.Conflict("该连接下的迁移版本已存在")
		}
		return result, err
	}
	return response.OK(*MigrationEntityToResponse(entity)), nil
}

func (r *Repository) Update(ctx context.Context, entity *DataMigration) (response.Response[MigrationResponse], error) {
	var result response.Response[MigrationResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current DataMigration
		if err := tx.Where("id = ?", entity.ID).Take(&current).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("迁移不存在")
			}
			return err
		}
		if current.Status != "draft" && current.Status != "failed" {
			return apperrors.Conflict("只有草稿或失败的迁移可以修改")
		}
		updated := tx.Model(&DataMigration{}).Where("id = ? and status in ?", entity.ID, []string{"draft", "failed"}).Updates(map[string]any{
			"name":          entity.Name,
			"up_sql":        entity.UpSQL,
			"down_sql":      entity.DownSQL,
			"checksum":      entity.Checksum,
			"status":        "draft",
			"error_message": "",
			"updated_at":    entity.UpdatedAt,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("迁移状态已变化")
		}
		if err := tx.Where("id = ?", entity.ID).Take(&current).Error; err != nil {
			return err
		}
		result = response.OK(*MigrationEntityToResponse(&current))
		return nil
	})
	if err != nil {
		return response.Response[MigrationResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	entity, err := r.find(ctx, request.ID)
	if err != nil {
		return response.Response[MigrationResponse]{}, err
	}
	return response.OK(*MigrationEntityToResponse(entity)), nil
}

func (r *Repository) Delete(ctx context.Context, request *MigrationRequest) (response.Response[MigrationResponse], error) {
	var result response.Response[MigrationResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity DataMigration
		if err := tx.Where("id = ?", request.ID).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("迁移不存在")
			}
			return err
		}
		if entity.Status != "draft" && entity.Status != "failed" {
			return apperrors.Conflict("只有草稿或失败的迁移可以删除")
		}
		deleted := tx.Where("id = ? and status in ?", entity.ID, []string{"draft", "failed"}).Delete(&DataMigration{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.Conflict("迁移状态已变化")
		}
		result = response.OK(*MigrationEntityToResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[MigrationResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, search *MigrationSearch) (response.PageResponse[MigrationSimple], error) {
	var result response.PageResponse[MigrationSimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&DataMigration{}).
			Where("connection_id in (select id from db_connection where created_by = ?)", search.OwnerID)
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
	var entities []DataMigration
	offset := (page - 1) * pageSize
	if err := makeQuery().Order("connection_id asc, version_no asc, id desc").Offset(offset).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]MigrationSimple, 0, len(entities))
	for i := range entities {
		items = append(items, *MigrationEntityToSimple(&entities[i]))
	}
	return response.PageResponse[MigrationSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) claim(ctx context.Context, id int64, from, to string) (*DataMigration, error) {
	var entity DataMigration
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("迁移不存在")
			}
			return err
		}
		if entity.Status != from {
			return apperrors.Conflict("迁移当前状态不允许该操作")
		}
		updated := tx.Model(&DataMigration{}).Where("id = ? and status = ?", id, from).Updates(map[string]any{
			"status":        to,
			"error_message": "",
			"updated_at":    time.Now().UTC(),
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("迁移状态已变化")
		}
		entity.Status = to
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) finish(ctx context.Context, id int64, from, to string, appliedAt *time.Time, message string) (*DataMigration, error) {
	updated := r.db.WithContext(ctx).Model(&DataMigration{}).Where("id = ? and status = ?", id, from).Updates(map[string]any{
		"status":        to,
		"error_message": message,
		"applied_at":    appliedAt,
		"updated_at":    time.Now().UTC(),
	})
	if updated.Error != nil {
		return nil, updated.Error
	}
	if updated.RowsAffected != 1 {
		return nil, apperrors.Conflict("迁移状态已变化")
	}
	return r.find(ctx, id)
}

func (r *Repository) listStatus(ctx context.Context, connectionID int64, status string) ([]DataMigration, error) {
	var items []DataMigration
	err := r.db.WithContext(ctx).Where("connection_id = ? and status = ?", connectionID, status).
		Order("version_no asc, id asc").Limit(201).Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) > 200 {
		return nil, apperrors.Invalid("待处理迁移超过 200 条")
	}
	return items, nil
}

func (r *Repository) claimLatestRollback(ctx context.Context, id int64) (*DataMigration, error) {
	var entity DataMigration
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("迁移不存在")
			}
			return err
		}
		if entity.Status != "applied" {
			return apperrors.Conflict("只有已执行的迁移可以回滚")
		}
		var latest int64
		if err := tx.Model(&DataMigration{}).
			Where("connection_id = ? and status = ?", entity.ConnectionID, "applied").
			Select("coalesce(max(version_no), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		if entity.VersionNo != latest {
			return apperrors.Conflict("只能回滚该连接最新的已执行版本")
		}
		updated := tx.Model(&DataMigration{}).Where("id = ? and status = ?", id, "applied").Updates(map[string]any{
			"status":        "reverting",
			"error_message": "",
			"updated_at":    time.Now().UTC(),
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("迁移状态已变化")
		}
		entity.Status = "reverting"
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) find(ctx context.Context, id int64) (*DataMigration, error) {
	var entity DataMigration
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("迁移不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// DeleteOwnedWithTx 只使用调用方传入的事务，删除某个连接名下的迁移定义。
func (r *Repository) DeleteOwnedWithTx(ctx context.Context, tx *gorm.DB, connectionID int64) error {
	if tx == nil {
		return apperrors.Internal(nil)
	}
	var busy int64
	if err := tx.WithContext(ctx).Model(&DataMigration{}).
		Where("connection_id = ? and status in ?", connectionID, []string{"applying", "reverting"}).
		Count(&busy).Error; err != nil {
		return err
	}
	if busy > 0 {
		return apperrors.Conflict("连接仍有执行中的迁移，不能删除")
	}
	return tx.WithContext(ctx).Where("connection_id = ?", connectionID).Delete(&DataMigration{}).Error
}

package connection

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

type MigrationCleaner interface {
	DeleteOwnedWithTx(ctx context.Context, tx *gorm.DB, connectionID int64) error
}

type Repository struct {
	db      *gorm.DB
	cleaner MigrationCleaner
}

func NewRepository(handle *database.Handle, cleaner MigrationCleaner) *Repository {
	return &Repository{db: handle.Gorm, cleaner: cleaner}
}

func (r *Repository) Create(ctx context.Context, entity *Connection) (response.Response[ConnectionResponse], error) {
	var result response.Response[ConnectionResponse]
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if database.IsUnique(err) {
			return result, apperrors.Conflict("连接名称已存在")
		}
		return result, err
	}
	return response.OK(*ConnectionEntityToResponse(entity)), nil
}

func (r *Repository) Update(ctx context.Context, request *ConnectionRequest, fields map[string]any) (response.Response[ConnectionResponse], error) {
	var result response.Response[ConnectionResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity Connection
		if err := tx.Where("id = ?", request.ID).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("连接不存在")
			}
			return err
		}
		if entity.Version != request.Version {
			return apperrors.Conflict("连接已被其他人修改")
		}
		fields["version"] = entity.Version + 1
		fields["updated_at"] = time.Now().UTC()
		updated := tx.Model(&Connection{}).Where("id = ? and version = ?", request.ID, request.Version).Updates(fields)
		if updated.Error != nil {
			if database.IsUnique(updated.Error) {
				return apperrors.Conflict("连接名称已存在")
			}
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("连接已被其他人修改")
		}
		if err := tx.Where("id = ?", request.ID).Take(&entity).Error; err != nil {
			return err
		}
		result = response.OK(*ConnectionEntityToResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[ConnectionResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, request *ConnectionRequest) (response.Response[ConnectionResponse], error) {
	var result response.Response[ConnectionResponse]
	entity, err := r.find(ctx, request.ID)
	if err != nil {
		return result, err
	}
	return response.OK(*ConnectionEntityToResponse(entity)), nil
}

func (r *Repository) Delete(ctx context.Context, request *ConnectionDeleteRequest) (response.Response[ConnectionResponse], error) {
	var result response.Response[ConnectionResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity Connection
		if err := tx.Where("id = ?", request.ID).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("连接不存在")
			}
			return err
		}
		if r.cleaner == nil {
			return apperrors.Internal(nil)
		}
		if err := r.cleaner.DeleteOwnedWithTx(ctx, tx, entity.ID); err != nil {
			return err
		}
		deleted := tx.Where("id = ?", entity.ID).Delete(&Connection{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.NotFound("连接不存在")
		}
		result = response.OK(*ConnectionEntityToResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[ConnectionResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, search *ConnectionSearch) (response.PageResponse[ConnectionSimple], error) {
	var result response.PageResponse[ConnectionSimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	orderBy, err := domain.OrderBy(search.Sort, search.Direction, map[string]string{
		"id":         "id",
		"name":       "name",
		"created_at": "created_at",
	}, "id")
	if err != nil {
		return result, err
	}
	search.Page = page
	search.PageSize = pageSize
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Connection{})
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name like ? escape '\\'", "%"+escapeLike(keyword)+"%")
		}
		if driver := strings.TrimSpace(search.Driver); driver != "" {
			query = query.Where("driver = ?", driver)
		}
		return query
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []Connection
	offset := (page - 1) * pageSize
	if err := makeQuery().Order(orderBy).Offset(offset).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]ConnectionSimple, 0, len(entities))
	for i := range entities {
		items = append(items, *ConnectionEntityToSimple(&entities[i]))
	}
	return response.PageResponse[ConnectionSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) List(ctx context.Context, ids []int64) ([]Connection, error) {
	query := r.db.WithContext(ctx).Model(&Connection{}).Order("name asc")
	if len(ids) > 0 {
		query = query.Where("id in ?", ids)
	}
	var entities []Connection
	if err := query.Limit(500).Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Connection{}).Count(&total).Error
	return total, err
}

func (r *Repository) CreateMany(ctx context.Context, entities []*Connection) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			if err := tx.Create(entity).Error; err != nil {
				if database.IsUnique(err) {
					return apperrors.Conflict("连接名称已存在")
				}
				return err
			}
		}
		return nil
	})
}

func (r *Repository) find(ctx context.Context, id int64) (*Connection, error) {
	var entity Connection
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("连接不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

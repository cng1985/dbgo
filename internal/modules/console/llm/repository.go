package llm

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

func (r *Repository) Create(ctx context.Context, entity *Profile) (response.Response[ProfileResponse], error) {
	var result response.Response[ProfileResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entity.IsDefault {
			if err := clearDefault(tx, 0); err != nil {
				return err
			}
		}
		if err := tx.Create(entity).Error; err != nil {
			if database.IsUnique(err) {
				return apperrors.Conflict("模型名称已存在")
			}
			return err
		}
		result = response.OK(toResponse(entity))
		return nil
	})
	if err != nil {
		return response.Response[ProfileResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Update(ctx context.Context, id, version int64, fields map[string]any, makeDefault bool) (response.Response[ProfileResponse], error) {
	var result response.Response[ProfileResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity Profile
		if err := tx.Where("id = ?", id).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("模型配置不存在")
			}
			return err
		}
		if entity.Version != version {
			return apperrors.Conflict("模型配置已被其他人修改")
		}
		if makeDefault {
			if err := clearDefault(tx, id); err != nil {
				return err
			}
		}
		fields["version"] = entity.Version + 1
		fields["updated_at"] = time.Now().UTC()
		updated := tx.Model(&Profile{}).Where("id = ? and version = ?", id, version).Updates(fields)
		if updated.Error != nil {
			if database.IsUnique(updated.Error) {
				return apperrors.Conflict("模型名称已存在")
			}
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("模型配置已被其他人修改")
		}
		if err := tx.Where("id = ?", id).Take(&entity).Error; err != nil {
			return err
		}
		result = response.OK(toResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[ProfileResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, id int64) (*Profile, error) {
	var entity Profile
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("模型配置不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) (response.Response[ProfileResponse], error) {
	var result response.Response[ProfileResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity Profile
		if err := tx.Where("id = ?", id).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("模型配置不存在")
			}
			return err
		}
		deleted := tx.Where("id = ?", id).Delete(&Profile{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.NotFound("模型配置不存在")
		}
		result = response.OK(toResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[ProfileResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, search *ProfileSearch) (response.PageResponse[ProfileSimple], error) {
	var result response.PageResponse[ProfileSimple]
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
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Profile{})
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			like := "%" + escapeLike(keyword) + "%"
			query = query.Where("name like ? escape '\\' or model like ? escape '\\'", like, like)
		}
		return query
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []Profile
	if err := makeQuery().Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]ProfileSimple, 0, len(entities))
	for i := range entities {
		items = append(items, toSimple(&entities[i]))
	}
	return response.PageResponse[ProfileSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) FindDefault(ctx context.Context) (*Profile, error) {
	var entity Profile
	err := r.db.WithContext(ctx).Where("is_default = ?", true).Order("id desc").Take(&entity).Error
	if err == nil {
		return &entity, nil
	}
	if !database.IsNotFound(err) {
		return nil, err
	}
	err = r.db.WithContext(ctx).Order("id asc").Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("还没有模型配置")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func clearDefault(tx *gorm.DB, except int64) error {
	query := tx.Model(&Profile{}).Where("is_default = ?", true)
	if except > 0 {
		query = query.Where("id <> ?", except)
	}
	return query.Update("is_default", false).Error
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

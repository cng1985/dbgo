package snippet

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

func (r *Repository) Create(ctx context.Context, entity *Snippet) (response.Response[SnippetResponse], error) {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if database.IsUnique(err) {
			return response.Response[SnippetResponse]{}, apperrors.Conflict("代码段名称已存在")
		}
		return response.Response[SnippetResponse]{}, err
	}
	return response.OK(toResponse(entity)), nil
}

func (r *Repository) Update(ctx context.Context, ownerID int64, request *SnippetRequest, fields map[string]any) (response.Response[SnippetResponse], error) {
	var result response.Response[SnippetResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity Snippet
		if err := tx.Where("id = ? and created_by = ?", request.ID, ownerID).Take(&entity).Error; err != nil {
			if database.IsNotFound(err) {
				return apperrors.NotFound("代码段不存在")
			}
			return err
		}
		if entity.Version != request.Version {
			return apperrors.Conflict("代码段已被其他人修改")
		}
		fields["version"] = entity.Version + 1
		fields["updated_at"] = time.Now().UTC()
		updated := tx.Model(&Snippet{}).Where("id = ? and created_by = ? and version = ?", request.ID, ownerID, request.Version).Updates(fields)
		if updated.Error != nil {
			if database.IsUnique(updated.Error) {
				return apperrors.Conflict("代码段名称已存在")
			}
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return apperrors.Conflict("代码段已被其他人修改")
		}
		if err := tx.Where("id = ? and created_by = ?", request.ID, ownerID).Take(&entity).Error; err != nil {
			return err
		}
		result = response.OK(toResponse(&entity))
		return nil
	})
	if err != nil {
		return response.Response[SnippetResponse]{}, err
	}
	return result, nil
}

func (r *Repository) One(ctx context.Context, ownerID, id int64) (*Snippet, error) {
	return r.find(ctx, ownerID, id)
}

func (r *Repository) Delete(ctx context.Context, ownerID, id int64) (response.Response[SnippetResponse], error) {
	var result response.Response[SnippetResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := r.findTx(tx, ownerID, id)
		if err != nil {
			return err
		}
		deleted := tx.Where("id = ? and created_by = ?", id, ownerID).Delete(&Snippet{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return apperrors.NotFound("代码段不存在")
		}
		result = response.OK(toResponse(entity))
		return nil
	})
	if err != nil {
		return response.Response[SnippetResponse]{}, err
	}
	return result, nil
}

func (r *Repository) Search(ctx context.Context, ownerID int64, search *SnippetSearch) (response.PageResponse[SnippetSimple], error) {
	var result response.PageResponse[SnippetSimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	orderBy, err := domain.OrderBy(search.Sort, search.Direction, map[string]string{
		"id":         "id",
		"name":       "name",
		"created_at": "created_at",
	}, "name")
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Snippet{}).Where("created_by = ?", ownerID)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			like := "%" + escapeLike(keyword) + "%"
			query = query.Where("name like ? escape '\\' or description like ? escape '\\'", like, like)
		}
		return query
	}
	var total int64
	if err := makeQuery().Count(&total).Error; err != nil {
		return result, err
	}
	var entities []Snippet
	if err := makeQuery().Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]SnippetSimple, 0, len(entities))
	for i := range entities {
		items = append(items, toSimple(&entities[i]))
	}
	return response.PageResponse[SnippetSimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *Repository) List(ctx context.Context, ownerID int64, ids []int64) ([]Snippet, error) {
	query := r.db.WithContext(ctx).Model(&Snippet{}).Where("created_by = ?", ownerID).Order("name asc")
	if len(ids) > 0 {
		query = query.Where("id in ?", ids)
	}
	var entities []Snippet
	if err := query.Limit(500).Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (r *Repository) Count(ctx context.Context, ownerID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&Snippet{}).Where("created_by = ?", ownerID).Count(&total).Error
	return total, err
}

func (r *Repository) CreateMany(ctx context.Context, entities []*Snippet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			if err := tx.Create(entity).Error; err != nil {
				if database.IsUnique(err) {
					return apperrors.Conflict("代码段名称已存在")
				}
				return err
			}
		}
		return nil
	})
}

func (r *Repository) find(ctx context.Context, ownerID, id int64) (*Snippet, error) {
	var entity Snippet
	err := r.db.WithContext(ctx).Where("id = ? and created_by = ?", id, ownerID).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("代码段不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) findTx(tx *gorm.DB, ownerID, id int64) (*Snippet, error) {
	var entity Snippet
	err := tx.Where("id = ? and created_by = ?", id, ownerID).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("代码段不存在")
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

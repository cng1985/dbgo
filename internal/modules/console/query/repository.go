package query

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

func (r *Repository) insert(ctx context.Context, entity *QueryHistory) (int64, error) {
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return 0, err
	}
	return entity.ID, nil
}

func (r *Repository) One(ctx context.Context, request *QueryOneRequest) (response.Response[QueryResponse], error) {
	var result response.Response[QueryResponse]
	var entity QueryHistory
	err := r.db.WithContext(ctx).Where("id = ?", request.ID).Take(&entity).Error
	if database.IsNotFound(err) {
		return result, apperrors.NotFound("查询记录不存在")
	}
	if err != nil {
		return result, err
	}
	return response.OK(*QueryEntityToResponse(&entity)), nil
}

func (r *Repository) Search(ctx context.Context, search *QuerySearch) (response.PageResponse[QuerySimple], error) {
	var result response.PageResponse[QuerySimple]
	page, pageSize, err := domain.NormalizePage(search.Page, search.PageSize)
	if err != nil {
		return result, err
	}
	makeQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&QueryHistory{})
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
	var entities []QueryHistory
	offset := (page - 1) * pageSize
	if err := makeQuery().Order("id desc").Offset(offset).Limit(pageSize).Find(&entities).Error; err != nil {
		return result, err
	}
	items := make([]QuerySimple, 0, len(entities))
	for i := range entities {
		items = append(items, *QueryEntityToSimple(&entities[i]))
	}
	return response.PageResponse[QuerySimple]{Data: items, Total: total, Page: page, PageSize: pageSize}, nil
}

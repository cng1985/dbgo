package auth

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/cng1985/dbgo/internal/platform/database"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(handle *database.Handle) *Repository {
	return &Repository{db: handle.Gorm}
}

func (r *Repository) count(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&User{}).Count(&total).Error
	return total, err
}

// EnsureAdmin 只在没有任何用户时插入管理员。request.Password 必须已经是口令哈希。
func (r *Repository) EnsureAdmin(ctx context.Context, request *AuthEnsureRequest) (response.Response[AuthProfileResponse], error) {
	var result response.Response[AuthProfileResponse]
	var existing User
	err := r.db.WithContext(ctx).Where("username = ?", request.Username).Take(&existing).Error
	if err == nil {
		result = response.OK(*UserEntityToProfile(&existing, nil))
		return result, nil
	}
	if !database.IsNotFound(err) {
		return result, err
	}
	now := time.Now().UTC()
	entity := &User{
		Username:     request.Username,
		PasswordHash: request.Password,
		DisplayName:  "管理员",
		RoleCode:     "admin",
		Disabled:     false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		if database.IsUnique(err) {
			return r.EnsureAdmin(ctx, request)
		}
		return result, err
	}
	return response.OK(*UserEntityToProfile(entity, nil)), nil
}

func (r *Repository) findByUsername(ctx context.Context, username string) (*User, error) {
	var entity User
	err := r.db.WithContext(ctx).Where("username = ?", username).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("用户不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) findByID(ctx context.Context, id int64) (*User, error) {
	var entity User
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&entity).Error
	if database.IsNotFound(err) {
		return nil, apperrors.NotFound("用户不存在")
	}
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) updatePassword(ctx context.Context, userID int64, hash string) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash": hash,
		"updated_at":    time.Now().UTC(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.NotFound("用户不存在")
	}
	return nil
}

func (r *Repository) createSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	entity := &Session{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt.UTC(),
		CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *Repository) findSession(ctx context.Context, tokenHash string) (*Session, *User, error) {
	var session Session
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Take(&session).Error
	if database.IsNotFound(err) {
		return nil, nil, apperrors.Unauthenticated("")
	}
	if err != nil {
		return nil, nil, err
	}
	user, err := r.findByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, err
	}
	return &session, user, nil
}

func (r *Repository) revokeSession(ctx context.Context, tokenHash string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&Session{}).
		Where("token_hash = ? and revoked_at is null", tokenHash).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *Repository) revokeUserSessions(ctx context.Context, userID int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&Session{}).
		Where("user_id = ? and revoked_at is null", userID).
		Update("revoked_at", now).Error
}

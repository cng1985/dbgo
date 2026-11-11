package auth

import (
	"context"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/platform/config"
	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/response"
)

type Resource struct {
	repo *Repository
	ttl  time.Duration
	cfg  *config.Config
	log  *logging.Logger
}

func NewResource(repo *Repository, cfg *config.Config, log *logging.Logger) *Resource {
	return &Resource{repo: repo, ttl: cfg.Auth.SessionTTL, cfg: cfg, log: log}
}

func (r *Resource) Login(ctx context.Context, request *AuthLoginRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, err
	}
	if request.Password == "" {
		return zero, apperrors.Invalid("密码不能为空")
	}
	user, findErr := r.repo.findByUsername(ctx, username)
	if findErr != nil {
		if appErr, ok := apperrors.As(findErr); !ok || appErr.Kind != apperrors.KindNotFound {
			return zero, findErr
		}
	}
	hash := ""
	if user != nil {
		hash = user.PasswordHash
	}
	if !checkPassword(hash, request.Password) || user == nil || user.Disabled {
		return zero, apperrors.Invalid("用户名或密码错误")
	}
	return r.openSession(ctx, user)
}

func (r *Resource) openSession(ctx context.Context, user *User) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	token, tokenHash, err := newToken()
	if err != nil {
		return zero, apperrors.Internal(err)
	}
	expires := time.Now().UTC().Add(r.ttl)
	if err := r.repo.createSession(ctx, user.ID, tokenHash, expires); err != nil {
		return zero, err
	}
	profile := UserEntityToProfile(user, &expires)
	profile.SessionToken = token
	return response.OK(*profile), nil
}

func (r *Resource) Register(ctx context.Context, request *AuthRegisterRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	if !r.cfg.Auth.AllowRegister {
		return zero, apperrors.Forbidden("当前环境未开放注册")
	}
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, err
	}
	if strings.EqualFold(username, r.cfg.Auth.AdminUsername) {
		return zero, apperrors.Conflict("该用户名不可注册")
	}
	displayName, err := normalizeDisplayName(request.DisplayName, username)
	if err != nil {
		return zero, err
	}
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return zero, err
	}
	hash, err := hashPassword(request.Password)
	if err != nil {
		return zero, err
	}
	now := time.Now().UTC()
	user := &User{
		Username:     username,
		PasswordHash: hash,
		DisplayName:  displayName,
		Email:        email,
		RoleCode:     "member",
		Disabled:     false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.repo.createUser(ctx, user); err != nil {
		return zero, err
	}
	return r.openSession(ctx, user)
}

func (r *Resource) Options(_ context.Context, _ *AuthProfileRequest) (response.Response[AuthOptionsResponse], error) {
	return response.OK(AuthOptionsResponse{AllowRegister: r.cfg.Auth.AllowRegister}), nil
}

func (r *Resource) UpdateProfile(ctx context.Context, request *AuthProfileUpdateRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("auth:profile"); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	user, err := r.repo.findByID(ctx, scope.ActorID)
	if err != nil {
		return zero, err
	}
	displayName, err := normalizeDisplayName(request.DisplayName, user.Username)
	if err != nil {
		return zero, err
	}
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return zero, err
	}
	if err := r.repo.updateProfile(ctx, user.ID, displayName, email); err != nil {
		return zero, err
	}
	user.DisplayName = displayName
	user.Email = email
	return response.OK(*UserEntityToProfile(user, nil)), nil
}

func (r *Resource) Logout(ctx context.Context, _ *AuthLogoutRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	if _, err := domain.RequireScope(ctx); err != nil {
		return zero, err
	}
	token := domain.SessionToken(ctx)
	if token == "" {
		return zero, apperrors.Unauthenticated("")
	}
	if err := r.repo.revokeSession(ctx, hashToken(token)); err != nil {
		return zero, err
	}
	return response.OK(AuthProfileResponse{}), nil
}

func (r *Resource) Profile(ctx context.Context, _ *AuthProfileRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	user, err := r.repo.findByID(ctx, scope.ActorID)
	if err != nil {
		return zero, err
	}
	if user.Disabled {
		return zero, apperrors.Unauthenticated("")
	}
	return response.OK(*UserEntityToProfile(user, nil)), nil
}

func (r *Resource) ChangePassword(ctx context.Context, request *AuthPasswordRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	scope, err := domain.RequireScope(ctx)
	if err != nil {
		return zero, err
	}
	if err := scope.Allow("auth:password"); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	user, err := r.repo.findByID(ctx, scope.ActorID)
	if err != nil {
		return zero, err
	}
	if !checkPassword(user.PasswordHash, request.CurrentPassword) {
		return zero, apperrors.Invalid("当前密码不正确")
	}
	hash, err := hashPassword(request.NewPassword)
	if err != nil {
		return zero, err
	}
	if err := r.repo.updatePassword(ctx, user.ID, hash); err != nil {
		return zero, err
	}
	if err := r.repo.revokeUserSessions(ctx, user.ID); err != nil {
		return zero, err
	}
	return response.OK(*UserEntityToProfile(user, nil)), nil
}

func (r *Resource) EnsureAdmin(ctx context.Context, request *AuthEnsureRequest) (response.Response[AuthProfileResponse], error) {
	var zero response.Response[AuthProfileResponse]
	if request == nil {
		return zero, apperrors.Invalid("请求不能为空")
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, err
	}
	total, err := r.repo.count(ctx)
	if err != nil {
		return zero, err
	}
	if total > 0 {
		existing, findErr := r.repo.findByUsername(ctx, username)
		if findErr == nil {
			return response.OK(*UserEntityToProfile(existing, nil)), nil
		}
		if appErr, ok := apperrors.As(findErr); ok && appErr.Kind == apperrors.KindNotFound {
			return response.OK(AuthProfileResponse{}), nil
		}
		return zero, findErr
	}
	password := request.Password
	if password == "" {
		if r.cfg.App.Profile == "pro" {
			return zero, apperrors.Invalid("生产环境首次启动必须设置 APP_AUTH_ADMIN_PASSWORD")
		}
		password, err = randomPassword()
		if err != nil {
			return zero, apperrors.Internal(err)
		}
		path := adminPasswordPath(r.cfg)
		if writeErr := writeAdminPassword(path, password); writeErr != nil {
			return zero, writeErr
		}
		r.log.Info("已生成初始管理员口令", "path", path, "username", username)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return zero, err
	}
	stored := &AuthEnsureRequest{Username: username, Password: hash}
	return r.repo.EnsureAdmin(ctx, stored)
}

func (r *Resource) Authenticate(ctx context.Context, token string) (domain.Scope, error) {
	if token == "" {
		return domain.Scope{}, apperrors.Unauthenticated("")
	}
	session, user, err := r.repo.findSession(ctx, hashToken(token))
	if err != nil {
		return domain.Scope{}, apperrors.Unauthenticated("")
	}
	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) || user.Disabled {
		return domain.Scope{}, apperrors.Unauthenticated("")
	}
	return domain.Scope{ActorID: user.ID, Role: user.RoleCode}, nil
}

func normalizeUsername(username string) (string, error) {
	return responseTrim(username)
}

func responseTrim(username string) (string, error) {
	value, err := trimUsername(username)
	return value, err
}

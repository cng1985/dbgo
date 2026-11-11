package domain

import (
	"context"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

// Scope 是认证后写入请求上下文的可信身份。客户端提交的用户标识不能替代它。
type Scope struct {
	ActorID   int64
	RequestID string
	Role      string
}

type requestIDKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type scopeKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func ScopeFrom(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || scope.ActorID <= 0 {
		return Scope{}, false
	}
	return scope, true
}

func RequireScope(ctx context.Context) (Scope, error) {
	scope, ok := ScopeFrom(ctx)
	if !ok || scope.RequestID == "" {
		return Scope{}, apperrors.Unauthenticated("")
	}
	return scope, nil
}

// Allow 是当前共享控制台的角色判定。admin 与注册产生的 member 都能使用业务功能。
// 连接不按用户隔离。
type sessionTokenKey struct{}

func WithSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionTokenKey{}, token)
}

func SessionToken(ctx context.Context) string {
	token, _ := ctx.Value(sessionTokenKey{}).(string)
	return token
}

func (s Scope) Allow(permission string) error {
	if s.ActorID <= 0 {
		return apperrors.Unauthenticated("")
	}
	if permission == "" {
		return apperrors.Forbidden("")
	}
	switch s.Role {
	case "admin", "member":
		return nil
	default:
		return apperrors.Forbidden("")
	}
}

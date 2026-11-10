package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func contextWithTimeout(c *gin.Context, limit time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), limit)
}

func unauth() error { return apperrors.Unauthenticated("") }

func responseForbidden() error {
	return apperrors.Forbidden("来源不被允许")
}

func responseForbiddenHeader() error {
	return apperrors.Forbidden("缺少控制台请求标识")
}

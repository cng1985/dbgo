package response

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cng1985/dbgo/internal/platform/domain"
	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func HandleError(c *gin.Context, err error) {
	if err == nil || c == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		err = apperrors.Unavailable("执行超时", err)
	}
	appErr, ok := apperrors.As(err)
	if !ok {
		appErr = apperrors.Internal(err)
	}
	logHandled(c, appErr)
	if appErr.Kind == apperrors.KindCanceled {
		return
	}
	c.Abort()
	c.JSON(http.StatusOK, Response[any]{Code: appErr.Code, Msg: appErr.Message})
}

func JSONStatus(c *gin.Context, status int, code int, message string) {
	c.Abort()
	c.JSON(status, Response[any]{Code: code, Msg: message})
}

func logHandled(c *gin.Context, appErr *apperrors.Error) {
	logger, _ := c.Get("logger")
	slogger, _ := logger.(*slog.Logger)
	if slogger == nil {
		slogger = slog.Default()
	}
	scope, _ := domain.ScopeFrom(c.Request.Context())
	attrs := []any{
		"request_id", scope.RequestID,
		"actor_id", scope.ActorID,
		"route", c.FullPath(),
		"code", appErr.Code,
		"error_kind", string(appErr.Kind),
	}
	if appErr.Cause != nil {
		attrs = append(attrs, "error", appErr.Cause.Error())
	}
	if appErr.Code >= 500 {
		slogger.Error("request failed", attrs...)
		return
	}
	slogger.Info("request rejected", attrs...)
}

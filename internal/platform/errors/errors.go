// Package errors 定义与 Gin、GORM 和具体业务无关的错误分类。
package errors

import "errors"

type Kind string

const (
	KindInvalid         Kind = "invalid"
	KindUnauthenticated Kind = "unauthenticated"
	KindForbidden       Kind = "forbidden"
	KindNotFound        Kind = "not_found"
	KindConflict        Kind = "conflict"
	KindTooLarge        Kind = "too_large"
	KindRateLimited     Kind = "rate_limited"
	KindInternal        Kind = "internal"
	KindUnavailable     Kind = "unavailable"
	KindCanceled        Kind = "canceled"
)

// Error 是可公开消息与内部原因分离的应用错误。
type Error struct {
	Kind    Kind
	Code    int
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	if !ok || other == nil || e == nil {
		return false
	}
	if other.Kind != "" && other.Kind != e.Kind {
		return false
	}
	if other.Code != 0 && other.Code != e.Code {
		return false
	}
	if other.Message != "" && other.Message != e.Message {
		return false
	}
	return other.Kind != "" || other.Code != 0 || other.Message != ""
}

func New(kind Kind, code int, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Wrap(kind Kind, code int, message string, cause error) *Error {
	return &Error{Kind: kind, Code: code, Message: message, Cause: cause}
}

func Invalid(message string) *Error {
	return New(KindInvalid, 400, message)
}

func Unauthenticated(message string) *Error {
	if message == "" {
		message = "未登录或会话已失效"
	}
	return New(KindUnauthenticated, 401, message)
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "没有权限执行该操作"
	}
	return New(KindForbidden, 403, message)
}

func NotFound(message string) *Error {
	if message == "" {
		message = "记录不存在"
	}
	return New(KindNotFound, 404, message)
}

func Conflict(message string) *Error {
	return New(KindConflict, 409, message)
}

func TooLarge(message string) *Error {
	return New(KindTooLarge, 413, message)
}

func RateLimited(message string) *Error {
	if message == "" {
		message = "请求过于频繁，请稍后再试"
	}
	return New(KindRateLimited, 429, message)
}

func Internal(cause error) *Error {
	return Wrap(KindInternal, 500, "内部错误", cause)
}

func Unavailable(message string, cause error) *Error {
	if message == "" {
		message = "依赖暂不可用"
	}
	return Wrap(KindUnavailable, 503, message, cause)
}

func Canceled() *Error {
	return New(KindCanceled, 499, "请求已取消")
}

// Business 登记模块扩展码。公开消息由调用方给出，原因只进入日志。
func Business(code int, kind Kind, message string, cause error) *Error {
	if code < 10000 {
		code = 10000
	}
	if kind == "" {
		kind = KindInvalid
	}
	return &Error{Kind: kind, Code: code, Message: message, Cause: cause}
}

func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

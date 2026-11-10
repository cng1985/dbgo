package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func BindJSON(c *gin.Context, dst any) error {
	if c.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperrors.TooLarge("请求体超过限制")
		}
		return apperrors.Invalid("无法读取请求体")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperrors.TooLarge("请求体超过限制")
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return apperrors.Invalid("请求参数格式不正确")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return apperrors.Invalid("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func RejectClientID(id int64) error {
	if id != 0 {
		return apperrors.Invalid("创建时不能指定主键")
	}
	return nil
}

func RequireID(id int64) error {
	if id <= 0 {
		return apperrors.Invalid("主键无效")
	}
	return nil
}

func TrimRequired(value, label string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperrors.Invalid(label + "不能为空")
	}
	if max > 0 && len([]rune(value)) > max {
		return "", apperrors.Invalid(label + "超出长度限制")
	}
	return value, nil
}

func TrimLimit(value, label string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if max > 0 && len([]rune(value)) > max {
		return "", apperrors.Invalid(label + "超出长度限制")
	}
	return value, nil
}

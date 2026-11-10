package auth

import (
	"strings"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

func trimUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return "", apperrors.Invalid("用户名长度需要在 3 到 64 之间")
	}
	for _, ch := range username {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return "", apperrors.Invalid("用户名只允许字母、数字、下划线和连字符")
	}
	return username, nil
}

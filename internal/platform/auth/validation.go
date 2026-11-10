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

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", nil
	}
	if len([]rune(email)) > 120 || strings.ContainsAny(email, " \t\r\n") || strings.Count(email, "@") != 1 {
		return "", apperrors.Invalid("邮箱格式不正确")
	}
	local, host, _ := strings.Cut(email, "@")
	if local == "" || host == "" || strings.Contains(host, "@") {
		return "", apperrors.Invalid("邮箱格式不正确")
	}
	return email, nil
}

func normalizeDisplayName(name, fallback string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	if len([]rune(name)) > 80 {
		return "", apperrors.Invalid("显示名超出长度限制")
	}
	if name == "" {
		return "", apperrors.Invalid("显示名不能为空")
	}
	return name, nil
}

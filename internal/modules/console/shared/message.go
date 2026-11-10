package shared

import "strings"

func PublicError(prefix, secret string, err error) string {
	if err == nil {
		return prefix
	}
	detail := err.Error()
	if secret != "" {
		detail = strings.ReplaceAll(detail, secret, "***")
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	return prefix + "：" + detail
}

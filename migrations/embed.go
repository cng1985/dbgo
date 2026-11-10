// Package migrations 嵌入版本化 SQL。发布包与二进制使用同一份脚本。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

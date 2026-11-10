// Package buildinfo 保存版本、提交号和构建时间。发布脚本通过 -ldflags 注入。
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Time    = "unknown"
)

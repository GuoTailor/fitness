// Package migrations 内嵌全部 SQL 迁移脚本，供 internal/migrate 执行。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

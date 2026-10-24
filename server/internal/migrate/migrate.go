// Package migrate 提供极简 SQL 迁移：读取 migrations 包内嵌的 *.sql，
// 按文件名版本号顺序执行，已完成记录在 schema_migrations 表。
// MySQL DDL 隐式提交无法整体回滚，脚本失败即停止并报出所在语句。
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type migration struct {
	version int64
	name    string
	sql     string
}

func loadAll(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var ms []migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		verStr := strings.SplitN(name, "_", 2)[0]
		ver, err := strconv.ParseInt(verStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration %s: 文件名必须以数字版本号开头: %w", name, err)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		ms = append(ms, migration{version: ver, name: name, sql: string(data)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms, nil
}

// Up 执行 fsys 中全部未应用的迁移，返回本次应用数量。
func Up(ctx context.Context, db *gorm.DB, fsys fs.FS) (int, error) {
	if err := db.WithContext(ctx).Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version_id BIGINT NOT NULL PRIMARY KEY,
			name VARCHAR(255) NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		return 0, fmt.Errorf("create schema_migrations: %w", err)
	}

	var applied []int64
	if err := db.WithContext(ctx).Raw(`SELECT version_id FROM schema_migrations`).Scan(&applied).Error; err != nil {
		return 0, err
	}
	done := map[int64]bool{}
	for _, v := range applied {
		done[v] = true
	}

	ms, err := loadAll(fsys)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, m := range ms {
		if done[m.version] {
			continue
		}
		for _, stmt := range splitStatements(m.sql) {
			if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
				return count, fmt.Errorf("migration %s 失败（语句: %.80s…）: %w", m.name, stmt, err)
			}
		}
		if err := db.WithContext(ctx).Exec(
			`INSERT INTO schema_migrations (version_id, name) VALUES (?, ?)`, m.version, m.name).Error; err != nil {
			return count, fmt.Errorf("record migration %s: %w", m.name, err)
		}
		count++
	}
	return count, nil
}

// splitStatements 去掉 `--` 注释行后按分号切分；脚本约定不使用含分号的字符串字面量。
func splitStatements(script string) []string {
	var out []string
	var buf strings.Builder
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(buf.String())
			stmt = strings.TrimSuffix(stmt, ";")
			if stmt != "" {
				out = append(out, stmt)
			}
			buf.Reset()
		}
	}
	if stmt := strings.TrimSpace(buf.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out
}

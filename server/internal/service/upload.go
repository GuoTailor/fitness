package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fitness/internal/config"
	"fitness/internal/errs"
)

// UploadService 图片上传（评价图片等场景）。
// 白名单扩展名 + 魔数校验（防改后缀伪装）、随机文件名入日期子目录，
// 不接受客户端路径，防目录穿越与可执行文件上传。
type UploadService struct {
	dir      string
	baseURL  string
	maxBytes int64
}

func NewUploadService(cfg config.UploadConfig) (*UploadService, error) {
	dir := cfg.Dir
	if dir == "" {
		dir = "data/uploads"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errs.Server(err, "上传目录不可写")
	}
	base := cfg.BaseURL
	if base == "" {
		base = "/uploads"
	}
	max := int64(cfg.MaxMB)
	if max <= 0 {
		max = 2
	}
	return &UploadService{dir: dir, baseURL: strings.TrimRight(base, "/"), maxBytes: max << 20}, nil
}

// Dir 存储目录（路由注册静态直出用）
func (s *UploadService) Dir() string { return s.dir }

// SaveImage 校验并保存图片，返回可直接访问的 URL
func (s *UploadService) SaveImage(r io.Reader, filename string) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, s.maxBytes+1))
	if err != nil {
		return "", errs.Server(err, "读取文件失败")
	}
	if int64(len(data)) > s.maxBytes {
		return "", errs.BadParam(fmt.Sprintf("图片不能超过 %d MB", s.maxBytes>>20))
	}
	ext := strings.ToLower(filepath.Ext(filename))
	kind := ""
	switch ext {
	case ".jpg", ".jpeg":
		kind = ".jpg"
	case ".png":
		kind = ".png"
	case ".webp":
		kind = ".webp"
	default:
		return "", errs.BadParam("仅支持 jpg / png / webp 图片")
	}
	if !matchMagic(data, kind) {
		return "", errs.BadParam("文件内容与图片格式不符")
	}
	rel := "images/" + time.Now().Format("200601") + "/" + randHex(16) + kind
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", errs.Server(err, "创建上传目录失败")
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", errs.Server(err, "保存文件失败")
	}
	return s.baseURL + "/" + rel, nil
}

// matchMagic 魔数校验：jpg FF D8 FF；png 89 50 4E 47…；webp RIFF????WEBP
func matchMagic(data []byte, kind string) bool {
	switch kind {
	case ".jpg":
		return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
	case ".png":
		png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		return len(data) >= 8 && bytes.Equal(data[:8], png)
	case ".webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	}
	return false
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

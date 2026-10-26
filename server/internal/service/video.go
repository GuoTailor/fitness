package service

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fitness/internal/config"
	"fitness/internal/errs"
)

// VideoService 课程介绍视频上传。流式落盘（不整读进内存），
// 扩展名白名单 + 容器头校验，随机文件名入日期子目录，先写 .part 再原子改名。
type VideoService struct {
	dir      string
	baseURL  string
	maxBytes int64
}

func NewVideoService(cfg config.VideoConfig) (*VideoService, error) {
	dir := cfg.Dir
	if dir == "" {
		dir = "data/videos"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errs.Server(err, "视频目录不可写")
	}
	base := cfg.BaseURL
	if base == "" {
		base = "/videos"
	}
	max := int64(cfg.MaxMB)
	if max <= 0 {
		max = 200
	}
	return &VideoService{dir: dir, baseURL: strings.TrimRight(base, "/"), maxBytes: max << 20}, nil
}

// Dir 存储目录（路由注册静态直出用）
func (s *VideoService) Dir() string { return s.dir }

// MaxMB 单视频上限（MB），供错误提示
func (s *VideoService) MaxMB() int { return int(s.maxBytes >> 20) }

var videoExts = map[string]string{
	".mp4": ".mp4", ".m4v": ".m4v", ".mov": ".mov", ".webm": ".webm",
}

// Save 校验并保存视频，返回可直接播放的 URL
func (s *VideoService) Save(r io.Reader, filename string) (string, error) {
	kind, ok := videoExts[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		return "", errs.BadParam("仅支持 mp4 / mov / m4v / webm 视频")
	}
	br := bufio.NewReaderSize(r, 64*1024)
	head, _ := br.Peek(16)
	if len(head) < 12 {
		return "", errs.BadParam("视频文件为空或已损坏")
	}
	if !matchVideo(head, kind) {
		return "", errs.BadParam("文件内容与视频格式不符")
	}

	rel := time.Now().Format("200601") + "/" + randHex(16) + kind
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", errs.Server(err, "创建视频目录失败")
	}
	tmp := full + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", errs.Server(err, "创建文件失败")
	}
	written, err := io.Copy(f, io.LimitReader(br, s.maxBytes+1))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmp)
		return "", errs.Server(err, "保存视频失败")
	}
	if written > s.maxBytes {
		os.Remove(tmp)
		return "", errs.BadParam(fmt.Sprintf("视频不能超过 %d MB", s.MaxMB()))
	}
	if err := os.Rename(tmp, full); err != nil {
		os.Remove(tmp)
		return "", errs.Server(err, "保存视频失败")
	}
	return s.baseURL + "/" + rel, nil
}

// matchVideo 容器头校验：webm=EBML 1A45DFA3；mp4/m4v/mov 第 4-8 字节为 ftyp box
func matchVideo(head []byte, kind string) bool {
	if kind == ".webm" {
		return head[0] == 0x1A && head[1] == 0x45 && head[2] == 0xDF && head[3] == 0xA3
	}
	return string(head[4:8]) == "ftyp"
}

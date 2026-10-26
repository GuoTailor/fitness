package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fitness/internal/config"
)

func newVideoSvc(t *testing.T) (*VideoService, string) {
	t.Helper()
	dir := t.TempDir()
	svc, err := NewVideoService(config.VideoConfig{Dir: dir, BaseURL: "http://x/videos", MaxMB: 1})
	require.NoError(t, err)
	return svc, dir
}

// 构造带 ftyp box 的伪 mp4（仅校验头部，不做真实解码）
func fakeMP4(payload int) []byte {
	head := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	return append(head, bytes.Repeat([]byte{0}, payload)...)
}

func TestVideoSaveRejectsNonVideoExt(t *testing.T) {
	svc, _ := newVideoSvc(t)
	_, err := svc.Save(bytes.NewReader(fakeMP4(10)), "a.avi")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "仅支持")
}

func TestVideoSaveRejectsFakeMagic(t *testing.T) {
	svc, _ := newVideoSvc(t)
	_, err := svc.Save(strings.NewReader("this is not a video at all"), "a.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "格式不符")
}

func TestVideoSaveOK(t *testing.T) {
	svc, dir := newVideoSvc(t)
	url, err := svc.Save(bytes.NewReader(fakeMP4(1024)), "课程介绍.MP4")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(url, "http://x/videos/"))
	assert.True(t, strings.HasSuffix(url, ".mp4"))
	// 文件确实落盘且无残留 .part
	rel := strings.TrimPrefix(url, "http://x/videos/")
	_, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	assert.NoError(t, statErr)
	_, partErr := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)) + ".part")
	assert.True(t, os.IsNotExist(partErr))
}

func TestVideoSaveTooLarge(t *testing.T) {
	svc, dir := newVideoSvc(t)
	_, err := svc.Save(bytes.NewReader(fakeMP4(2<<20)), "big.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能超过")
	// 超限文件不得残留（空目录允许存在）
	files := 0
	filepath.WalkDir(dir, func(_ string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			files++
		}
		return nil
	})
	assert.Zero(t, files)
}

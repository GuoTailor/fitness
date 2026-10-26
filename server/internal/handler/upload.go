package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// UploadHandler 图片 / 视频上传（multipart 字段 file，wx.uploadFile 直接可用）
type UploadHandler struct {
	upload *service.UploadService
	video  *service.VideoService
}

func NewUploadHandler(u *service.UploadService, v *service.VideoService) *UploadHandler {
	return &UploadHandler{upload: u, video: v}
}

// Create POST /api/v1/uploads
func (h *UploadHandler) Create(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		resp.Handle(c, errs.BadParam("缺少文件字段 file"))
		return
	}
	if fh.Size <= 0 {
		resp.Handle(c, errs.BadParam("文件为空"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		resp.Handle(c, errs.Server(err, "读取文件失败"))
		return
	}
	defer f.Close()
	url, err := h.upload.SaveImage(f, fh.Filename)
	if err != nil {
		resp.Handle(c, err)
		return
	}
	resp.OK(c, gin.H{"url": url})
}

// CreateVideo POST /admin/api/v1/uploads/video
// 大文件流式解析（不落 gin 的临时缓冲），边读边写盘
func (h *UploadHandler) CreateVideo(c *gin.Context) {
	mr, err := c.Request.MultipartReader()
	if err != nil {
		resp.Handle(c, errs.BadParam("请使用 multipart/form-data 上传"))
		return
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			resp.Handle(c, errs.BadParam("读取上传内容失败"))
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		url, saveErr := h.video.Save(part, part.FileName())
		part.Close()
		if saveErr != nil {
			resp.Handle(c, saveErr)
			return
		}
		resp.OK(c, gin.H{"url": url})
		return
	}
	resp.Handle(c, errs.BadParam("缺少文件字段 file"))
}

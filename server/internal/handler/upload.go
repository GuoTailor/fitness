package handler

import (
	"github.com/gin-gonic/gin"

	"fitness/internal/errs"
	"fitness/internal/resp"
	"fitness/internal/service"
)

// UploadHandler 图片上传（评价图片等，multipart 字段 file，wx.uploadFile 直接可用）
type UploadHandler struct {
	upload *service.UploadService
}

func NewUploadHandler(u *service.UploadService) *UploadHandler { return &UploadHandler{upload: u} }

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

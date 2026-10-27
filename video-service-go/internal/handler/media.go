package handler

import (
	"io"
	"net/http"

	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// MediaHandler 提供图片上传（封面 / 头像 / 商品图 / 直播封面）。
//
// 为什么放在 video-service-go：它是唯一已经持有 MinIO 客户端与桶访问逻辑的服务，
// 新建一个 media-service 只会多一份重复的存储装配；后续真要拆再按前缀迁移。
type MediaHandler struct {
	svc *service.ImageService
}

// NewMediaHandler 构造图片 handler。
func NewMediaHandler(svc *service.ImageService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

// UploadImage 处理 `POST /api/v1/media/images`。
//
// 请求：multipart/form-data
//
//	file  <binary>
//	scene video_cover | avatar | product_cover | live_cover
//
// 响应：{url,width,height,sizeBytes,variants:{s,m}}
func (h *MediaHandler) UploadImage(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// 限流式读取：多留 1MB 余量给 multipart 边界与表单字段，
	// 真正的 5MB 判断在 service 里做（按文件字节数，而不是请求体大小）。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxImageBytes+(1<<20))

	if err := c.Request.ParseMultipartForm(service.MaxImageBytes); err != nil {
		response.Fail(c, errs.CodeFileTooLarge.New("图片不能超过 5MB"))
		return
	}

	scene := c.PostForm("scene")
	if scene == "" {
		scene = c.Query("scene")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.CodeInvalidParam.New("缺少 file 字段"))
		return
	}
	if fileHeader.Size > service.MaxImageBytes {
		response.Fail(c, errs.CodeFileTooLarge.New("图片不能超过 5MB"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.CodeInvalidParam.New("无法读取上传文件"))
		return
	}
	defer func() { _ = file.Close() }()

	// 多读 1 字节用于判断“是否真的超限”，避免依赖不可信的 Size 字段
	data, err := io.ReadAll(io.LimitReader(file, service.MaxImageBytes+1))
	if err != nil {
		response.Fail(c, errs.CodeInvalidParam.New("读取上传文件失败"))
		return
	}

	result, err := h.svc.SaveImage(c.Request.Context(), u.UID, scene, data)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

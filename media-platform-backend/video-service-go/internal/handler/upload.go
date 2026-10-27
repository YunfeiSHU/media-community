package handler

import (
	"strconv"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/internal/service"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/example/video-service-go/pkg/identity"
	"github.com/example/video-service-go/pkg/response"
	"github.com/gin-gonic/gin"
)

// UploadHandler 提供大文件分片上传与断点续传接口。
//
// 与 `POST /videos/:id/upload`（一次性上传）的分工：
//   - 小文件（几十 MB 以内）走一次性上传，一次请求搞定，最简单；
//   - 大文件走这里的 5 个接口，可并发传分片、可在任何一端断开后续传。
type UploadHandler struct {
	svc *service.VideoService
}

// NewUploadHandler 构造分片上传 handler。
func NewUploadHandler(svc *service.VideoService) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// Init 处理 `POST /videos/:id/upload/init`：初始化（或复用）分片上传会话。
func (h *UploadHandler) Init(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req dto.InitUploadRequest
	if err := bindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.InitChunkedUpload(c.Request.Context(), u, id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Status 处理 `GET /videos/:id/upload/status`：查询上传进度（断点续传入口）。
func (h *UploadHandler) Status(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.ChunkedUploadStatus(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Part 处理 `PUT /videos/:id/upload/parts/:partNumber`：上传单个分片。
//
// 请求体是**原始二进制**（application/octet-stream），不是 multipart 表单：
// 分片本身就是字节序列，再包一层 multipart 只会多一次编解码和一份内存拷贝。
func (h *UploadHandler) Part(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	partNumber, err := pathPartNumber(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// Content-Length 必须显式给出：对象存储要按长度校验分片，
	// 分块传输（chunked）下拿不到长度，只能让客户端改正。
	size := c.Request.ContentLength
	if size <= 0 {
		response.Fail(c, errs.CodeUploadPartInvalid.New("必须通过 Content-Length 声明分片大小"))
		return
	}

	result, err := h.svc.UploadChunk(c.Request.Context(), u, id, partNumber, c.Request.Body, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Complete 处理 `POST /videos/:id/upload/complete`：合并分片并提交转码。
func (h *UploadHandler) Complete(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.CompleteChunkedUpload(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// Abort 处理 `DELETE /videos/:id/upload`：取消上传并回收已上传的分片。
func (h *UploadHandler) Abort(c *gin.Context) {
	u, err := identity.Require(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	result, err := h.svc.AbortChunkedUpload(c.Request.Context(), u, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, result)
}

// pathPartNumber 解析路径参数 :partNumber。
func pathPartNumber(c *gin.Context) (int, error) {
	n, err := strconv.Atoi(c.Param("partNumber"))
	if err != nil || n <= 0 {
		return 0, errs.CodeUploadPartInvalid.New("路径参数 partNumber 必须是正整数")
	}
	return n, nil
}

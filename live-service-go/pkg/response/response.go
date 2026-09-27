// Package response 统一 HTTP 响应体。
//
// 对外契约与工作区 Java 侧一致： {"code":0,"message":"success","data":{...}}
// 失败时 data 固定为 null（Java 侧因 non_null 会省略该字段，前端只需判断 code）。
package response

import (
	"net/http"

	"github.com/example/live-service-go/pkg/errs"
	"github.com/gin-gonic/gin"
)

// Body 是统一响应体。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Page 是统一分页响应体。
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// Success 渲染成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data})
}

// NewPage 构造分页响应体；items 为 nil 时输出空数组而非 null，便于前端遍历。
func NewPage[T any](items []T, total int64, page, pageSize int) Page[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return Page[T]{Items: items, Total: total, Page: page, PageSize: pageSize}
}

// Failure 按错误码渲染失败响应。
func Failure(c *gin.Context, err error) {
	be, ok := errs.AsBizError(err)
	if !ok {
		be = errs.CodeInternal.Plain()
	}
	c.JSON(be.Code.HTTPStatus, Body{Code: be.Code.Code, Message: be.Message(), Data: nil})
}

// Fail 把错误登记到 gin.Context 并终止后续 handler，
// 由 middleware.ErrorHandler 统一渲染与记日志（保证一个请求只输出一次响应）。
func Fail(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

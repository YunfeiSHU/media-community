// Package errs 定义业务错误码与业务错误类型。
//
// 编号规范与工作区 Java 侧保持一致（见《Go微服务-公共约定.md》§4）：
// 40001 参数、40101 未认证、40301 无权限、404xx 不存在、409xx 冲突、429xx 限流、
// 5xxxx 内部/依赖，6xxxx 为 video 域专用段。
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode 描述一个业务错误：对外错误码、默认提示、对应 HTTP 状态码。
type ErrorCode struct {
	Code       int
	Message    string
	HTTPStatus int
}

// 通用错误码。
var (
	CodeSuccess          = ErrorCode{Code: 0, Message: "success", HTTPStatus: http.StatusOK}
	CodeInvalidParam     = ErrorCode{Code: 40001, Message: "请求参数不合法", HTTPStatus: http.StatusBadRequest}
	CodeUnauthorized     = ErrorCode{Code: 40101, Message: "未登录或登录已过期", HTTPStatus: http.StatusUnauthorized}
	CodeForbidden        = ErrorCode{Code: 40301, Message: "没有操作权限", HTTPStatus: http.StatusForbidden}
	CodeNotFound         = ErrorCode{Code: 40401, Message: "视频不存在", HTTPStatus: http.StatusNotFound}
	CodeCategoryNotFound = ErrorCode{Code: 40402, Message: "分类不存在", HTTPStatus: http.StatusNotFound}
	CodeTaskNotFound     = ErrorCode{Code: 40403, Message: "转码任务不存在", HTTPStatus: http.StatusNotFound}
	CodeConflict         = ErrorCode{Code: 40901, Message: "资源冲突", HTTPStatus: http.StatusConflict}
	CodeTooManyRequests  = ErrorCode{Code: 42901, Message: "请求过于频繁", HTTPStatus: http.StatusTooManyRequests}
	CodeInternal         = ErrorCode{Code: 50000, Message: "内部错误", HTTPStatus: http.StatusInternalServerError}
	CodeDependencyDown   = ErrorCode{Code: 50001, Message: "下游依赖不可用", HTTPStatus: http.StatusServiceUnavailable}
	CodeUploadNotFound   = ErrorCode{Code: 40404, Message: "上传会话不存在或已失效", HTTPStatus: http.StatusNotFound}
)

// video 域专用错误码。
var (
	CodeUnsupportedFileType = ErrorCode{Code: 60001, Message: "文件类型不支持", HTTPStatus: http.StatusBadRequest}
	CodeFileTooLarge        = ErrorCode{Code: 60002, Message: "文件超出大小限制", HTTPStatus: http.StatusRequestEntityTooLarge}
	CodeVideoStatusConflict = ErrorCode{Code: 60003, Message: "视频状态不允许该操作", HTTPStatus: http.StatusConflict}
	CodeTranscodeSubmitFail = ErrorCode{Code: 60004, Message: "转码任务提交失败", HTTPStatus: http.StatusServiceUnavailable}
	CodeUploadPartInvalid   = ErrorCode{Code: 60005, Message: "上传分片不合法", HTTPStatus: http.StatusBadRequest}
	CodeUploadIncomplete    = ErrorCode{Code: 60006, Message: "分片尚未全部上传", HTTPStatus: http.StatusConflict}
)

// BizError 是可对外暴露的业务错误。
// Detail 用于承载「更具体的说明」，为空时回退到 ErrorCode.Message。
type BizError struct {
	Code   ErrorCode
	Detail string
}

// Error 实现 error 接口。
func (e *BizError) Error() string {
	if e.Detail == "" {
		return e.Code.Message
	}
	return fmt.Sprintf("%s: %s", e.Code.Message, e.Detail)
}

// Message 返回对外可见的 message：优先具体说明，其次默认提示。
func (e *BizError) Message() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Code.Message
}

// New 用具体说明构造业务错误。
func (c ErrorCode) New(detail string) *BizError {
	return &BizError{Code: c, Detail: detail}
}

// Newf 用格式化说明构造业务错误。
func (c ErrorCode) Newf(format string, args ...any) *BizError {
	return &BizError{Code: c, Detail: fmt.Sprintf(format, args...)}
}

// Plain 构造不带具体说明的业务错误。
func (c ErrorCode) Plain() *BizError {
	return &BizError{Code: c}
}

// AsBizError 尝试把任意 error 归一化成 *BizError。
func AsBizError(err error) (*BizError, bool) {
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

// Is 判断 err 是否属于指定错误码（用于上层分支处理）。
//
// 比较整个 ErrorCode 而不是只比 Code：有的码刻意复用对外 code（例如「撤回超窗」
// 沿用 40001 但 HTTP 是 409），只比 Code 会把两者混为一谈。
func Is(err error, code ErrorCode) bool {
	be, ok := AsBizError(err)
	return ok && be.Code == code
}

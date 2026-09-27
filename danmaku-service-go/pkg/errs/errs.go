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
	CodeSuccess         = ErrorCode{Code: 0, Message: "success", HTTPStatus: http.StatusOK}
	CodeInvalidParam    = ErrorCode{Code: 40001, Message: "请求参数不合法", HTTPStatus: http.StatusBadRequest}
	CodeUnauthorized    = ErrorCode{Code: 40101, Message: "未登录或登录已过期", HTTPStatus: http.StatusUnauthorized}
	CodeForbidden       = ErrorCode{Code: 40301, Message: "没有操作权限", HTTPStatus: http.StatusForbidden}
	CodeConflict        = ErrorCode{Code: 40901, Message: "资源冲突", HTTPStatus: http.StatusConflict}
	CodeTooManyRequests = ErrorCode{Code: 42901, Message: "请求过于频繁", HTTPStatus: http.StatusTooManyRequests}
	CodeInternal        = ErrorCode{Code: 50000, Message: "内部错误", HTTPStatus: http.StatusInternalServerError}
	CodeDependencyDown  = ErrorCode{Code: 50001, Message: "下游依赖不可用", HTTPStatus: http.StatusServiceUnavailable}
)

// danmaku 域专用错误码（编号段 8xxxx，与错误码总表保持一致）。
var (
	// CodeDanmakuNotFound 撤回时目标弹幕不存在。
	CodeDanmakuNotFound = ErrorCode{Code: 40401, Message: "弹幕不存在", HTTPStatus: http.StatusNotFound}
	// CodeContentRejected 命中敏感词 REJECT 策略。不入库、不广播。
	CodeContentRejected = ErrorCode{Code: 80001, Message: "弹幕包含违规内容", HTTPStatus: http.StatusBadRequest}
	// CodeRateLimited 触发用户级 / 房间级限流，data 中带 retryAfterMs。
	CodeRateLimited = ErrorCode{Code: 80002, Message: "发送过于频繁", HTTPStatus: http.StatusTooManyRequests}
	// CodeRoomNotLiving 向未开播 / 已结束的直播间发送弹幕。
	CodeRoomNotLiving = ErrorCode{Code: 80003, Message: "直播间当前未开播", HTTPStatus: http.StatusConflict}
	// CodeRecallWindowExpired 超过撤回时间窗。
	//
	// 注意：对外 code 沿用 40001（见《接口文档》§3.3），但 HTTP 状态是 409
	// ——前端按 code 分支，运维按 HTTP 状态排查，两者都符合既有契约。
	CodeRecallWindowExpired = ErrorCode{Code: 40001, Message: "超过可撤回时间窗", HTTPStatus: http.StatusConflict}
)

// BizError 是可对外暴露的业务错误。
// Detail 用于承载「更具体的说明」，为空时回退到 ErrorCode.Message；
// Data 用于少数需要额外字段的失败响应（如限流带 retryAfterMs）。
type BizError struct {
	Code   ErrorCode
	Detail string
	Data   any
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

// WithData 构造带附加数据的业务错误（如限流的 retryAfterMs）。
func (c ErrorCode) WithData(detail string, data any) *BizError {
	return &BizError{Code: c, Detail: detail, Data: data}
}

// ExtraData 返回失败响应需要携带的附加数据（无则为 nil）。
func (e *BizError) ExtraData() any { return e.Data }

// AsBizError 尝试把任意 error 归一化成 *BizError。
func AsBizError(err error) (*BizError, bool) {
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

// Is 判断 err 是否属于指定错误码（用于上层分支处理）。
func Is(err error, code ErrorCode) bool {
	be, ok := AsBizError(err)
	return ok && be.Code.Code == code.Code
}

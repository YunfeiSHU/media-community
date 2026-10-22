// Package handler 是 HTTP / WebSocket 出入口：只做参数绑定、取身份、调 service、渲染响应。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/example/danmaku-service-go/api/dto"
	"github.com/example/danmaku-service-go/pkg/errs"
	"github.com/gin-gonic/gin"
)

// bindJSON 绑定 JSON 请求体，并把绑定错误翻译成 40001 + 字段级提示。
func bindJSON(c *gin.Context, target any) error {
	if err := c.ShouldBindJSON(target); err != nil {
		return errs.CodeInvalidParam.New(describeBindError(err))
	}
	return nil
}

func describeBindError(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		if typeErr.Field != "" {
			return fmt.Sprintf("字段 %s 类型不正确", typeErr.Field)
		}
		return "请求体字段类型不正确"
	}
	if errors.Is(err, io.EOF) {
		return "请求体不能为空"
	}
	return "请求体格式不正确"
}

// pathUint 解析路径中的无符号整型参数。
func pathUint(c *gin.Context, name string) (uint64, error) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errs.CodeInvalidParam.Newf("路径参数 %s 必须是正整数", name)
	}
	return value, nil
}

// pathParam 解析路径中的非空字符串参数（roomId / videoId）。
func pathParam(c *gin.Context, name string) (string, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return "", errs.CodeInvalidParam.Newf("%s 不能为空", name)
	}
	return value, nil
}

// parseRoomHistoryQuery 解析直播历史查询参数。
func parseRoomHistoryQuery(c *gin.Context) (dto.RoomHistoryQuery, error) {
	query := dto.RoomHistoryQuery{Source: c.Query("source")}

	page, err := queryInt(c, "page", 1)
	if err != nil {
		return query, err
	}
	pageSize, err := queryInt(c, "pageSize", 50)
	if err != nil {
		return query, err
	}
	query.Page, query.PageSize = page, pageSize
	return query, nil
}

// parseVideoHistoryQuery 解析点播历史查询参数。
func parseVideoHistoryQuery(c *gin.Context) (dto.VideoHistoryQuery, error) {
	fromMs, err := queryInt(c, "fromMs", 0)
	if err != nil {
		return dto.VideoHistoryQuery{}, err
	}
	toMs, err := queryInt(c, "toMs", 0)
	if err != nil {
		return dto.VideoHistoryQuery{}, err
	}
	limit, err := queryInt(c, "limit", 2000)
	if err != nil {
		return dto.VideoHistoryQuery{}, err
	}
	return dto.VideoHistoryQuery{FromMs: fromMs, ToMs: toMs, Limit: limit}, nil
}

func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errs.CodeInvalidParam.Newf("%s 必须是整数", name)
	}
	return value, nil
}

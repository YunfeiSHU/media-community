// Package handler 是 HTTP 出入口：只做参数绑定、取身份、调 service、渲染响应。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/example/video-service-go/api/dto"
	"github.com/example/video-service-go/pkg/errs"
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

// pathID 解析路径参数 :id。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errs.CodeInvalidParam.New("路径参数 id 必须是正整数")
	}
	return id, nil
}

// parseListQuery 解析视频列表查询参数。
func parseListQuery(c *gin.Context) (dto.ListQuery, error) {
	q := dto.ListQuery{
		Keyword: c.Query("keyword"),
		OwnerID: strings.TrimSpace(c.Query("ownerId")),
		Status:  strings.TrimSpace(c.Query("status")),
	}

	page, err := queryInt(c, "page", 1)
	if err != nil {
		return q, err
	}
	pageSize, err := queryInt(c, "pageSize", 20)
	if err != nil {
		return q, err
	}
	q.Page, q.PageSize = page, pageSize

	if raw := c.Query("categoryId"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return q, errs.CodeInvalidParam.New("categoryId 必须是正整数")
		}
		q.CategoryID = &id
	}

	if raw := c.Query("mine"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return q, errs.CodeInvalidParam.New("mine 必须是布尔值")
		}
		q.Mine = value
	}

	if raw := c.Query("favorite"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return q, errs.CodeInvalidParam.New("favorite 必须是布尔值")
		}
		q.Favorite = value
	}

	return q, nil
}

func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errs.CodeInvalidParam.Newf("%s 必须是整数", name)
	}
	return value, nil
}

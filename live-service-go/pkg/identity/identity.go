// Package identity 处理「谁在调用」。
//
// 与工作区 Java 体系完全一致：
//   - uid 唯一来源：请求头 X-User-Id（网关从 JWT sub 注入）
//   - 角色唯一来源：请求头 X-User-Role（ADMIN / user，缺失按 user）
//   - 本服务不解析 JWT，也不接受请求体 / 查询参数里的用户标识
package identity

import (
	"github.com/example/live-service-go/pkg/errs"
	"github.com/gin-gonic/gin"
)

// 身份相关请求头与角色常量。
const (
	HeaderUserID   = "X-User-Id"
	HeaderUserRole = "X-User-Role"

	RoleAdmin = "ADMIN"
	RoleUser  = "user"
)

// CurrentUser 是当前调用者。
type CurrentUser struct {
	UID  string
	Role string
}

// From 从请求头解析当前调用者（不做校验）。
func From(c *gin.Context) CurrentUser {
	role := c.GetHeader(HeaderUserRole)
	if role == "" {
		role = RoleUser
	}
	return CurrentUser{UID: c.GetHeader(HeaderUserID), Role: role}
}

// Authenticated 表示是否已登录。
func (u CurrentUser) Authenticated() bool { return u.UID != "" }

// IsAdmin 表示是否为管理员。
func (u CurrentUser) IsAdmin() bool { return u.Role == RoleAdmin }

// Owns 判断当前用户是否为资源所有者。
func (u CurrentUser) Owns(ownerID string) bool {
	return u.UID != "" && u.UID == ownerID
}

// Require 要求已登录，否则返回 401/40101。
func Require(c *gin.Context) (CurrentUser, error) {
	u := From(c)
	if !u.Authenticated() {
		return u, errs.CodeUnauthorized.Plain()
	}
	return u, nil
}

// RequireAdmin 要求管理员，否则返回 401/40101 或 403/40301。
func RequireAdmin(c *gin.Context) (CurrentUser, error) {
	u, err := Require(c)
	if err != nil {
		return u, err
	}
	if !u.IsAdmin() {
		return u, errs.CodeForbidden.New("需要管理员权限")
	}
	return u, nil
}

// RequireOwner 要求当前用户是资源所有者。
//
// 注意：ADMIN **不能**绕过归属校验 —— 归属是数据所有权，角色是治理权限。
// 管理员可执行的动作（如强制下架）由业务层单独判断。
func (u CurrentUser) RequireOwner(ownerID string) error {
	if u.Owns(ownerID) {
		return nil
	}
	if !u.Authenticated() {
		return errs.CodeUnauthorized.Plain()
	}
	return errs.CodeForbidden.New("只能操作自己创建的资源")
}

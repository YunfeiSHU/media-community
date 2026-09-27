package handler

import (
	"github.com/example/danmaku-service-go/pkg/identity"
	"github.com/gin-gonic/gin"
)

// currentUser 不做校验地取当前调用者（可选登录的接口用它）。
func currentUser(c *gin.Context) identity.CurrentUser { return identity.From(c) }

// requireUser 要求已登录，否则返回 401/40101。
func requireUser(c *gin.Context) (identity.CurrentUser, error) { return identity.Require(c) }

// requireAdmin 要求管理员，否则返回 401/40101 或 403/40301。
func requireAdmin(c *gin.Context) (identity.CurrentUser, error) { return identity.RequireAdmin(c) }

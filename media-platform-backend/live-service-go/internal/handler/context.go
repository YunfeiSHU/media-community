package handler

import (
	"github.com/example/live-service-go/pkg/identity"
	"github.com/gin-gonic/gin"
)

// currentUser 读取当前调用者（不做校验，用于「登录可有可无」的读接口）。
func currentUser(c *gin.Context) identity.CurrentUser {
	return identity.From(c)
}

// requireUser 要求已登录（缺 X-User-Id → 401/40101）。
//
// 写接口虽然挂在 middleware.Auth() 之后，但 service 层依赖 user.UID，
// 这里再取一次可避免「中间件顺序被改动」导致 uid 为空却继续执行的隐患。
func requireUser(c *gin.Context) (identity.CurrentUser, error) {
	return identity.Require(c)
}

// Package middleware — 角色校验中间件。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole 返回一个中间件，校验当前用户角色是否在允许列表中。
// 角色从 gin.Context 的 "role" 键获取（由上游 auth 中间件设置）。
// P2-2: 用于保护敏感管理 API，如 DB Config。
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"detail": "Access denied: role information missing",
			})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"detail": "Access denied: invalid role format",
			})
			c.Abort()
			return
		}

		if _, allowed := roleSet[roleStr]; !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"detail": "Access denied: insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

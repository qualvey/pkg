package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/response"
)

// GinMiddleware 提供 Gin 路由组鉴权中间件
func (m *Manager[T]) GinMiddleware(optional bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ExtractTokenFromHeader(c.GetHeader("Authorization"))
		if tokenStr == "" {
			if optional {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Response{
				Code:    http.StatusUnauthorized,
				Message: "未提供 Token 认证",
				Error:   "MISSING_TOKEN",
			})
			return
		}

		user, claims, err := m.Parse(tokenStr)
		if err != nil {
			if optional {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Response{
				Code:    http.StatusUnauthorized,
				Message: "Token 已失效或无效",
				Error:   "INVALID_TOKEN",
			})
			return
		}

		// 同时注入标准 request.Context 与 gin.Keys，兼顾两套读取习惯
		ctx := WithUser(c.Request.Context(), user)
		ctx = WithClaims(ctx, claims)
		c.Request = c.Request.WithContext(ctx)

		c.Set("claims", claims)
		c.Set("user", user)
		c.Next()
	}
}

// GinCurrentUser 优先从 gin.Keys 读取，不存在时从 Context 读取
func GinCurrentUser[T any](c *gin.Context) (T, bool) {
	if val, exists := c.Get("user"); exists {
		if user, ok := val.(T); ok {
			return user, true
		}
	}
	if c.Request != nil {
		return CurrentUser[T](c.Request.Context())
	}
	var zero T
	return zero, false
}

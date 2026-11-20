package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/logger"
	"github.com/qualvey/pkg/response"
)

// Recovery 标准库 HTTP Panic 恢复中间件
func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					logger.ErrorContext(r.Context(), "🔥 HTTP panic recovered: %v\n%s", rec, string(stack))
					_ = response.WriteError(w, r.Context(), fmt.Errorf("internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// GinRecovery Gin 框架 Panic 恢复中间件
func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				logger.ErrorContext(c.Request.Context(), "🔥 HTTP panic recovered: %v\n%s", rec, string(stack))

				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
					Code:    http.StatusInternalServerError,
					Message: "服务器内部异常",
					Error:   "INTERNAL_PANIC_RECOVERED",
				})
			}
		}()
		c.Next()
	}
}

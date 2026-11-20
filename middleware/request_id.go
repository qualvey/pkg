package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/logger"
)

type requestIDKey struct{}

const HeaderXRequestID = "X-Request-ID"

// WithRequestID 将 requestID 写入 Context
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext 从 Context 读取 requestID
func RequestIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey{}).(string); ok {
		return val
	}
	return ""
}

// GenerateRequestID 生成 16 字节随机十六进制 RequestID（快速且不依赖外部 uuid 库）
func GenerateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestID 标准库 HTTP 中间件：自动获取或生成 RequestID 并注入 Context 与 Logger
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderXRequestID)
			if rid == "" {
				rid = GenerateRequestID()
			}

			w.Header().Set(HeaderXRequestID, rid)

			ctx := WithRequestID(r.Context(), rid)
			// 同时自动为 logger 注入 trace_id 属性
			ctx = logger.WithAttrs(ctx, "trace_id", rid)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GinRequestID Gin 框架对应的 RequestID 中间件
func GinRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderXRequestID)
		if rid == "" {
			rid = GenerateRequestID()
		}

		c.Header(HeaderXRequestID, rid)
		c.Set(HeaderXRequestID, rid)
		c.Set("request_id", rid)

		ctx := WithRequestID(c.Request.Context(), rid)
		ctx = logger.WithAttrs(ctx, "trace_id", rid)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

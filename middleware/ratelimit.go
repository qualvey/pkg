package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/response"
)

// Limiter 限流器抽象接口
type Limiter interface {
	Allow(key string) bool
}

// ------------------------------------------------------------
// 纯标准库并发安全内存令牌桶限流器 (Memory Token Bucket)
// ------------------------------------------------------------

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// MemoryLimiter 单机内存令牌桶限流器
type MemoryLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64       // 每秒补充的令牌数
	capacity float64       // 令牌桶容量（单窗口最大突发请求数）
	ttl      time.Duration // 空闲多久清理记录
	stopCh   chan struct{}
}

// NewMemoryLimiter 创建内存令牌桶限流器
// maxRequests: 时间窗口内的最大请求数
// window: 速率窗口大小，如 1 分钟、1 秒
func NewMemoryLimiter(maxRequests int, window time.Duration) *MemoryLimiter {
	limiter := &MemoryLimiter{
		visitors: make(map[string]*visitor),
		capacity: float64(maxRequests),
		rate:     float64(maxRequests) / window.Seconds(),
		ttl:      window * 2,
		stopCh:   make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(window * 3)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				limiter.cleanup()
			case <-limiter.stopCh:
				return
			}
		}
	}()

	return limiter
}

// Close 关闭后台清理协程
func (l *MemoryLimiter) Close() {
	select {
	case <-l.stopCh:
	default:
		close(l.stopCh)
	}
}

func (l *MemoryLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for key, v := range l.visitors {
		if now.Sub(v.lastSeen) > l.ttl {
			delete(l.visitors, key)
		}
	}
}

// Allow 判定 key 是否允许本次请求
func (l *MemoryLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	v, exists := l.visitors[key]
	if !exists {
		l.visitors[key] = &visitor{
			tokens:   l.capacity - 1,
			lastSeen: now,
		}
		return true
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.lastSeen = now
	v.tokens += elapsed * l.rate
	if v.tokens > l.capacity {
		v.tokens = l.capacity
	}

	if v.tokens >= 1.0 {
		v.tokens -= 1.0
		return true
	}

	return false
}

// ExtractClientIP 提取真实客户端 IP（优先检查 X-Forwarded-For，其次 X-Real-IP，最后 RemoteAddr）
func ExtractClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" && net.ParseIP(xri) != nil {
		return xri
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// RateLimit 标准库 HTTP 限流中间件
func RateLimit(limiter Limiter, keyFunc ...func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := ExtractClientIP(r)
			if len(keyFunc) > 0 && keyFunc[0] != nil {
				key = keyFunc[0](r)
			}

			if !limiter.Allow(key) {
				_ = response.WriteJSON(w, http.StatusTooManyRequests, response.Response{
					Code:    http.StatusTooManyRequests,
					Message: "请求过于频繁，请稍后再试",
					Error:   "RATE_LIMIT_EXCEEDED",
					TraceID: response.TraceIDFromContext(r.Context()),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GinRateLimit Gin 框架限流中间件
func GinRateLimit(limiter Limiter, keyFunc ...func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if len(keyFunc) > 0 && keyFunc[0] != nil {
			key = keyFunc[0](c)
		}

		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Response{
				Code:    http.StatusTooManyRequests,
				Message: "请求过于频繁，请稍后再试",
				Error:   "RATE_LIMIT_EXCEEDED",
			})
			return
		}

		c.Next()
	}
}

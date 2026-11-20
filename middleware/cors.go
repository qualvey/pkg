package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig 跨域配置
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
	AllowLocalhost   bool // 是否在非生产环境下放行 localhost / 127.0.0.1
}

// DefaultCORSConfig 默认推荐安全跨域配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposedHeaders:   []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowLocalhost:   true,
	}
}

func isOriginAllowed(origin string, cfg *CORSConfig) bool {
	if origin == "" {
		return true
	}
	lowerOrigin := strings.ToLower(origin)

	for _, o := range cfg.AllowedOrigins {
		if o == "*" || strings.ToLower(o) == lowerOrigin {
			return true
		}
	}

	if cfg.AllowLocalhost {
		if strings.HasPrefix(lowerOrigin, "http://localhost:") ||
			strings.HasPrefix(lowerOrigin, "http://127.0.0.1:") ||
			lowerOrigin == "http://localhost" ||
			lowerOrigin == "http://127.0.0.1" {
			return true
		}
	}

	return false
}

// CORS 标准库 HTTP CORS 中间件
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(int(cfg.MaxAge.Seconds()))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if isOriginAllowed(origin, &cfg) {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}

				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposed != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposed)
				}
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				if maxAge != "0" {
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GinCORS Gin 框架 CORS 中间件
func GinCORS(cfg CORSConfig) gin.HandlerFunc {
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(int(cfg.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if isOriginAllowed(origin, &cfg) {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}

			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if exposed != "" {
				c.Header("Access-Control-Expose-Headers", exposed)
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", methods)
			c.Header("Access-Control-Allow-Headers", headers)
			if maxAge != "0" {
				c.Header("Access-Control-Max-Age", maxAge)
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

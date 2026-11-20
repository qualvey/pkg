package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMiddleware_RequestID(t *testing.T) {
	r := gin.New()
	r.Use(GinRequestID())
	r.GET("/test-rid", func(c *gin.Context) {
		rid := RequestIDFromContext(c.Request.Context())
		response.Success(c, gin.H{"rid": rid})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test-rid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	headerRid := w.Header().Get(HeaderXRequestID)
	if headerRid == "" {
		t.Errorf("expected X-Request-ID header to be set")
	}

	var resp response.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TraceID != headerRid {
		t.Errorf("expected response TraceID %s, got %s", headerRid, resp.TraceID)
	}
}

func TestMiddleware_CORS(t *testing.T) {
	r := gin.New()
	r.Use(GinCORS(DefaultCORSConfig()))
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 测试 OPTIONS 预检请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/data", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("unexpected allow origin: %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddleware_Recovery(t *testing.T) {
	r := gin.New()
	r.Use(GinRecovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("something went catastrophically wrong!")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on panic recovery, got %d", w.Code)
	}

	var resp response.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 500 || resp.Error != "INTERNAL_PANIC_RECOVERED" {
		t.Errorf("unexpected panic response: %+v", resp)
	}
}

func TestMiddleware_RateLimit(t *testing.T) {
	limiter := NewMemoryLimiter(2, 1*time.Second)
	defer limiter.Close()

	r := gin.New()
	r.Use(GinRateLimit(limiter))
	r.GET("/limited", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 第 1 次与第 2 次放行
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/limited", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d expected 200, got %d", i+1, w.Code)
		}
	}

	// 第 3 次应触发 429 限流
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/limited", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("request 3 expected 429 rate limited, got %d", w3.Code)
	}
}

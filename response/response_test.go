package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResponse_StandardWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := WithTraceID(contextBackground(), "req-12345")

	err := WriteError(w, ctx, ErrNotFound("商品不存在"))
	if err != nil {
		t.Fatalf("WriteError failed: %v", err)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 404 || resp.Message != "商品不存在" || resp.Error != "NOT_FOUND" || resp.TraceID != "req-12345" {
		t.Errorf("unexpected response content: %+v", resp)
	}
}

func TestResponse_GinSuccessAndFail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "trace-999")

	Success(c, gin.H{"id": "prod-1"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if resp.TraceID != "trace-999" {
		t.Errorf("expected traceId trace-999, got %s", resp.TraceID)
	}
}

func TestResponse_GinBizError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	FailErr(c, ErrForbidden("您无权访问该资源"))

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 403 || resp.Error != "FORBIDDEN" {
		t.Errorf("unexpected biz error response: %+v", resp)
	}
}

func TestResponse_GinFallbackError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	FailErr(c, errors.New("unknown db error"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 500 || resp.Error != "INTERNAL_SERVER_ERROR" {
		t.Errorf("unexpected fallback error response: %+v", resp)
	}
}

func contextBackground() context.Context {
	return context.Background()
}

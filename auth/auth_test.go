package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qualvey/pkg/response"
)

type TestUserSession struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuth_SignAndParse(t *testing.T) {
	mgr := New[TestUserSession]("my-secret-key-123", 1*time.Hour)

	user := TestUserSession{ID: "usr-100", Role: "admin"}
	token, err := mgr.Sign(user, "usr-100")
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	parsedUser, claims, err := mgr.Parse(token)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsedUser.ID != "usr-100" || parsedUser.Role != "admin" {
		t.Errorf("parsed user mismatch: %+v", parsedUser)
	}
	if claims.Subject != "usr-100" {
		t.Errorf("expected subject usr-100, got %s", claims.Subject)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	mgr := New[TestUserSession]("my-secret-key-123", 1*time.Hour)

	user := TestUserSession{ID: "usr-expired", Role: "user"}
	token, err := mgr.SignWithTTL(user, -1*time.Minute)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	_, _, err = mgr.Parse(token)
	if err == nil {
		t.Fatalf("expected error for expired token, got nil")
	}
}

func TestAuth_GinMiddleware(t *testing.T) {
	mgr := New[TestUserSession]("test-secret-456", 1*time.Hour)
	token, _ := mgr.Sign(TestUserSession{ID: "u_999", Role: "merchant"}, "u_999")

	r := gin.New()
	r.GET("/protected", mgr.GinMiddleware(false), func(c *gin.Context) {
		u, ok := GinCurrentUser[TestUserSession](c)
		if !ok {
			c.String(http.StatusInternalServerError, "no user in context")
			return
		}
		c.JSON(http.StatusOK, u)
	})

	// 1. 无 Token 拦截测试
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w1.Code)
	}

	// 2. 带有效 Token 成功测试
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w2.Code)
	}

	var user TestUserSession
	_ = json.Unmarshal(w2.Body.Bytes(), &user)
	if user.ID != "u_999" || user.Role != "merchant" {
		t.Errorf("user mismatch: %+v", user)
	}
}

func TestAuth_OptionalMiddleware(t *testing.T) {
	mgr := New[TestUserSession]("test-secret-789", 1*time.Hour)

	r := gin.New()
	r.GET("/optional", mgr.GinMiddleware(true), func(c *gin.Context) {
		_, ok := GinCurrentUser[TestUserSession](c)
		response.Success(c, gin.H{"hasUser": ok})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/optional", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for optional auth, got %d", w.Code)
	}
}

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingToken = errors.New("auth: missing authorization token")
	ErrInvalidToken = errors.New("auth: invalid or expired token")
)

// Claims 泛型 Claims，封装业务数据 T 与标准 RFC 注册声明
type Claims[T any] struct {
	Data T `json:"data"`
	jwt.RegisteredClaims
}

// Manager 泛型 JWT 鉴权管理器
type Manager[T any] struct {
	secret     []byte
	issuer     string
	defaultTTL time.Duration
}

// Option 配置选项
type Option func(*options)

type options struct {
	issuer string
}

// WithIssuer 设置 Issuer 签发者信息
func WithIssuer(issuer string) Option {
	return func(o *options) {
		o.issuer = issuer
	}
}

// New 创建针对类型 T 的 JWT 鉴权管理器
func New[T any](secret string, defaultTTL time.Duration, opts ...Option) *Manager[T] {
	if secret == "" {
		panic("auth: secret must not be empty")
	}
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}

	o := &options{
		issuer: "qualvey-auth",
	}
	for _, opt := range opts {
		opt(o)
	}

	return &Manager[T]{
		secret:     []byte(secret),
		issuer:     o.issuer,
		defaultTTL: defaultTTL,
	}
}

// Sign 签发 Token（使用管理器默认 TTL）
func (m *Manager[T]) Sign(data T, subject ...string) (string, error) {
	return m.SignWithTTL(data, m.defaultTTL, subject...)
}

// SignWithTTL 签发指定过期时间的 Token
func (m *Manager[T]) SignWithTTL(data T, ttl time.Duration, subject ...string) (string, error) {
	now := time.Now()
	sub := ""
	if len(subject) > 0 {
		sub = subject[0]
	}

	claims := &Claims[T]{
		Data: data,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse 解析并校验 Token
func (m *Manager[T]) Parse(tokenString string) (T, *Claims[T], error) {
	var zero T
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return zero, nil, ErrMissingToken
	}

	claims := &Claims[T]{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil || !token.Valid {
		return zero, nil, ErrInvalidToken
	}

	return claims.Data, claims, nil
}

// ExtractTokenFromHeader 从 Authorization: Bearer <token> 请求头中剥离 Token
func ExtractTokenFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

// ------------------------------------------------------------
// Context 存取门面
// ------------------------------------------------------------

type contextUserKey struct{}
type contextClaimsKey struct{}

// WithUser 将用户会话 T 注入 context
func WithUser[T any](ctx context.Context, user T) context.Context {
	return context.WithValue(ctx, contextUserKey{}, user)
}

// CurrentUser 从 Context 提取已登录的用户会话实体 T
func CurrentUser[T any](ctx context.Context) (T, bool) {
	val := ctx.Value(contextUserKey{})
	if val == nil {
		var zero T
		return zero, false
	}
	user, ok := val.(T)
	return user, ok
}

// WithClaims 将完整 Claims[T] 注入 context
func WithClaims[T any](ctx context.Context, claims *Claims[T]) context.Context {
	return context.WithValue(ctx, contextClaimsKey{}, claims)
}

// ClaimsFromContext 从 Context 提取完整 Claims[T]
func ClaimsFromContext[T any](ctx context.Context) (*Claims[T], bool) {
	val := ctx.Value(contextClaimsKey{})
	if val == nil {
		return nil, false
	}
	claims, ok := val.(*Claims[T])
	return claims, ok
}

// HTTPMiddleware 标准库 http.Handler 鉴权中间件
func (m *Manager[T]) HTTPMiddleware(optional bool, onUnauthorized ...func(w http.ResponseWriter, r *http.Request, err error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ExtractTokenFromHeader(r.Header.Get("Authorization"))
			if tokenStr == "" {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				if len(onUnauthorized) > 0 {
					onUnauthorized[0](w, r, ErrMissingToken)
				} else {
					http.Error(w, "Unauthorized: missing bearer token", http.StatusUnauthorized)
				}
				return
			}

			user, claims, err := m.Parse(tokenStr)
			if err != nil {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				if len(onUnauthorized) > 0 {
					onUnauthorized[0](w, r, ErrInvalidToken)
				} else {
					http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				}
				return
			}

			ctx := WithUser(r.Context(), user)
			ctx = WithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

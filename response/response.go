package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Response 统一标准 RESTful API 响应契约
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	TraceID string `json:"traceId,omitempty"`
}

// PageData 统一分页或列表泛型包装
type PageData[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page,omitempty"`
	PageSize int   `json:"pageSize,omitempty"`
	LimitMax int   `json:"limitMax,omitempty"`
}

// BizError 业务错误标准接口
type BizError interface {
	error
	HTTPStatus() int
	Code() int
	Message() string
	ErrorType() string
}

type bizErrorImpl struct {
	httpStatus int
	code       int
	message    string
	errType    string
	cause      error
}

func (e *bizErrorImpl) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s/%d] %s: %v", e.errType, e.code, e.message, e.cause)
	}
	return fmt.Sprintf("[%s/%d] %s", e.errType, e.code, e.message)
}

func (e *bizErrorImpl) HTTPStatus() int {
	return e.httpStatus
}

func (e *bizErrorImpl) Code() int {
	return e.code
}

func (e *bizErrorImpl) Message() string {
	return e.message
}

func (e *bizErrorImpl) ErrorType() string {
	return e.errType
}

func (e *bizErrorImpl) Unwrap() error {
	return e.cause
}

// NewBizError 创建自定义业务错误
func NewBizError(httpStatus, code int, message, errType string, cause ...error) BizError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &bizErrorImpl{
		httpStatus: httpStatus,
		code:       code,
		message:    message,
		errType:    errType,
		cause:      c,
	}
}

// 常见快捷预定义业务错误构造器
func ErrBadRequest(msg string, cause ...error) BizError {
	return NewBizError(http.StatusBadRequest, 400, msg, "BAD_REQUEST", cause...)
}

func ErrUnauthorized(msg string, cause ...error) BizError {
	return NewBizError(http.StatusUnauthorized, 401, msg, "UNAUTHORIZED", cause...)
}

func ErrForbidden(msg string, cause ...error) BizError {
	return NewBizError(http.StatusForbidden, 403, msg, "FORBIDDEN", cause...)
}

func ErrNotFound(msg string, cause ...error) BizError {
	return NewBizError(http.StatusNotFound, 404, msg, "NOT_FOUND", cause...)
}

func ErrConflict(msg string, cause ...error) BizError {
	return NewBizError(http.StatusConflict, 409, msg, "CONFLICT", cause...)
}

func ErrTooManyRequests(msg string, cause ...error) BizError {
	return NewBizError(http.StatusTooManyRequests, 429, msg, "RATE_LIMIT_EXCEEDED", cause...)
}

func ErrInternal(msg string, cause ...error) BizError {
	return NewBizError(http.StatusInternalServerError, 500, msg, "INTERNAL_ERROR", cause...)
}

// ------------------------------------------------------------
// 纯标准库 http.ResponseWriter 渲染器
// ------------------------------------------------------------

type traceIDContextKey struct{}

// WithTraceID 将 TraceID/RequestID 注入 Context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFromContext 从 Context 中提取 TraceID
func TraceIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return val
	}
	return ""
}

// WriteJSON 纯标准库写入 JSON 响应
func WriteJSON(w http.ResponseWriter, httpStatus int, resp Response) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	return json.NewEncoder(w).Encode(resp)
}

// WriteSuccess 纯标准库快速成功返回 (HTTP 200)
func WriteSuccess(w http.ResponseWriter, data any) error {
	return WriteJSON(w, http.StatusOK, Response{
		Code: http.StatusOK,
		Data: data,
	})
}

// WriteError 纯标准库将错误格式化输出
func WriteError(w http.ResponseWriter, ctx context.Context, err error) error {
	var biz BizError
	if errors.As(err, &biz) {
		return WriteJSON(w, biz.HTTPStatus(), Response{
			Code:    biz.Code(),
			Message: biz.Message(),
			Error:   biz.ErrorType(),
			TraceID: TraceIDFromContext(ctx),
		})
	}

	// 兜底未捕获的普通错误
	return WriteJSON(w, http.StatusInternalServerError, Response{
		Code:    500,
		Message: "服务器内部异常",
		Error:   "INTERNAL_SERVER_ERROR",
		TraceID: TraceIDFromContext(ctx),
	})
}

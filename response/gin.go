package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Success 成功返回，默认 code=200
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Data:    data,
		TraceID: extractTraceID(c),
	})
}

// SuccessWithMsg 成功返回附带提示消息
func SuccessWithMsg(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: message,
		Data:    data,
		TraceID: extractTraceID(c),
	})
}

// List 统一列表与分页返回包装（完全兼容 Mall/backend 历史调用习惯）
func List(c *gin.Context, list any, total int, limitMax ...int) {
	limit := 0
	if len(limitMax) > 0 {
		limit = limitMax[0]
	}
	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: gin.H{
			"list":     list,
			"total":    total,
			"limitMax": limit,
		},
		TraceID: extractTraceID(c),
	})
}

// Page 泛型分页返回
func Page[T any](c *gin.Context, list []T, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: PageData[T]{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
		TraceID: extractTraceID(c),
	})
}

// Fail 业务/校验失败返回，支持指定 HTTP 状态码与业务 code
func Fail(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		TraceID: extractTraceID(c),
	})
}

// FailErr 自动识别 BizError 并渲染对应 HTTP 状态码与业务 code
func FailErr(c *gin.Context, err error) {
	traceID := extractTraceID(c)

	var biz BizError
	if errors.As(err, &biz) {
		c.JSON(biz.HTTPStatus(), Response{
			Code:    biz.Code(),
			Message: biz.Message(),
			Error:   biz.ErrorType(),
			TraceID: traceID,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: "服务器内部异常",
		Error:   "INTERNAL_SERVER_ERROR",
		TraceID: traceID,
	})
}

// BadRequest 快速返回 400 错误
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    http.StatusBadRequest,
		Message: message,
		Error:   "BAD_REQUEST",
		TraceID: extractTraceID(c),
	})
}

// Unauthorized 快速返回 401 错误
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    http.StatusUnauthorized,
		Message: message,
		Error:   "UNAUTHORIZED",
		TraceID: extractTraceID(c),
	})
}

// Forbidden 快速返回 403 错误
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    http.StatusForbidden,
		Message: message,
		Error:   "FORBIDDEN",
		TraceID: extractTraceID(c),
	})
}

// NotFound 快速返回 404 错误
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    http.StatusNotFound,
		Message: message,
		Error:   "NOT_FOUND",
		TraceID: extractTraceID(c),
	})
}

// InternalError 快速返回 500 错误
func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    http.StatusInternalServerError,
		Message: message,
		Error:   "INTERNAL_ERROR",
		TraceID: extractTraceID(c),
	})
}

// extractTraceID 尝试从 Gin 上下文或请求上下文提取 RequestID / TraceID
func extractTraceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, exists := c.Get("X-Request-ID"); exists {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, exists := c.Get("request_id"); exists {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if c.Request != nil {
		if reqID := c.Request.Header.Get("X-Request-ID"); reqID != "" {
			return reqID
		}
		return TraceIDFromContext(c.Request.Context())
	}
	return ""
}

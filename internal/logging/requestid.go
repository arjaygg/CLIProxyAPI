package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
)

// requestIDKey is the context key for storing/retrieving request IDs.
type requestIDKey struct{}

// sessionIDKey is the context key for storing/retrieving session IDs.
type sessionIDKey struct{}

// ginRequestIDKey is the Gin context key for request IDs.
const ginRequestIDKey = "__request_id__"

// ginSessionIDKey is the Gin context key for session IDs.
const ginSessionIDKey = "__session_id__"

// ginModelNameKey is the Gin context key for the requested model name.
const GinModelNameKey = "__model_name__"

// GenerateRequestID creates a new 8-character hex request ID.
func GenerateRequestID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}

// WithRequestID returns a new context with the request ID attached.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// GetRequestID retrieves the request ID from the context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// SetGinRequestID stores the request ID in the Gin context.
func SetGinRequestID(c *gin.Context, requestID string) {
	if c != nil {
		c.Set(ginRequestIDKey, requestID)
	}
}

// GetGinRequestID retrieves the request ID from the Gin context.
func GetGinRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id, exists := c.Get(ginRequestIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// WithSessionID returns a new context with the session ID attached.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context.
func GetSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(sessionIDKey{}).(string); ok {
		return id
	}
	return ""
}

// SetGinSessionID stores the session ID in the Gin context.
func SetGinSessionID(c *gin.Context, sessionID string) {
	if c != nil && sessionID != "" {
		c.Set(ginSessionIDKey, sessionID)
	}
}

// GetGinSessionID retrieves the session ID from the Gin context.
func GetGinSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id, exists := c.Get(ginSessionIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// SetGinModelName stores the requested model name in the Gin context for logging.
func SetGinModelName(c *gin.Context, modelName string) {
	if c != nil && modelName != "" {
		c.Set(GinModelNameKey, modelName)
	}
}

// GetGinModelName retrieves the requested model name from the Gin context.
func GetGinModelName(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if m, exists := c.Get(GinModelNameKey); exists {
		if s, ok := m.(string); ok {
			return s
		}
	}
	return ""
}

// ExtractSessionID extracts a session identifier from request headers using a priority chain:
//  1. X-Session-Id header (explicit)
//  2. Idempotency-Key header (used by Claude Code per-request, provides request-level correlation)
//  3. User-Agent version (fallback client identification)
func ExtractSessionID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	if sid := strings.TrimSpace(c.GetHeader("X-Session-Id")); sid != "" {
		return sid
	}

	if ikey := strings.TrimSpace(c.GetHeader("Idempotency-Key")); ikey != "" {
		return ikey
	}

	return ""
}

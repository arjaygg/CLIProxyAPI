package logging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateRequestID_Length(t *testing.T) {
	id := GenerateRequestID()
	if len(id) != 8 {
		t.Errorf("expected 8-char hex ID, got %q (len %d)", id, len(id))
	}
}

func TestGenerateRequestID_Unique(t *testing.T) {
	a := GenerateRequestID()
	b := GenerateRequestID()
	if a == b {
		t.Errorf("two generated IDs should differ, both are %q", a)
	}
}

func TestWithRequestID_AndGetRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "abc12345")
	if got := GetRequestID(ctx); got != "abc12345" {
		t.Errorf("GetRequestID = %q, want %q", got, "abc12345")
	}
}

func TestGetRequestID_NilContext(t *testing.T) {
	if got := GetRequestID(nil); got != "" {
		t.Errorf("GetRequestID(nil) = %q, want empty", got)
	}
}

func TestGetRequestID_MissingKey(t *testing.T) {
	if got := GetRequestID(context.Background()); got != "" {
		t.Errorf("GetRequestID(background) = %q, want empty", got)
	}
}

func TestWithSessionID_AndGetSessionID(t *testing.T) {
	ctx := WithSessionID(context.Background(), "sess-123")
	if got := GetSessionID(ctx); got != "sess-123" {
		t.Errorf("GetSessionID = %q, want %q", got, "sess-123")
	}
}

func TestWithSessionID_EmptyIsNoop(t *testing.T) {
	ctx := context.Background()
	result := WithSessionID(ctx, "")
	if result != ctx {
		t.Error("WithSessionID with empty string should return same context")
	}
}

func TestWithSessionID_WhitespaceOnlyIsNoop(t *testing.T) {
	ctx := context.Background()
	result := WithSessionID(ctx, "   ")
	if result != ctx {
		t.Error("WithSessionID with whitespace-only should return same context")
	}
}

func TestGetSessionID_NilContext(t *testing.T) {
	if got := GetSessionID(nil); got != "" {
		t.Errorf("GetSessionID(nil) = %q, want empty", got)
	}
}

func newGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c
}

func TestSetGetGinRequestID(t *testing.T) {
	c := newGinContext()
	SetGinRequestID(c, "req-abc")
	if got := GetGinRequestID(c); got != "req-abc" {
		t.Errorf("GetGinRequestID = %q, want %q", got, "req-abc")
	}
}

func TestGetGinRequestID_NilContext(t *testing.T) {
	if got := GetGinRequestID(nil); got != "" {
		t.Errorf("GetGinRequestID(nil) = %q, want empty", got)
	}
}

func TestSetGetGinSessionID(t *testing.T) {
	c := newGinContext()
	SetGinSessionID(c, "sess-xyz")
	if got := GetGinSessionID(c); got != "sess-xyz" {
		t.Errorf("GetGinSessionID = %q, want %q", got, "sess-xyz")
	}
}

func TestSetGinSessionID_EmptyDoesNotStore(t *testing.T) {
	c := newGinContext()
	SetGinSessionID(c, "")
	if got := GetGinSessionID(c); got != "" {
		t.Errorf("GetGinSessionID after empty set = %q, want empty", got)
	}
}

func TestGetGinSessionID_NilContext(t *testing.T) {
	if got := GetGinSessionID(nil); got != "" {
		t.Errorf("GetGinSessionID(nil) = %q, want empty", got)
	}
}

func TestSetGetGinModelName(t *testing.T) {
	c := newGinContext()
	SetGinModelName(c, "claude-opus-4-6")
	if got := GetGinModelName(c); got != "claude-opus-4-6" {
		t.Errorf("GetGinModelName = %q, want %q", got, "claude-opus-4-6")
	}
}

func TestSetGinModelName_EmptyDoesNotStore(t *testing.T) {
	c := newGinContext()
	SetGinModelName(c, "")
	if got := GetGinModelName(c); got != "" {
		t.Errorf("GetGinModelName after empty set = %q, want empty", got)
	}
}

func TestGetGinModelName_NilContext(t *testing.T) {
	if got := GetGinModelName(nil); got != "" {
		t.Errorf("GetGinModelName(nil) = %q, want empty", got)
	}
}

func TestExtractSessionID_XSessionIdHeader(t *testing.T) {
	c := newGinContext()
	c.Request.Header.Set("X-Session-Id", "95b05a48-5562-4102-aee3-d5c80f240893")
	got := ExtractSessionID(c)
	if got != "95b05a48-5562-4102-aee3-d5c80f240893" {
		t.Errorf("ExtractSessionID = %q, want X-Session-Id value", got)
	}
}

func TestExtractSessionID_IdempotencyKeyFallback(t *testing.T) {
	c := newGinContext()
	c.Request.Header.Set("Idempotency-Key", "idem-key-abc")
	got := ExtractSessionID(c)
	if got != "idem-key-abc" {
		t.Errorf("ExtractSessionID = %q, want Idempotency-Key value", got)
	}
}

func TestExtractSessionID_XSessionIdTakesPrecedence(t *testing.T) {
	c := newGinContext()
	c.Request.Header.Set("X-Session-Id", "session-from-header")
	c.Request.Header.Set("Idempotency-Key", "idem-key-abc")
	got := ExtractSessionID(c)
	if got != "session-from-header" {
		t.Errorf("ExtractSessionID = %q, want X-Session-Id (higher priority)", got)
	}
}

func TestExtractSessionID_NoHeaders_ReturnsEmpty(t *testing.T) {
	c := newGinContext()
	got := ExtractSessionID(c)
	if got != "" {
		t.Errorf("ExtractSessionID = %q, want empty when no headers set", got)
	}
}

func TestExtractSessionID_NilContext(t *testing.T) {
	got := ExtractSessionID(nil)
	if got != "" {
		t.Errorf("ExtractSessionID(nil) = %q, want empty", got)
	}
}

func TestExtractSessionID_WhitespaceOnlyHeader(t *testing.T) {
	c := newGinContext()
	c.Request.Header.Set("X-Session-Id", "   ")
	got := ExtractSessionID(c)
	if got != "" {
		t.Errorf("ExtractSessionID = %q, want empty for whitespace-only header", got)
	}
}

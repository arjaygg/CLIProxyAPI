package logging

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func TestGinLogrusRecoveryRepanicsErrAbortHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusRecovery())
	engine.GET("/abort", func(c *gin.Context) {
		panic(http.ErrAbortHandler)
	})

	req := httptest.NewRequest(http.MethodGet, "/abort", nil)
	recorder := httptest.NewRecorder()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic, got nil")
		}
		err, ok := recovered.(error)
		if !ok {
			t.Fatalf("expected error panic, got %T", recovered)
		}
		if !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected ErrAbortHandler, got %v", err)
		}
		if err != http.ErrAbortHandler {
			t.Fatalf("expected exact ErrAbortHandler sentinel, got %v", err)
		}
	}()

	engine.ServeHTTP(recorder, req)
}

func TestGinLogrusRecoveryHandlesRegularPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusRecovery())
	engine.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestTruncateUA_WithComment(t *testing.T) {
	got := truncateUA("claude-cli/2.1.85 (external, cli)")
	if got != "claude-cli/2.1.85" {
		t.Errorf("truncateUA = %q, want %q", got, "claude-cli/2.1.85")
	}
}

func TestTruncateUA_NoComment(t *testing.T) {
	got := truncateUA("curl/8.4.0")
	if got != "curl/8.4.0" {
		t.Errorf("truncateUA = %q, want %q", got, "curl/8.4.0")
	}
}

func TestTruncateUA_Empty(t *testing.T) {
	got := truncateUA("")
	if got != "" {
		t.Errorf("truncateUA = %q, want empty", got)
	}
}

func TestIsAIAPIPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/messages", true},
		{"/v1/messages/count_tokens", true},
		{"/v1/chat/completions", true},
		{"/v1/completions", true},
		{"/v1/responses", true},
		{"/v1beta/models/gemini-pro:generateContent", true},
		{"/api/provider/anthropic/v1/messages", true},
		{"/v1/models", false},
		{"/", false},
		{"/health", false},
	}
	for _, tt := range tests {
		got := isAIAPIPath(tt.path)
		if got != tt.want {
			t.Errorf("isAIAPIPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func captureLogOutput(fn func()) string {
	var buf bytes.Buffer
	origOut := log.StandardLogger().Out
	origFmt := log.StandardLogger().Formatter
	log.SetOutput(&buf)
	log.SetFormatter(&LogFormatter{})
	log.SetLevel(log.DebugLevel)
	defer func() {
		log.SetOutput(origOut)
		log.SetFormatter(origFmt)
	}()
	fn()
	return buf.String()
}

func TestGinLogrusLogger_AIPath_SetsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedRequestID string
	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/messages", func(c *gin.Context) {
		capturedRequestID = GetGinRequestID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if capturedRequestID == "" {
		t.Error("expected request ID to be set for AI API path")
	}
	if len(capturedRequestID) != 8 {
		t.Errorf("expected 8-char request ID, got %q", capturedRequestID)
	}
}

func TestGinLogrusLogger_AIPath_ExtractsSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedSessionID string
	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/messages", func(c *gin.Context) {
		capturedSessionID = GetGinSessionID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("X-Session-Id", "test-session-abc")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if capturedSessionID != "test-session-abc" {
		t.Errorf("expected session ID %q, got %q", "test-session-abc", capturedSessionID)
	}
}

func TestGinLogrusLogger_NonAIPath_NoRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedRequestID string
	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.GET("/v1/models", func(c *gin.Context) {
		capturedRequestID = GetGinRequestID(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if capturedRequestID != "" {
		t.Errorf("expected no request ID for non-AI path, got %q", capturedRequestID)
	}
}

func TestGinLogrusLogger_LogLineContainsModelAndUA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/messages", func(c *gin.Context) {
		SetGinModelName(c, "claude-4.6-opus-high")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("User-Agent", "claude-cli/2.1.85 (external, cli)")
	recorder := httptest.NewRecorder()

	output := captureLogOutput(func() {
		engine.ServeHTTP(recorder, req)
	})

	if !strings.Contains(output, "model=claude-4.6-opus-high") {
		t.Errorf("log output should contain model field, got: %s", output)
	}
	if !strings.Contains(output, "ua=claude-cli/2.1.85") {
		t.Errorf("log output should contain truncated ua field, got: %s", output)
	}
}

func TestGinLogrusLogger_LogLineContainsSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(GinLogrusLogger())
	engine.POST("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("X-Session-Id", "sess-42")
	recorder := httptest.NewRecorder()

	output := captureLogOutput(func() {
		engine.ServeHTTP(recorder, req)
	})

	if !strings.Contains(output, "session_id=sess-42") {
		t.Errorf("log output should contain session_id field, got: %s", output)
	}
}

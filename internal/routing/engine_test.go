package routing

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func boolPtr(b bool) *bool { return &b }

func makeRequest(messages []map[string]interface{}) []byte {
	body := map[string]interface{}{
		"model":    "claude-opus-4-6",
		"messages": messages,
	}
	data, _ := json.Marshal(body)
	return data
}

func TestAnalyze_BasicSignals(t *testing.T) {
	raw := makeRequest([]map[string]interface{}{
		{"role": "system", "content": "You are helpful"},
		{"role": "user", "content": "Hello world"},
	})
	signals := Analyze(raw, "claude-opus-4-6")

	if signals.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", signals.MessageCount)
	}
	if signals.LastUserMessage != "Hello world" {
		t.Errorf("unexpected last user message: %s", signals.LastUserMessage)
	}
	if signals.SystemPrompt != "You are helpful" {
		t.Errorf("unexpected system prompt: %s", signals.SystemPrompt)
	}
	if signals.HasCodeBlocks {
		t.Error("should not detect code blocks")
	}
	if signals.HasToolBlocks {
		t.Error("should not detect tool blocks")
	}
}

func TestAnalyze_CodeBlocks(t *testing.T) {
	raw := makeRequest([]map[string]interface{}{
		{"role": "user", "content": "Fix this:\n```go\nfunc main() {}\n```"},
	})
	signals := Analyze(raw, "test")
	if !signals.HasCodeBlocks {
		t.Error("should detect code blocks")
	}
}

func TestAnalyze_ToolBlocks(t *testing.T) {
	raw := makeRequest([]map[string]interface{}{
		{"role": "assistant", "content": []map[string]interface{}{
			{"type": "text", "text": "Let me help"},
			{"type": "tool_use", "id": "t1", "name": "read_file"},
		}},
		{"role": "user", "content": []map[string]interface{}{
			{"type": "tool_result", "tool_use_id": "t1", "content": "file contents"},
		}},
	})
	signals := Analyze(raw, "test")
	if !signals.HasToolBlocks {
		t.Error("should detect tool blocks")
	}
}

func TestEngine_DisabledReturnsEmpty(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{Enabled: false})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "opus")
	if result != "" {
		t.Errorf("disabled engine should return empty, got %s", result)
	}
}

func TestEngine_FirstMatchWins(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		Rules: []config.ModelRoutingRule{
			{Name: "short", Priority: 1, TargetModel: "haiku",
				Conditions: []config.ModelRoutingCondition{
					{Type: "last-message-length", Operator: "less-than", Value: "100"},
				}},
			{Name: "always", Priority: 2, TargetModel: "sonnet",
				Conditions: []config.ModelRoutingCondition{
					{Type: "always"},
				}},
		},
	})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "hi"},
	}), "opus")
	if result != "haiku" {
		t.Errorf("expected haiku, got %s", result)
	}
}

func TestEngine_PriorityOrdering(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		Rules: []config.ModelRoutingRule{
			{Name: "low-priority", Priority: 10, TargetModel: "haiku",
				Conditions: []config.ModelRoutingCondition{{Type: "always"}}},
			{Name: "high-priority", Priority: 1, TargetModel: "opus",
				Conditions: []config.ModelRoutingCondition{{Type: "always"}}},
		},
	})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "original")
	if result != "opus" {
		t.Errorf("expected opus (priority 1), got %s", result)
	}
}

func TestEngine_DisabledRule(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		Rules: []config.ModelRoutingRule{
			{Name: "disabled", Priority: 1, TargetModel: "haiku", Enabled: boolPtr(false),
				Conditions: []config.ModelRoutingCondition{{Type: "always"}}},
			{Name: "enabled", Priority: 2, TargetModel: "sonnet",
				Conditions: []config.ModelRoutingCondition{{Type: "always"}}},
		},
	})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "opus")
	if result != "sonnet" {
		t.Errorf("expected sonnet (disabled rule skipped), got %s", result)
	}
}

func TestEngine_DryRunMode(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		DryRun:  true,
		Rules: []config.ModelRoutingRule{
			{Name: "match", Priority: 1, TargetModel: "haiku",
				Conditions: []config.ModelRoutingCondition{{Type: "always"}}},
		},
	})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "opus")
	if result != "" {
		t.Errorf("dry-run should return empty, got %s", result)
	}
}

func TestEngine_DefaultModel(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled:      true,
		DefaultModel: "sonnet",
		Rules: []config.ModelRoutingRule{
			{Name: "no-match", Priority: 1, TargetModel: "haiku",
				Conditions: []config.ModelRoutingCondition{
					{Type: "last-message-length", Operator: "greater-than", Value: "999999"},
				}},
		},
	})
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "short"},
	}), "opus")
	if result != "sonnet" {
		t.Errorf("expected default model sonnet, got %s", result)
	}
}

func TestEngine_ModelFamily(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		Rules: []config.ModelRoutingRule{
			{Name: "claude-only", Priority: 1, TargetModel: "sonnet",
				Conditions: []config.ModelRoutingCondition{
					{Type: "requested-model-family", Operator: "equals", Value: "claude"},
				}},
		},
	})

	// Claude model should match
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "claude-opus-4-6")
	if result != "sonnet" {
		t.Errorf("expected sonnet for claude model, got %s", result)
	}

	// GPT model should not match
	result = e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "test"},
	}), "gpt-5")
	if result != "" {
		t.Errorf("expected empty for gpt model, got %s", result)
	}
}

func TestEngine_MultipleConditions(t *testing.T) {
	e := NewEngine(config.ModelRoutingConfig{
		Enabled: true,
		Rules: []config.ModelRoutingRule{
			{Name: "short-no-code", Priority: 1, TargetModel: "haiku",
				Conditions: []config.ModelRoutingCondition{
					{Type: "last-message-length", Operator: "less-than", Value: "100"},
					{Type: "has-code-blocks", Operator: "equals", Value: "false"},
				}},
		},
	})

	// Short message without code -> matches
	result := e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "hello"},
	}), "opus")
	if result != "haiku" {
		t.Errorf("expected haiku, got %s", result)
	}

	// Short message WITH code -> no match
	result = e.Evaluate(makeRequest([]map[string]interface{}{
		{"role": "user", "content": "fix:\n```\ncode\n```"},
	}), "opus")
	if result != "" {
		t.Errorf("expected empty (code blocks), got %s", result)
	}
}

func makeAnthropicRequest(system interface{}, messages []map[string]interface{}) []byte {
	body := map[string]interface{}{
		"model":    "claude-opus-4-6",
		"system":   system,
		"messages": messages,
	}
	data, _ := json.Marshal(body)
	return data
}

func TestAnalyze_AnthropicSystemString(t *testing.T) {
	raw := makeAnthropicRequest(
		"You are an architect",
		[]map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	)
	signals := Analyze(raw, "test")
	if signals.SystemPrompt != "You are an architect" {
		t.Errorf("expected Anthropic system string, got %q", signals.SystemPrompt)
	}
	if signals.LastUserMessage != "Hello" {
		t.Errorf("expected user message, got %q", signals.LastUserMessage)
	}
}

func TestAnalyze_AnthropicSystemContentBlocks(t *testing.T) {
	raw := makeAnthropicRequest(
		[]map[string]interface{}{
			{"type": "text", "text": "You are an architect"},
			{"type": "text", "text": "designing systems"},
		},
		[]map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	)
	signals := Analyze(raw, "test")
	if signals.SystemPrompt != "You are an architect\ndesigning systems" {
		t.Errorf("expected concatenated system blocks, got %q", signals.SystemPrompt)
	}
}

func TestAnalyze_OpenAISystemMessageStillWorks(t *testing.T) {
	raw := makeRequest([]map[string]interface{}{
		{"role": "system", "content": "You are helpful"},
		{"role": "user", "content": "Hello"},
	})
	signals := Analyze(raw, "test")
	if signals.SystemPrompt != "You are helpful" {
		t.Errorf("expected OpenAI system message, got %q", signals.SystemPrompt)
	}
}

func TestAnalyze_AnthropicSystemTakesPrecedence(t *testing.T) {
	body := map[string]interface{}{
		"model":  "test",
		"system": "Anthropic system",
		"messages": []map[string]interface{}{
			{"role": "system", "content": "OpenAI system"},
			{"role": "user", "content": "Hello"},
		},
	}
	raw, _ := json.Marshal(body)
	signals := Analyze(raw, "test")
	if signals.SystemPrompt != "Anthropic system" {
		t.Errorf("Anthropic system field should take precedence, got %q", signals.SystemPrompt)
	}
}

// makeClaudeCodeRequest builds a request resembling what Claude Code sends to
// the Anthropic Messages API: a rich system prompt (with code examples and
// workspace rules) in the top-level "system" field, tool definitions in "tools",
// and user/assistant messages with structured content blocks.
func makeClaudeCodeRequest(userContent interface{}, extraMessages ...map[string]interface{}) []byte {
	systemBlocks := []map[string]interface{}{
		{"type": "text", "text": "You are an interactive CLI tool that helps users with software engineering tasks."},
		{"type": "text", "text": "When making code changes use the Read tool.\n\n```python\nfor i in range(10):\n    print(i)\n```\n\nFollow system instructions carefully.", "cache_control": map[string]string{"type": "ephemeral"}},
		{"type": "text", "text": "# Testing Standards\n```csharp\n[Fact]\npublic async Task Test() { }\n```\n# SOLID Principles\nPrefer composition.\n# Security Essentials\nNever log sensitive data."},
	}

	messages := make([]map[string]interface{}, 0, 1+len(extraMessages))
	messages = append(messages, extraMessages...)
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": userContent,
	})

	body := map[string]interface{}{
		"model":      "claude-3-7-sonnet-20250219",
		"max_tokens": 8096,
		"system":     systemBlocks,
		"messages":   messages,
		"tools": []map[string]interface{}{
			{"name": "Read", "description": "Read a file", "input_schema": map[string]interface{}{"type": "object"}},
			{"name": "Shell", "description": "Run a command", "input_schema": map[string]interface{}{"type": "object"}},
		},
	}
	data, _ := json.Marshal(body)
	return data
}

// cursorRoutingEngine returns a routing engine configured like cursor.yaml:
//   - tool-use multi-turn → sonnet (gemini-3.1-pro alias)
//   - long conversations  → sonnet
//   - default             → flash  (gemini-3-flash alias)
func cursorRoutingEngine() *Engine {
	return NewEngine(config.ModelRoutingConfig{
		Enabled:      true,
		DefaultModel: "claude-3-5-sonnet-20241022",
		Rules: []config.ModelRoutingRule{
			{
				Name: "Multi-turn Tool Use to Sonnet", Priority: 10, TargetModel: "claude-3-7-sonnet-20250219",
				Conditions: []config.ModelRoutingCondition{
					{Type: "has-tool-blocks", Operator: "equals", Value: "true"},
				},
			},
			{
				Name: "Long Conversation to Sonnet", Priority: 20, TargetModel: "claude-3-7-sonnet-20250219",
				Conditions: []config.ModelRoutingCondition{
					{Type: "message-count", Operator: "greater-than", Value: "4"},
				},
			},
		},
	})
}

func TestAnalyze_ClaudeCodePayload_PlainPrompt(t *testing.T) {
	raw := makeClaudeCodeRequest("Say hello")
	signals := Analyze(raw, "claude-3-7-sonnet-20250219")

	if signals.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", signals.MessageCount)
	}
	if signals.LastUserMessage != "Say hello" {
		t.Errorf("LastUserMessage = %q, want %q", signals.LastUserMessage, "Say hello")
	}
	if signals.HasToolBlocks {
		t.Error("HasToolBlocks should be false for a first-turn prompt")
	}
	if signals.HasCodeBlocks {
		t.Error("HasCodeBlocks should be false (code blocks are in system, not user message)")
	}
	if !strings.Contains(signals.SystemPrompt, "software engineering") {
		t.Errorf("SystemPrompt should contain Claude Code instructions, got %d chars", len(signals.SystemPrompt))
	}
}

func TestAnalyze_ClaudeCodePayload_ContentBlocksUserMessage(t *testing.T) {
	raw := makeClaudeCodeRequest([]map[string]interface{}{
		{"type": "text", "text": "Review:\n```go\nfunc main() {}\n```"},
	})
	signals := Analyze(raw, "test")

	if !signals.HasCodeBlocks {
		t.Error("HasCodeBlocks should be true when user message contains code")
	}
	if signals.HasToolBlocks {
		t.Error("HasToolBlocks should be false (no tool_use/tool_result blocks)")
	}
}

func TestAnalyze_ClaudeCodePayload_MultiTurnWithToolUse(t *testing.T) {
	raw := makeClaudeCodeRequest(
		"Now fix it",
		map[string]interface{}{
			"role":    "user",
			"content": "Read foo.go",
		},
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Reading the file..."},
				{"type": "tool_use", "id": "toolu_abc", "name": "Read", "input": map[string]string{"path": "foo.go"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "toolu_abc", "content": "package main\nfunc main() {}"},
			},
		},
		map[string]interface{}{
			"role":    "assistant",
			"content": "The file looks fine. What would you like changed?",
		},
	)
	signals := Analyze(raw, "test")

	if !signals.HasToolBlocks {
		t.Error("HasToolBlocks should be true for multi-turn with tool_use + tool_result")
	}
	if signals.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5 (4 history + 1 user prompt appended by helper)", signals.MessageCount)
	}
	if signals.LastUserMessage != "Now fix it" {
		t.Errorf("LastUserMessage = %q, want %q", signals.LastUserMessage, "Now fix it")
	}
}

func TestCursorRouting_PlainPrompt_RoutesToDefault(t *testing.T) {
	e := cursorRoutingEngine()
	raw := makeClaudeCodeRequest("Say hello")
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-3-5-sonnet-20241022" {
		t.Errorf("plain prompt should route to default (flash), got %q", result)
	}
}

func TestCursorRouting_ToolUse_RoutesToSonnet(t *testing.T) {
	e := cursorRoutingEngine()
	raw := makeClaudeCodeRequest(
		"Now fix it",
		map[string]interface{}{
			"role":    "user",
			"content": "Read foo.go",
		},
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]string{"path": "foo.go"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "t1", "content": "file contents"},
			},
		},
	)
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-3-7-sonnet-20250219" {
		t.Errorf("tool-use multi-turn should route to sonnet, got %q", result)
	}
}

func TestCursorRouting_LongConversation_RoutesToSonnet(t *testing.T) {
	e := cursorRoutingEngine()
	raw := makeClaudeCodeRequest(
		"One more thing",
		map[string]interface{}{"role": "user", "content": "first"},
		map[string]interface{}{"role": "assistant", "content": "ok"},
		map[string]interface{}{"role": "user", "content": "second"},
		map[string]interface{}{"role": "assistant", "content": "ok"},
	)
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-3-7-sonnet-20250219" {
		t.Errorf("long conversation (>4 messages) should route to sonnet, got %q", result)
	}
}

func TestCursorRouting_ToolUseTakesPriorityOverMessageCount(t *testing.T) {
	e := cursorRoutingEngine()
	raw := makeClaudeCodeRequest(
		"done",
		map[string]interface{}{"role": "user", "content": "read foo"},
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]string{"path": "foo"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "t1", "content": "data"},
			},
		},
		map[string]interface{}{"role": "assistant", "content": "here it is"},
		map[string]interface{}{"role": "user", "content": "thanks"},
		map[string]interface{}{"role": "assistant", "content": "np"},
	)
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	// Both rules match (tool blocks AND >4 messages); tool-use has priority 10 vs 20
	if result != "claude-3-7-sonnet-20250219" {
		t.Errorf("expected sonnet (tool-use priority 10), got %q", result)
	}
}

func TestCursorRouting_SystemPromptCodeBlocks_DoNotAffectRouting(t *testing.T) {
	e := cursorRoutingEngine()
	// Claude Code system prompt contains code blocks, but routing should
	// only consider the user message for has-code-blocks, and the current
	// cursor rules don't use has-code-blocks at all.
	raw := makeClaudeCodeRequest("Simple question")
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-3-5-sonnet-20241022" {
		t.Errorf("system prompt code blocks should not trigger non-default routing, got %q", result)
	}
}

func TestCursorRouting_ShortConversation_NoToolUse_RoutesToDefault(t *testing.T) {
	e := cursorRoutingEngine()
	raw := makeClaudeCodeRequest(
		"What does this do?",
		map[string]interface{}{"role": "user", "content": "Explain Go interfaces"},
		map[string]interface{}{"role": "assistant", "content": "Go interfaces are..."},
	)
	// 3 messages total (2 history + 1 new), no tool blocks → default
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-3-5-sonnet-20241022" {
		t.Errorf("short conversation without tools should route to default, got %q", result)
	}
}

// configYamlRoutingEngine returns a routing engine matching config.yaml's model-routing rules:
//   - Priority 10: Deep Reasoning → Opus 4.6 (last-message-contains architect|design|...)
//   - Priority 20: Agent/Tool Use → Sonnet 4.6 (has-tool-blocks)
//   - Priority 30: Deep Context → Gemini 3.1 Pro (total-message-length > 40000)
//   - Priority 40: Code Generation → Sonnet 4.6 (has-code-blocks)
//   - Default: claude-haiku-4-5-20251001
func configYamlRoutingEngine() *Engine {
	return NewEngine(config.ModelRoutingConfig{
		Enabled:      true,
		DefaultModel: "claude-haiku-4-5-20251001",
		Rules: []config.ModelRoutingRule{
			{
				Name: "Deep Reasoning to Opus 4.6", Priority: 10, TargetModel: "claude-opus-4-6",
				Conditions: []config.ModelRoutingCondition{
					{Type: "last-message-contains", Operator: "matches", Value: "(?i)(architect|design|deeply review|complex|strategy|system)"},
				},
			},
			{
				Name: "Agent/Tool Use to Sonnet 4.6", Priority: 20, TargetModel: "claude-sonnet-4-6",
				Conditions: []config.ModelRoutingCondition{
					{Type: "has-tool-blocks", Operator: "equals", Value: "true"},
				},
			},
			{
				Name: "Deep Context Code Analysis to Gemini 3.1 Pro", Priority: 30, TargetModel: "gemini-3.1-pro",
				Conditions: []config.ModelRoutingCondition{
					{Type: "total-message-length", Operator: "greater-than", Value: "40000"},
				},
			},
			{
				Name: "Code Generation to Sonnet 4.6", Priority: 40, TargetModel: "claude-sonnet-4-6",
				Conditions: []config.ModelRoutingCondition{
					{Type: "has-code-blocks", Operator: "equals", Value: "true"},
				},
			},
		},
	})
}

func TestConfigYaml_DeepReasoning_RoutesToOpus(t *testing.T) {
	e := configYamlRoutingEngine()
	tests := []struct {
		name    string
		message string
	}{
		{"architect keyword", "Please architect a microservices solution"},
		{"design keyword", "Design the database schema"},
		{"deeply review", "Deeply review this pull request"},
		{"complex keyword", "This is a complex distributed system"},
		{"strategy keyword", "What strategy should we use for caching?"},
		{"system keyword", "Build a system for real-time analytics"},
		{"case insensitive", "ARCHITECT the new payment flow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := makeClaudeCodeRequest(tt.message)
			result := e.Evaluate(raw, "claude-sonnet-4-6")
			if result != "claude-opus-4-6" {
				t.Errorf("expected opus for %q, got %q", tt.message, result)
			}
		})
	}
}

func TestConfigYaml_DeepReasoning_NoMatchForOrdinaryPrompt(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequest("Say hello")
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	if result != "claude-haiku-4-5-20251001" {
		t.Errorf("ordinary prompt should route to default (haiku), got %q", result)
	}
}

func TestConfigYaml_ToolUse_RoutesToSonnet(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequest(
		"Now apply the fix",
		map[string]interface{}{
			"role":    "user",
			"content": "Read main.go",
		},
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]string{"path": "main.go"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "t1", "content": "package main"},
			},
		},
	)
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	if result != "claude-sonnet-4-6" {
		t.Errorf("tool-use should route to sonnet, got %q", result)
	}
}

func TestConfigYaml_DeepContext_RoutesToGemini(t *testing.T) {
	e := configYamlRoutingEngine()
	longContent := strings.Repeat("x", 41000)
	raw := makeRequest([]map[string]interface{}{
		{"role": "user", "content": longContent},
	})
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	if result != "gemini-3.1-pro" {
		t.Errorf("deep context (>40k) should route to gemini, got %q", result)
	}
}

func TestConfigYaml_DeepContext_JustUnderThreshold_RoutesToDefault(t *testing.T) {
	e := configYamlRoutingEngine()
	content := strings.Repeat("x", 39999)
	raw := makeRequest([]map[string]interface{}{
		{"role": "user", "content": content},
	})
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	if result != "claude-haiku-4-5-20251001" {
		t.Errorf("under 40k should route to default, got %q", result)
	}
}

func TestConfigYaml_CodeGeneration_RoutesToSonnet(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequest("Fix this:\n```go\nfunc main() {}\n```")
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	if result != "claude-sonnet-4-6" {
		t.Errorf("code blocks should route to sonnet, got %q", result)
	}
}

func TestConfigYaml_DeepReasoningBeatsToolUse(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequest(
		"Architect a new system for this",
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]string{"path": "foo"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "t1", "content": "data"},
			},
		},
	)
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	// Deep Reasoning (priority 10) beats Tool Use (priority 20)
	if result != "claude-opus-4-6" {
		t.Errorf("deep reasoning (priority 10) should beat tool use (priority 20), got %q", result)
	}
}

func TestConfigYaml_ToolUseBeatsCodeBlocks(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequest(
		"Fix this:\n```go\nfunc main() {}\n```",
		map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "id": "t1", "name": "Shell", "input": map[string]string{"cmd": "go build"}},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "tool_result", "tool_use_id": "t1", "content": "ok"},
			},
		},
	)
	result := e.Evaluate(raw, "claude-sonnet-4-6")
	// Tool Use (priority 20) beats Code Generation (priority 40)
	if result != "claude-sonnet-4-6" {
		t.Errorf("tool use (priority 20) should beat code gen (priority 40), got %q", result)
	}
}

func TestStripXMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "say hello", "say hello"},
		{"system-reminder wrapper", "<system-reminder>\nRemember rules.\n</system-reminder>\n\nsay hello", "Remember rules.\n\n\nsay hello"},
		{"user_info block", "<user_info>\nOS: darwin\n</user_info>\nHello", "OS: darwin\n\nHello"},
		{"self-closing tag preserved", "Hello <br/> world", "Hello <br/> world"},
		{"tag with attributes", `<attached_files path="/foo">content</attached_files>`, "content"},
		{"nested tags", "<outer><inner>text</inner></outer>", "text"},
		{"preserves code blocks", "Fix:\n```go\nfunc main() {}\n```", "Fix:\n```go\nfunc main() {}\n```"},
		{"preserves angle brackets in code", "check if a < b && c > d", "check if a < b && c > d"},
		{"empty after stripping", "<system-reminder></system-reminder>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripXMLTags(tt.input)
			if got != tt.expected {
				t.Errorf("stripXMLTags(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func makeClaudeCodeRequestWithReminder(userText string) []byte {
	wrappedContent := "<system-reminder>\nThe assistant should follow workspace rules.\n</system-reminder>\n\n" + userText
	return makeClaudeCodeRequest(wrappedContent)
}

func TestAnalyze_ClaudeCode_SystemReminderStripped(t *testing.T) {
	raw := makeClaudeCodeRequestWithReminder("say hello")
	signals := Analyze(raw, "claude-3-7-sonnet-20250219")

	if strings.Contains(signals.LastUserMessage, "<system-reminder>") {
		t.Error("LastUserMessage should not contain XML tags after stripping")
	}
	if !strings.Contains(signals.LastUserMessage, "say hello") {
		t.Errorf("LastUserMessage should contain the actual user text, got %q", signals.LastUserMessage)
	}
}

func TestConfigYaml_SystemReminder_DoesNotTriggerDeepReasoning(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequestWithReminder("say hello")
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result == "claude-opus-4-6" {
		t.Error("system-reminder tags should not trigger Deep Reasoning rule")
	}
	if result != "claude-haiku-4-5-20251001" {
		t.Errorf("expected default (haiku), got %q", result)
	}
}

func TestConfigYaml_SystemReminder_RealKeywordStillMatches(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequestWithReminder("architect a microservices solution")
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-opus-4-6" {
		t.Errorf("real keyword 'architect' should still trigger Deep Reasoning, got %q", result)
	}
}

func TestConfigYaml_SystemReminder_CodeBlocksStillDetected(t *testing.T) {
	e := configYamlRoutingEngine()
	raw := makeClaudeCodeRequestWithReminder("Fix this:\n```go\nfunc main() {}\n```")
	result := e.Evaluate(raw, "claude-3-7-sonnet-20250219")
	if result != "claude-sonnet-4-6" {
		t.Errorf("code blocks after stripping should route to sonnet, got %q", result)
	}
}

func TestDetectModelFamily(t *testing.T) {
	tests := []struct{ model, expected string }{
		{"claude-opus-4-6", "claude"},
		{"claude-sonnet-4-5", "claude"},
		{"gemini-claude-opus-4-6-thinking", "claude"},
		{"gpt-5", "gpt"},
		{"o1-preview", "gpt"},
		{"o4-mini", "gpt"},
		{"gemini-2.5-flash", "gemini"},
		{"deepseek-v3", "compatible"},
	}
	for _, tt := range tests {
		got := detectModelFamily(tt.model)
		if got != tt.expected {
			t.Errorf("detectModelFamily(%q) = %q, want %q", tt.model, got, tt.expected)
		}
	}
}

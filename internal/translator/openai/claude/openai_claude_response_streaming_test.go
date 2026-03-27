package claude

import (
	"strings"
	"testing"
)

func TestConvertOpenAIStreamingChunkToAnthropic_ToolCallStreaming(t *testing.T) {
	param := &ConvertOpenAIResponseToAnthropicParams{
		ToolCallBlockIndexes: make(map[int]int),
		TextContentBlockIndex: -1,
		ThinkingContentBlockIndex: -1,
		ToolNameMap: map[string]string{"my_tool": "my_tool"},
	}

	// 1. Tool call start
	chunk1 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","function":{"name":"my_tool","arguments":""}}]}}]}`)
	res1 := convertOpenAIStreamingChunkToAnthropic(chunk1, param)
	
	joined1 := strings.Join(res1, "\n")
	if !strings.Contains(joined1, "content_block_start") || !strings.Contains(joined1, "tool_use") {
		t.Errorf("Expected content_block_start for tool_use, got: %v", joined1)
	}

	// 2. Tool call arguments chunk 1
	chunk2 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"foo\":\""}}]}}]}`)
	res2 := convertOpenAIStreamingChunkToAnthropic(chunk2, param)
	joined2 := strings.Join(res2, "\n")
	if !strings.Contains(joined2, "content_block_delta") || !strings.Contains(joined2, "input_json_delta") || !strings.Contains(joined2, "{\\\"foo\\\":\\\"") {
		t.Errorf("Expected immediate input_json_delta with arguments, got: %v", joined2)
	}

	// 3. Tool call arguments chunk 2
	chunk3 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"bar\"}"}}]}}]}`)
	res3 := convertOpenAIStreamingChunkToAnthropic(chunk3, param)
	joined3 := strings.Join(res3, "\n")
	if !strings.Contains(joined3, "content_block_delta") || !strings.Contains(joined3, "bar\\\"}") {
		t.Errorf("Expected immediate input_json_delta with arguments, got: %v", joined3)
	}

	// 4. Finish reason
	chunk4 := []byte(`{"id":"chatcmpl-123","choices":[{"finish_reason":"tool_calls","delta":{}}]}`)
	res4 := convertOpenAIStreamingChunkToAnthropic(chunk4, param)
	joined4 := strings.Join(res4, "\n")
	if !strings.Contains(joined4, "content_block_stop") {
		t.Errorf("Expected content_block_stop, got: %v", joined4)
	}
}

func TestConvertOpenAIStreamingChunkToAnthropic_ThinkingTextTransition(t *testing.T) {
	param := &ConvertOpenAIResponseToAnthropicParams{
		ToolCallBlockIndexes: make(map[int]int),
		TextContentBlockIndex: -1,
		ThinkingContentBlockIndex: -1,
	}

	// 1. Thinking block starts
	chunk1 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"reasoning_content":"think1"}}]}`)
	res1 := convertOpenAIStreamingChunkToAnthropic(chunk1, param)
	joined1 := strings.Join(res1, "\n")
	if !strings.Contains(joined1, "content_block_start") || !strings.Contains(joined1, "thinking") {
		t.Errorf("Expected thinking block start, got: %v", joined1)
	}

	// 2. Text block starts
	chunk2 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"content":"text1"}}]}`)
	res2 := convertOpenAIStreamingChunkToAnthropic(chunk2, param)
	joined2 := strings.Join(res2, "\n")
	if !strings.Contains(joined2, "content_block_stop") || !strings.Contains(joined2, "text_delta") {
		t.Errorf("Expected thinking block to stop and text to start, got: %v", joined2)
	}

	// 3. Interleaved thinking block (Should NOT stop text block and start thinking again)
	chunk3 := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"reasoning_content":"think2"}}]}`)
	res3 := convertOpenAIStreamingChunkToAnthropic(chunk3, param)
	joined3 := strings.Join(res3, "\n")
	if strings.Contains(joined3, "content_block_stop") || strings.Contains(joined3, "thinking_delta") {
		t.Errorf("Should not switch back to thinking after text started, got: %v", joined3)
	}
}

package kids

import (
	"strings"
	"testing"
)

func TestClaudeShapeFilter_ExtractText_CleanText(t *testing.T) {
	body := []byte(`{
		"id": "msg_1",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello there"}],
		"stop_reason": "end_turn"
	}`)
	text, ok := claudeShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if text != "Hello there" {
		t.Errorf("ExtractText() text = %q, want %q", text, "Hello there")
	}
}

func TestClaudeShapeFilter_ExtractText_KnownNonTextResponse(t *testing.T) {
	body := []byte(`{
		"id": "msg_1",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "tool_use", "id": "toolu_1", "name": "f", "input": {}}],
		"stop_reason": "tool_use"
	}`)
	text, ok := claudeShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true (recognised tool_use response)")
	}
	if text != "" {
		t.Errorf("ExtractText() text = %q, want \"\" for pure tool_use response", text)
	}
}

func TestClaudeShapeFilter_ExtractText_Unparseable(t *testing.T) {
	_, ok := claudeShapeFilter{}.ExtractText([]byte(`not json`))
	if ok {
		t.Errorf("ExtractText(garbage) ok = true, want false")
	}

	_, ok = claudeShapeFilter{}.ExtractText([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"x"}}`))
	if ok {
		t.Errorf("ExtractText(non-message type) ok = true, want false")
	}
}

func TestClaudeShapeFilter_ReplaceText(t *testing.T) {
	body := []byte(`{
		"id": "msg_1",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "blocked text"}],
		"stop_reason": "end_turn"
	}`)
	out, err := claudeShapeFilter{}.ReplaceText(body, "fallback text")
	if err != nil {
		t.Fatalf("ReplaceText() error = %v", err)
	}

	text, ok := claudeShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(replaced) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(replaced) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"stop_reason":"end_turn"`) {
		t.Errorf("ReplaceText() output missing stop_reason=end_turn: %s", out)
	}
	if strings.Contains(string(out), "blocked text") {
		t.Errorf("ReplaceText() output still contains blocked text: %s", out)
	}
}

func TestClaudeShapeFilter_ExtractStreamText_CleanStream(t *testing.T) {
	raw := []byte(
		"event: message_start\n" +
			"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\"}}\n\n" +
			"event: content_block_start\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n" +
			"event: content_block_stop\n" +
			"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
			"event: message_delta\n" +
			"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n" +
			"event: message_stop\n" +
			"data: {\"type\":\"message_stop\"}\n\n")

	text, ok := claudeShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true")
	}
	if text != "Hello" {
		t.Errorf("ExtractStreamText() text = %q, want %q", text, "Hello")
	}
}

func TestClaudeShapeFilter_ExtractStreamText_KnownNonTextStream(t *testing.T) {
	raw := []byte(
		"event: message_start\n" +
			"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\"}}\n\n" +
			"event: content_block_start\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"f\"}}\n\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{}\"}}\n\n" +
			"event: content_block_stop\n" +
			"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
			"event: message_delta\n" +
			"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n" +
			"event: message_stop\n" +
			"data: {\"type\":\"message_stop\"}\n\n")

	text, ok := claudeShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true (recognised tool_use stream)")
	}
	if text != "" {
		t.Errorf("ExtractStreamText() text = %q, want \"\" for pure tool_use stream", text)
	}
}

func TestClaudeShapeFilter_ExtractStreamText_Unparseable(t *testing.T) {
	_, ok := claudeShapeFilter{}.ExtractStreamText([]byte("not an SSE stream at all"))
	if ok {
		t.Errorf("ExtractStreamText(garbage) ok = true, want false")
	}

	_, ok = claudeShapeFilter{}.ExtractStreamText([]byte("data: {\"foo\":\"bar\"}\n\n"))
	if ok {
		t.Errorf("ExtractStreamText(no type field) ok = true, want false")
	}
}

func TestClaudeShapeFilter_BuildFallbackStream(t *testing.T) {
	out := claudeShapeFilter{}.BuildFallbackStream("fallback text")

	text, ok := claudeShapeFilter{}.ExtractStreamText(out)
	if !ok {
		t.Fatalf("ExtractStreamText(fallback stream) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractStreamText(fallback stream) text = %q, want %q", text, "fallback text")
	}
	if !strings.HasSuffix(string(out), "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n") {
		t.Errorf("BuildFallbackStream() does not end with message_stop event, got %s", out)
	}
	if !strings.Contains(string(out), `"stop_reason":"end_turn"`) {
		t.Errorf("BuildFallbackStream() missing stop_reason=end_turn: %s", out)
	}
}

func TestClaudeShapeFilter_BuildFallbackBody(t *testing.T) {
	out := claudeShapeFilter{}.BuildFallbackBody("fallback text")

	text, ok := claudeShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(fallback body) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(fallback body) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"stop_reason":"end_turn"`) {
		t.Errorf("BuildFallbackBody() missing stop_reason=end_turn: %s", out)
	}
}

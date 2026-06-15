package kids

import (
	"strings"
	"testing"
)

func TestChatShapeFilter_ExtractText_CleanText(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello there"}, "finish_reason": "stop"}]
	}`)
	text, ok := chatShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if text != "Hello there" {
		t.Errorf("ExtractText() text = %q, want %q", text, "Hello there")
	}
}

func TestChatShapeFilter_ExtractText_KnownNonTextResponse(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": null, "tool_calls": [{"id":"call_1","type":"function","function":{"name":"f","arguments":"{}"}}]}, "finish_reason": "tool_calls"}]
	}`)
	text, ok := chatShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true (recognised tool_calls response)")
	}
	if text != "" {
		t.Errorf("ExtractText() text = %q, want \"\" for pure tool_calls response", text)
	}
}

func TestChatShapeFilter_ExtractText_Unparseable(t *testing.T) {
	_, ok := chatShapeFilter{}.ExtractText([]byte(`not json`))
	if ok {
		t.Errorf("ExtractText(garbage) ok = true, want false")
	}

	_, ok = chatShapeFilter{}.ExtractText([]byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[]}`))
	if ok {
		t.Errorf("ExtractText(no choices) ok = true, want false")
	}
}

// TestChatShapeFilter_ExtractText_MultipleChoicesSecondBlocked covers design
// doc §5.2.1: ExtractText concatenates ALL choices, so a blocklist keyword in
// Choices[1] is reflected in the returned text even though Choices[0] is
// clean.
func TestChatShapeFilter_ExtractText_MultipleChoicesSecondBlocked(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [
			{"index": 0, "message": {"role": "assistant", "content": "Hello there"}, "finish_reason": "stop"},
			{"index": 1, "message": {"role": "assistant", "content": "I will murder you"}, "finish_reason": "stop"}
		]
	}`)
	text, ok := chatShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if !strings.Contains(text, "Hello there") {
		t.Errorf("ExtractText() text = %q, want it to contain Choices[0] text %q", text, "Hello there")
	}
	if !strings.Contains(text, "murder") {
		t.Errorf("ExtractText() text = %q, want it to contain Choices[1] blocklist keyword %q", text, "murder")
	}
}

func TestChatShapeFilter_ReplaceText(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "blocked text"}, "finish_reason": "stop"}]
	}`)
	out, err := chatShapeFilter{}.ReplaceText(body, "fallback text")
	if err != nil {
		t.Fatalf("ReplaceText() error = %v", err)
	}

	text, ok := chatShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(replaced) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(replaced) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"finish_reason":"content_filter"`) {
		t.Errorf("ReplaceText() output missing finish_reason=content_filter: %s", out)
	}
	if strings.Contains(string(out), "blocked text") {
		t.Errorf("ReplaceText() output still contains blocked text: %s", out)
	}
}

func TestChatShapeFilter_ExtractStreamText_CleanStream(t *testing.T) {
	raw := []byte(
		"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\n\n" +
			"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"}}]}\n\n" +
			"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n")

	text, ok := chatShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true")
	}
	if text != "Hello" {
		t.Errorf("ExtractStreamText() text = %q, want %q", text, "Hello")
	}
}

func TestChatShapeFilter_ExtractStreamText_KnownNonTextStream(t *testing.T) {
	raw := []byte(
		"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"\"}}]}}]}\n\n" +
			"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
			"data: [DONE]\n\n")

	text, ok := chatShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true (recognised tool_calls stream)")
	}
	if text != "" {
		t.Errorf("ExtractStreamText() text = %q, want \"\" for pure tool_calls stream", text)
	}
}

func TestChatShapeFilter_ExtractStreamText_Unparseable(t *testing.T) {
	_, ok := chatShapeFilter{}.ExtractStreamText([]byte("not an SSE stream at all"))
	if ok {
		t.Errorf("ExtractStreamText(garbage) ok = true, want false")
	}

	_, ok = chatShapeFilter{}.ExtractStreamText([]byte("data: [DONE]\n\n"))
	if ok {
		t.Errorf("ExtractStreamText(only [DONE]) ok = true, want false")
	}
}

// TestChatShapeFilter_ExtractStreamText_MultipleChoicesSecondBlocked covers
// design doc §5.2.1: ExtractStreamText accumulates deltas per Choices[].Index
// across chunks and concatenates all indices, so a blocklist keyword
// delivered only on index 1 is reflected in the returned text even though
// index 0 is clean.
func TestChatShapeFilter_ExtractStreamText_MultipleChoicesSecondBlocked(t *testing.T) {
	raw := []byte(
		"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n" +
			"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":1,\"delta\":{\"role\":\"assistant\",\"content\":\"I will murder you\"}}]}\n\n" +
			"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"},{\"index\":1,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n\n")

	text, ok := chatShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true")
	}
	if !strings.Contains(text, "Hello") {
		t.Errorf("ExtractStreamText() text = %q, want it to contain index 0 text %q", text, "Hello")
	}
	if !strings.Contains(text, "murder") {
		t.Errorf("ExtractStreamText() text = %q, want it to contain index 1 blocklist keyword %q", text, "murder")
	}
}

func TestChatShapeFilter_BuildFallbackStream(t *testing.T) {
	out := chatShapeFilter{}.BuildFallbackStream("fallback text")

	text, ok := chatShapeFilter{}.ExtractStreamText(out)
	if !ok {
		t.Fatalf("ExtractStreamText(fallback stream) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractStreamText(fallback stream) text = %q, want %q", text, "fallback text")
	}
	if !strings.HasSuffix(string(out), "data: [DONE]\n\n") {
		t.Errorf("BuildFallbackStream() does not end with data: [DONE], got %s", out)
	}
	if !strings.Contains(string(out), `"finish_reason":"content_filter"`) {
		t.Errorf("BuildFallbackStream() missing finish_reason=content_filter: %s", out)
	}
}

func TestChatShapeFilter_BuildFallbackBody(t *testing.T) {
	out := chatShapeFilter{}.BuildFallbackBody("fallback text")

	text, ok := chatShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(fallback body) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(fallback body) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"finish_reason":"content_filter"`) {
		t.Errorf("BuildFallbackBody() missing finish_reason=content_filter: %s", out)
	}
}

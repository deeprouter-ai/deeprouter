package kids

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestResponsesShapeFilter_ExtractText_CleanText(t *testing.T) {
	body := []byte(`{
		"id": "resp_1",
		"object": "response",
		"status": "completed",
		"output": [{"type": "message", "role": "assistant", "status": "completed", "content": [{"type": "output_text", "text": "Hello there", "annotations": []}]}]
	}`)
	text, ok := responsesShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if text != "Hello there" {
		t.Errorf("ExtractText() text = %q, want %q", text, "Hello there")
	}
}

func TestResponsesShapeFilter_ExtractText_KnownNonTextResponse(t *testing.T) {
	body := []byte(`{
		"id": "resp_1",
		"object": "response",
		"status": "completed",
		"output": [{"type": "function_call", "id": "item_1", "call_id": "call_1", "name": "f", "arguments": "{}"}]
	}`)
	text, ok := responsesShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true (recognised function_call response)")
	}
	if text != "" {
		t.Errorf("ExtractText() text = %q, want \"\" for pure function_call response", text)
	}
}

func TestResponsesShapeFilter_ExtractText_Unparseable(t *testing.T) {
	_, ok := responsesShapeFilter{}.ExtractText([]byte(`not json`))
	if ok {
		t.Errorf("ExtractText(garbage) ok = true, want false")
	}

	_, ok = responsesShapeFilter{}.ExtractText([]byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[]}`))
	if ok {
		t.Errorf("ExtractText(wrong object) ok = true, want false")
	}
}

func TestResponsesShapeFilter_ReplaceText(t *testing.T) {
	body := []byte(`{
		"id": "resp_1",
		"object": "response",
		"status": "completed",
		"output": [{"type": "message", "role": "assistant", "status": "completed", "content": [{"type": "output_text", "text": "blocked text", "annotations": []}]}]
	}`)
	out, err := responsesShapeFilter{}.ReplaceText(body, "fallback text")
	if err != nil {
		t.Fatalf("ReplaceText() error = %v", err)
	}

	text, ok := responsesShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(replaced) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(replaced) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"status":"completed"`) {
		t.Errorf("ReplaceText() output missing status=completed: %s", out)
	}
	if strings.Contains(string(out), "blocked text") {
		t.Errorf("ReplaceText() output still contains blocked text: %s", out)
	}
}

func TestResponsesShapeFilter_ReplaceText_ClearsOtherOutputEntries(t *testing.T) {
	body := []byte(`{
		"id": "resp_1",
		"object": "response",
		"status": "completed",
		"output": [
			{"type": "message", "role": "assistant", "status": "completed", "content": [{"type": "output_text", "text": "blocked text", "annotations": []}]},
			{"type": "function_call", "id": "item_1", "call_id": "call_1", "name": "f", "arguments": "{}"}
		]
	}`)
	out, err := responsesShapeFilter{}.ReplaceText(body, "fallback text")
	if err != nil {
		t.Fatalf("ReplaceText() error = %v", err)
	}

	var resp dto.OpenAIResponsesResponse
	if err := common.Unmarshal(out, &resp); err != nil {
		t.Fatalf("ReplaceText() output does not unmarshal as dto.OpenAIResponsesResponse: %v\nout: %s", err, out)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("Output: want exactly 1 entry (the fallback assistant message), got %d: %s", len(resp.Output), out)
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Role != "assistant" {
		t.Errorf("Output[0]: want type=message role=assistant, got type=%q role=%q", resp.Output[0].Type, resp.Output[0].Role)
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Type != "output_text" || resp.Output[0].Content[0].Text != "fallback text" {
		t.Errorf("Output[0].Content: want exactly 1 output_text block with fallback text, got %+v", resp.Output[0].Content)
	}
	if strings.Contains(string(out), "function_call") {
		t.Errorf("ReplaceText() output still contains a function_call entry: %s", out)
	}
	if strings.Contains(string(out), "blocked text") {
		t.Errorf("ReplaceText() output still contains blocked text: %s", out)
	}
}

func TestResponsesShapeFilter_ExtractStreamText_CleanStream(t *testing.T) {
	raw := []byte(
		"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"Hel\"}\n\n" +
			"event: response.output_text.delta\n" +
			"data: {\"type\":\"response.output_text.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"lo\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\",\"annotations\":[]}]}]}}\n\n")

	text, ok := responsesShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true")
	}
	if text != "Hello" {
		t.Errorf("ExtractStreamText() text = %q, want %q", text, "Hello")
	}
}

func TestResponsesShapeFilter_ExtractStreamText_KnownNonTextStream(t *testing.T) {
	raw := []byte(
		"event: response.function_call_arguments.delta\n" +
			"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"delta\":\"{}\"}\n\n" +
			"event: response.completed\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"f\",\"arguments\":\"{}\"}]}}\n\n")

	text, ok := responsesShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true (recognised function_call stream)")
	}
	if text != "" {
		t.Errorf("ExtractStreamText() text = %q, want \"\" for pure function_call stream", text)
	}
}

func TestResponsesShapeFilter_ExtractStreamText_Unparseable(t *testing.T) {
	_, ok := responsesShapeFilter{}.ExtractStreamText([]byte("not an SSE stream at all"))
	if ok {
		t.Errorf("ExtractStreamText(garbage) ok = true, want false")
	}

	_, ok = responsesShapeFilter{}.ExtractStreamText([]byte("data: {\"foo\":\"bar\"}\n\n"))
	if ok {
		t.Errorf("ExtractStreamText(no recognised type field) ok = true, want false")
	}
}

func TestResponsesShapeFilter_BuildFallbackStream(t *testing.T) {
	out := responsesShapeFilter{}.BuildFallbackStream("fallback text")

	text, ok := responsesShapeFilter{}.ExtractStreamText(out)
	if !ok {
		t.Fatalf("ExtractStreamText(fallback stream) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractStreamText(fallback stream) text = %q, want %q", text, "fallback text")
	}
	if !strings.HasPrefix(string(out), "event: response.output_text.delta\n") {
		t.Errorf("BuildFallbackStream() does not start with response.output_text.delta event, got %s", out)
	}
	if !strings.Contains(string(out), "event: response.completed\n") {
		t.Errorf("BuildFallbackStream() missing response.completed event: %s", out)
	}
}

func TestResponsesShapeFilter_BuildFallbackBody(t *testing.T) {
	out := responsesShapeFilter{}.BuildFallbackBody("fallback text")

	text, ok := responsesShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(fallback body) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(fallback body) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"status":"completed"`) {
		t.Errorf("BuildFallbackBody() missing status=completed: %s", out)
	}
}

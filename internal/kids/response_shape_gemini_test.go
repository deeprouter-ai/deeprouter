package kids

import (
	"strings"
	"testing"
)

func TestGeminiShapeFilter_ExtractText_CleanText(t *testing.T) {
	body := []byte(`{
		"candidates": [{"content": {"role": "model", "parts": [{"text": "Hello there"}]}, "finishReason": "STOP", "index": 0}]
	}`)
	text, ok := geminiShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if text != "Hello there" {
		t.Errorf("ExtractText() text = %q, want %q", text, "Hello there")
	}
}

func TestGeminiShapeFilter_ExtractText_KnownNonTextResponse(t *testing.T) {
	body := []byte(`{
		"candidates": [{"content": {"role": "model", "parts": [{"functionCall": {"name": "f", "args": {}}}]}, "finishReason": "STOP", "index": 0}]
	}`)
	text, ok := geminiShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true (recognised functionCall response)")
	}
	if text != "" {
		t.Errorf("ExtractText() text = %q, want \"\" for pure functionCall response", text)
	}
}

func TestGeminiShapeFilter_ExtractText_Unparseable(t *testing.T) {
	_, ok := geminiShapeFilter{}.ExtractText([]byte(`not json`))
	if ok {
		t.Errorf("ExtractText(garbage) ok = true, want false")
	}

	_, ok = geminiShapeFilter{}.ExtractText([]byte(`{"candidates": []}`))
	if ok {
		t.Errorf("ExtractText(no candidates) ok = true, want false")
	}
}

// TestGeminiShapeFilter_ExtractText_MultipleCandidatesSecondBlocked covers
// design doc §5.2.1: ExtractText concatenates ALL candidates, so a blocklist
// keyword in Candidates[1] is reflected in the returned text even though
// Candidates[0] is clean.
func TestGeminiShapeFilter_ExtractText_MultipleCandidatesSecondBlocked(t *testing.T) {
	body := []byte(`{
		"candidates": [
			{"content": {"role": "model", "parts": [{"text": "Hello there"}]}, "finishReason": "STOP", "index": 0},
			{"content": {"role": "model", "parts": [{"text": "I will murder you"}]}, "finishReason": "STOP", "index": 1}
		]
	}`)
	text, ok := geminiShapeFilter{}.ExtractText(body)
	if !ok {
		t.Fatalf("ExtractText() ok = false, want true")
	}
	if !strings.Contains(text, "Hello there") {
		t.Errorf("ExtractText() text = %q, want it to contain Candidates[0] text %q", text, "Hello there")
	}
	if !strings.Contains(text, "murder") {
		t.Errorf("ExtractText() text = %q, want it to contain Candidates[1] blocklist keyword %q", text, "murder")
	}
}

func TestGeminiShapeFilter_ReplaceText(t *testing.T) {
	body := []byte(`{
		"candidates": [{"content": {"role": "model", "parts": [{"text": "blocked text"}]}, "finishReason": "STOP", "index": 0}]
	}`)
	out, err := geminiShapeFilter{}.ReplaceText(body, "fallback text")
	if err != nil {
		t.Fatalf("ReplaceText() error = %v", err)
	}

	text, ok := geminiShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(replaced) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(replaced) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"finishReason":"SAFETY"`) {
		t.Errorf("ReplaceText() output missing finishReason=SAFETY: %s", out)
	}
	if strings.Contains(string(out), "blocked text") {
		t.Errorf("ReplaceText() output still contains blocked text: %s", out)
	}
}

func TestGeminiShapeFilter_ExtractStreamText_CleanStream(t *testing.T) {
	raw := []byte(
		"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hel\"}]},\"index\":0}]}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\",\"index\":0}]}\n\n")

	text, ok := geminiShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true")
	}
	if text != "Hello" {
		t.Errorf("ExtractStreamText() text = %q, want %q", text, "Hello")
	}
}

func TestGeminiShapeFilter_ExtractStreamText_KnownNonTextStream(t *testing.T) {
	raw := []byte(
		"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"name\":\"f\",\"args\":{}}}]},\"finishReason\":\"STOP\",\"index\":0}]}\n\n")

	text, ok := geminiShapeFilter{}.ExtractStreamText(raw)
	if !ok {
		t.Fatalf("ExtractStreamText() ok = false, want true (recognised functionCall stream)")
	}
	if text != "" {
		t.Errorf("ExtractStreamText() text = %q, want \"\" for pure functionCall stream", text)
	}
}

func TestGeminiShapeFilter_ExtractStreamText_Unparseable(t *testing.T) {
	_, ok := geminiShapeFilter{}.ExtractStreamText([]byte("not an SSE stream at all"))
	if ok {
		t.Errorf("ExtractStreamText(garbage) ok = true, want false")
	}

	_, ok = geminiShapeFilter{}.ExtractStreamText([]byte("data: {\"candidates\": []}\n\n"))
	if ok {
		t.Errorf("ExtractStreamText(no candidates) ok = true, want false")
	}
}

// TestGeminiShapeFilter_ExtractStreamText_MultipleCandidatesSecondBlocked
// covers design doc §5.2.1: ExtractStreamText accumulates parts per
// Candidates[].Index across chunks and concatenates all indices, so a
// blocklist keyword delivered only on index 1 is reflected in the returned
// text even though index 0 is clean.
func TestGeminiShapeFilter_ExtractStreamText_MultipleCandidatesSecondBlocked(t *testing.T) {
	raw := []byte(
		"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hel\"}]},\"index\":0}]}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"I will murder you\"}]},\"index\":1}]}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\",\"index\":0},{\"content\":{\"role\":\"model\",\"parts\":[]},\"finishReason\":\"STOP\",\"index\":1}]}\n\n")

	text, ok := geminiShapeFilter{}.ExtractStreamText(raw)
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

func TestGeminiShapeFilter_BuildFallbackStream(t *testing.T) {
	out := geminiShapeFilter{}.BuildFallbackStream("fallback text")

	text, ok := geminiShapeFilter{}.ExtractStreamText(out)
	if !ok {
		t.Fatalf("ExtractStreamText(fallback stream) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractStreamText(fallback stream) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"finishReason":"SAFETY"`) {
		t.Errorf("BuildFallbackStream() missing finishReason=SAFETY: %s", out)
	}
}

func TestGeminiShapeFilter_BuildFallbackBody(t *testing.T) {
	out := geminiShapeFilter{}.BuildFallbackBody("fallback text")

	text, ok := geminiShapeFilter{}.ExtractText(out)
	if !ok {
		t.Fatalf("ExtractText(fallback body) ok = false, want true")
	}
	if text != "fallback text" {
		t.Errorf("ExtractText(fallback body) text = %q, want %q", text, "fallback text")
	}
	if !strings.Contains(string(out), `"finishReason":"SAFETY"`) {
		t.Errorf("BuildFallbackBody() missing finishReason=SAFETY: %s", out)
	}
}

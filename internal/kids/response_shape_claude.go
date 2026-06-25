package kids

import (
	"bytes"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// claudeShapeFilter implements ShapeFilter for ResponseShapeClaudeMessages
// (/v1/messages), per design doc §5.2.
type claudeShapeFilter struct{}

// ExtractText implements ShapeFilter.
func (claudeShapeFilter) ExtractText(body []byte) (string, bool) {
	var resp dto.ClaudeResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return "", false
	}
	if resp.Type != "message" {
		// Doesn't match the Claude Messages non-stream response struct at
		// all — a real response always has top-level type "message".
		return "", false
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.GetText())
		}
	}
	// sb is "" for e.g. a pure tool_use response — recognised "clean,
	// nothing to filter" (ok=true, text=""), not a parse failure.
	return sb.String(), true
}

// ReplaceText implements ShapeFilter.
func (claudeShapeFilter) ReplaceText(body []byte, fallback string) ([]byte, error) {
	var resp dto.ClaudeResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Type != "message" {
		return nil, errors.New("kids: not a Claude Messages response")
	}
	fb := fallback
	resp.Content = []dto.ClaudeMediaMessage{{Type: "text", Text: &fb}}
	resp.StopReason = claudeBlockedStopReason
	return common.Marshal(resp)
}

// ExtractStreamText implements ShapeFilter.
func (claudeShapeFilter) ExtractStreamText(raw []byte) (string, bool) {
	var sb strings.Builder
	found := false
	for _, line := range sseLines(raw) {
		data, ok := sseData(line)
		if !ok {
			continue
		}
		var event dto.ClaudeResponse
		if err := common.Unmarshal([]byte(data), &event); err != nil {
			// A data: frame that doesn't even parse as JSON — cannot verify
			// safety, fail closed rather than silently dropping it (§5.3).
			return "", false
		}
		if event.Type == "" {
			// Doesn't match any real Claude SSE event shape (every real event
			// has a non-empty top-level type) — same fail-closed treatment as
			// a parse error.
			return "", false
		}
		found = true
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" {
			sb.WriteString(event.Delta.GetText())
		}
	}
	if !found {
		return "", false
	}
	return sb.String(), true
}

// BuildFallbackStream implements ShapeFilter.
func (claudeShapeFilter) BuildFallbackStream(fallback string) []byte {
	var buf bytes.Buffer

	zero := 0
	empty := ""
	fb := fallback
	stopReason := claudeBlockedStopReason

	writeSSEEvent(&buf, "message_start", dto.ClaudeResponse{
		Type:    "message_start",
		Message: &dto.ClaudeMediaMessage{Type: "message", Role: "assistant"},
	})
	writeSSEEvent(&buf, "content_block_start", dto.ClaudeResponse{
		Type:         "content_block_start",
		Index:        &zero,
		ContentBlock: &dto.ClaudeMediaMessage{Type: "text", Text: &empty},
	})
	writeSSEEvent(&buf, "content_block_delta", dto.ClaudeResponse{
		Type:  "content_block_delta",
		Index: &zero,
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: &fb},
	})
	writeSSEEvent(&buf, "content_block_stop", dto.ClaudeResponse{
		Type:  "content_block_stop",
		Index: &zero,
	})
	writeSSEEvent(&buf, "message_delta", dto.ClaudeResponse{
		Type:  "message_delta",
		Delta: &dto.ClaudeMediaMessage{StopReason: &stopReason},
	})
	writeSSEEvent(&buf, "message_stop", dto.ClaudeResponse{
		Type: "message_stop",
	})

	return buf.Bytes()
}

// BuildFallbackBody implements ShapeFilter.
func (claudeShapeFilter) BuildFallbackBody(fallback string) []byte {
	fb := fallback
	resp := dto.ClaudeResponse{
		Type:       "message",
		Role:       "assistant",
		Content:    []dto.ClaudeMediaMessage{{Type: "text", Text: &fb}},
		StopReason: claudeBlockedStopReason,
	}
	out, _ := common.Marshal(resp)
	return out
}

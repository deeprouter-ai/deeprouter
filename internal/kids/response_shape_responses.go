package kids

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// responsesShapeFilter implements ShapeFilter for ResponseShapeOpenAIResponses
// (/v1/responses), per design doc §5.2.
type responsesShapeFilter struct{}

// ExtractText implements ShapeFilter.
func (responsesShapeFilter) ExtractText(body []byte) (string, bool) {
	var resp dto.OpenAIResponsesResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return "", false
	}
	if resp.Object != "response" {
		// Doesn't match the OpenAI Responses non-stream response struct at
		// all — a real response always has top-level object "response".
		return "", false
	}
	var sb strings.Builder
	for _, out := range resp.Output {
		if out.Type != "message" {
			continue
		}
		for _, c := range out.Content {
			if c.Type == responsesOutputTextType {
				sb.WriteString(c.Text)
			}
		}
	}
	// sb is "" for e.g. a pure function_call response — recognised "clean,
	// nothing to filter" (ok=true, text=""), not a parse failure.
	return sb.String(), true
}

// ReplaceText implements ShapeFilter.
//
// A blocked response is replaced by exactly ONE fallback assistant message —
// the entire Output array is rebuilt from responsesFallbackResponse, so any
// other Output entries from the original response (additional output_text
// blocks, reasoning content, function_call items, etc.) are discarded rather
// than surviving alongside the fallback (design doc §5.4/§5.5).
func (responsesShapeFilter) ReplaceText(body []byte, fallback string) ([]byte, error) {
	var resp dto.OpenAIResponsesResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Object != "response" {
		return nil, errors.New("kids: not an OpenAI Responses response")
	}
	resp.Output = responsesFallbackResponse(fallback).Output
	resp.Status = json.RawMessage(`"completed"`)
	return common.Marshal(resp)
}

// ExtractStreamText implements ShapeFilter.
func (responsesShapeFilter) ExtractStreamText(raw []byte) (string, bool) {
	var sb strings.Builder
	found := false
	for _, line := range sseLines(raw) {
		data, ok := sseData(line)
		if !ok {
			continue
		}
		var event dto.ResponsesStreamResponse
		if err := common.Unmarshal([]byte(data), &event); err != nil {
			// A data: frame that doesn't even parse as JSON — cannot verify
			// safety, fail closed rather than silently dropping it (§5.3).
			return "", false
		}
		if !strings.HasPrefix(event.Type, "response.") {
			// Doesn't match the OpenAI Responses SSE event shape at all
			// (every real event's type is prefixed "response.") — same
			// fail-closed treatment as a parse error.
			return "", false
		}
		found = true
		if event.Type == "response.output_text.delta" {
			sb.WriteString(event.Delta)
		}
	}
	if !found {
		return "", false
	}
	return sb.String(), true
}

// BuildFallbackStream implements ShapeFilter.
func (responsesShapeFilter) BuildFallbackStream(fallback string) []byte {
	var buf bytes.Buffer

	writeSSEEvent(&buf, "response.output_text.delta", dto.ResponsesStreamResponse{
		Type:  "response.output_text.delta",
		Delta: fallback,
	})

	finalResp := responsesFallbackResponse(fallback)
	writeSSEEvent(&buf, "response.completed", dto.ResponsesStreamResponse{
		Type:     "response.completed",
		Response: &finalResp,
	})

	return buf.Bytes()
}

// BuildFallbackBody implements ShapeFilter.
func (responsesShapeFilter) BuildFallbackBody(fallback string) []byte {
	out, _ := common.Marshal(responsesFallbackResponse(fallback))
	return out
}

// responsesFallbackResponse builds the minimal, well-formed
// dto.OpenAIResponsesResponse used by both BuildFallbackBody and
// BuildFallbackStream's terminating "response.completed" event.
func responsesFallbackResponse(fallback string) dto.OpenAIResponsesResponse {
	return dto.OpenAIResponsesResponse{
		Object: "response",
		Status: json.RawMessage(`"completed"`),
		Output: []dto.ResponsesOutput{{
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []dto.ResponsesOutputContent{{
				Type:        responsesOutputTextType,
				Text:        fallback,
				Annotations: []interface{}{},
			}},
		}},
	}
}

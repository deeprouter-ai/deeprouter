package kids

import (
	"bytes"
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// chatShapeFilter implements ShapeFilter for ResponseShapeChatCompletions
// (/v1/chat/completions), per design doc §5.2.
type chatShapeFilter struct{}

// ExtractText implements ShapeFilter.
//
// Per design doc §5.2.1, ALL Choices[] are concatenated, not just Choices[0]:
// an n>1 response is one logical assistant turn split across choices, and a
// blocklist hit in any choice must block the whole response.
func (chatShapeFilter) ExtractText(body []byte) (string, bool) {
	var resp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return "", false
	}
	if len(resp.Choices) == 0 {
		// Doesn't match the Chat Completions response struct at all — a
		// real response always has at least one choice.
		return "", false
	}
	// Choice.Message.StringContent() returns "" for a pure tool_calls
	// message (Content == nil) — recognised "clean, nothing to filter"
	// (ok=true, text=""), not a parse failure.
	var sb strings.Builder
	for _, choice := range resp.Choices {
		sb.WriteString(choice.Message.StringContent())
	}
	return sb.String(), true
}

// ReplaceText implements ShapeFilter.
//
// A blocked response is replaced by exactly ONE fallback choice at index 0
// (design doc §5.2.1) — Choices is rebuilt from chatFallbackChoices, so any
// other choices from the original response are discarded rather than
// surviving alongside the fallback.
func (chatShapeFilter) ReplaceText(body []byte, fallback string) ([]byte, error) {
	var resp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("kids: chat completions response has no choices")
	}
	resp.Choices = chatFallbackChoices(fallback)
	return common.Marshal(resp)
}

// ExtractStreamText implements ShapeFilter.
//
// Per design doc §5.2.1, deltas are accumulated per Choices[].Index across
// chunks, then concatenated in index order — mirroring ExtractText's
// "concatenate all choices" semantics for the stream case.
func (chatShapeFilter) ExtractStreamText(raw []byte) (string, bool) {
	texts := make(map[int]*strings.Builder)
	var indices []int
	found := false
	for _, line := range sseLines(raw) {
		data, ok := sseData(line)
		if !ok {
			continue
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			// A data: frame that doesn't even parse as JSON — cannot verify
			// safety, fail closed rather than silently dropping it (§5.3).
			return "", false
		}
		if chunk.Object != "chat.completion.chunk" {
			// Doesn't match the Chat Completions stream chunk shape at all
			// (real chunks always have object="chat.completion.chunk") — same
			// fail-closed treatment as a parse error.
			return "", false
		}
		if len(chunk.Choices) == 0 {
			// Recognised "no text" chunk: the trailing usage-only chunk sent
			// when stream_options.include_usage is set
			// ({"object":"chat.completion.chunk","choices":[],"usage":{...}}).
			continue
		}
		found = true
		for _, choice := range chunk.Choices {
			sb, ok := texts[choice.Index]
			if !ok {
				sb = &strings.Builder{}
				texts[choice.Index] = sb
				indices = append(indices, choice.Index)
			}
			sb.WriteString(choice.Delta.GetContentString())
		}
	}
	if !found {
		return "", false
	}
	sort.Ints(indices)
	var sb strings.Builder
	for _, idx := range indices {
		sb.WriteString(texts[idx].String())
	}
	return sb.String(), true
}

// BuildFallbackStream implements ShapeFilter.
func (chatShapeFilter) BuildFallbackStream(fallback string) []byte {
	var buf bytes.Buffer

	contentChunk := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0}},
	}
	contentChunk.Choices[0].Delta.Role = "assistant"
	contentChunk.Choices[0].Delta.SetContentString(fallback)
	writeSSEData(&buf, contentChunk)

	finishReason := chatContentFilterFinishReason
	finishChunk := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0, FinishReason: &finishReason}},
	}
	writeSSEData(&buf, finishChunk)

	buf.WriteString("data: [DONE]\n\n")
	return buf.Bytes()
}

// BuildFallbackBody implements ShapeFilter.
func (chatShapeFilter) BuildFallbackBody(fallback string) []byte {
	resp := dto.OpenAITextResponse{
		Object:  "chat.completion",
		Choices: chatFallbackChoices(fallback),
	}
	out, _ := common.Marshal(resp)
	return out
}

// chatFallbackChoices builds the single-choice Choices[] slice used by both
// BuildFallbackBody and ReplaceText (design doc §5.2.1): index 0, role
// assistant, Content==fallback, FinishReason==chatContentFilterFinishReason,
// with no tool_calls/reasoning fields.
func chatFallbackChoices(fallback string) []dto.OpenAITextResponseChoice {
	choice := dto.OpenAITextResponseChoice{
		Index:        0,
		FinishReason: chatContentFilterFinishReason,
	}
	choice.Message.Role = "assistant"
	choice.Message.SetStringContent(fallback)
	return []dto.OpenAITextResponseChoice{choice}
}

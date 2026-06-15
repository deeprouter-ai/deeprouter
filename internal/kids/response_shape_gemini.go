package kids

import (
	"bytes"
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// geminiShapeFilter implements ShapeFilter for ResponseShapeGemini
// (/v1/models/{m}:generateContent and :streamGenerateContent), per design
// doc §5.2.
type geminiShapeFilter struct{}

// ExtractText implements ShapeFilter.
//
// Per design doc §5.2.1, ALL Candidates[] are concatenated, not just
// Candidates[0]: an n>1 response is one logical model turn split across
// candidates, and a blocklist hit in any candidate must block the whole
// response.
func (geminiShapeFilter) ExtractText(body []byte) (string, bool) {
	var resp dto.GeminiChatResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return "", false
	}
	if len(resp.Candidates) == 0 {
		// Doesn't match the Gemini response struct at all — a real response
		// always has at least one candidate.
		return "", false
	}
	var sb strings.Builder
	for _, candidate := range resp.Candidates {
		for _, part := range candidate.Content.Parts {
			sb.WriteString(part.Text)
		}
	}
	// sb is "" for e.g. a pure functionCall response — recognised "clean,
	// nothing to filter" (ok=true, text=""), not a parse failure.
	return sb.String(), true
}

// ReplaceText implements ShapeFilter.
//
// A blocked response is replaced by exactly ONE fallback candidate at index
// 0 (design doc §5.2.1) — Candidates is rebuilt from
// geminiFallbackResponse(fallback).Candidates, so any other candidates from
// the original response are discarded rather than surviving alongside the
// fallback.
func (geminiShapeFilter) ReplaceText(body []byte, fallback string) ([]byte, error) {
	var resp dto.GeminiChatResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Candidates) == 0 {
		return nil, errors.New("kids: gemini response has no candidates")
	}
	resp.Candidates = geminiFallbackResponse(fallback).Candidates
	return common.Marshal(resp)
}

// ExtractStreamText implements ShapeFilter.
//
// Per design doc §5.2.1, parts are accumulated per Candidates[].Index across
// chunks, then concatenated in index order — mirroring ExtractText's
// "concatenate all candidates" semantics for the stream case.
func (geminiShapeFilter) ExtractStreamText(raw []byte) (string, bool) {
	texts := make(map[int64]*strings.Builder)
	var indices []int64
	found := false
	for _, line := range sseLines(raw) {
		data, ok := sseData(line)
		if !ok {
			continue
		}
		var chunk dto.GeminiChatResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			// A data: frame that doesn't even parse as JSON — cannot verify
			// safety, fail closed rather than silently dropping it (§5.3).
			return "", false
		}
		if len(chunk.Candidates) == 0 {
			// Doesn't match the Gemini stream chunk shape at all (a real
			// chunk always has at least one candidate) — same fail-closed
			// treatment as a parse error.
			return "", false
		}
		found = true
		for _, candidate := range chunk.Candidates {
			sb, ok := texts[candidate.Index]
			if !ok {
				sb = &strings.Builder{}
				texts[candidate.Index] = sb
				indices = append(indices, candidate.Index)
			}
			for _, part := range candidate.Content.Parts {
				sb.WriteString(part.Text)
			}
		}
	}
	if !found {
		return "", false
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	var sb strings.Builder
	for _, idx := range indices {
		sb.WriteString(texts[idx].String())
	}
	return sb.String(), true
}

// BuildFallbackStream implements ShapeFilter.
//
// Gemini streams end naturally (no explicit terminator event, per design
// doc §5.2 table), so a single "data:" chunk carrying the entire fallback
// text is a minimal, well-formed replacement stream.
func (geminiShapeFilter) BuildFallbackStream(fallback string) []byte {
	var buf bytes.Buffer
	writeSSEData(&buf, geminiFallbackResponse(fallback))
	return buf.Bytes()
}

// BuildFallbackBody implements ShapeFilter.
func (geminiShapeFilter) BuildFallbackBody(fallback string) []byte {
	out, _ := common.Marshal(geminiFallbackResponse(fallback))
	return out
}

// geminiFallbackResponse builds the minimal, well-formed
// dto.GeminiChatResponse used by both BuildFallbackBody and
// BuildFallbackStream.
func geminiFallbackResponse(fallback string) dto.GeminiChatResponse {
	finishReason := geminiBlockedFinishReason
	return dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content: dto.GeminiChatContent{
				Role:  "model",
				Parts: []dto.GeminiPart{{Text: fallback}},
			},
			FinishReason: &finishReason,
			Index:        0,
		}},
	}
}

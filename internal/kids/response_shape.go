package kids

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ResponseShape identifies which relay endpoint's response wire format is
// being filtered (PRD §6.2 Step 1 "文本：再次过滤"). Each shape has a
// different non-stream JSON body and a different stream SSE framing.
type ResponseShape int

const (
	ResponseShapeChatCompletions ResponseShape = iota // /v1/chat/completions
	ResponseShapeClaudeMessages                       // /v1/messages
	ResponseShapeOpenAIResponses                      // /v1/responses
	ResponseShapeGemini                               // /v1/models/{m}:generateContent (+ streamGenerateContent)
)

// ShapeFilter extracts the assistant-visible text from a fully-buffered
// response (for classification) and, on a Blocked verdict, rewrites the
// buffered bytes to deliver SafeFallbackText() instead — in the same wire
// format, so the client's parser does not choke. Implementations are pure:
// []byte in, []byte out, no I/O.
type ShapeFilter interface {
	// ExtractText concatenates all assistant-visible text segments from a
	// complete non-stream JSON response body.
	//
	// ok=false means body does not parse as this shape's expected non-stream
	// structure at all (e.g. malformed JSON, or a body that doesn't match the
	// shape's response struct) — finalize() (§5.3) treats this as "cannot
	// verify safety" and fails closed.
	//
	// ok=true, text="" means body DOES parse as this shape's expected
	// structure but contains no assistant-visible text in the recognised
	// text fields — e.g. a Chat Completions response whose message is a
	// pure tool_calls call with empty Content. This is a recognised "clean,
	// nothing to filter" case, NOT a parse failure: finalize() treats
	// ok=true + empty text as passing the filter (verdict.Blocked=false for
	// empty input), not as fail-closed. Implementations MUST distinguish
	// these two cases — collapsing "no text field" into ok=false would make
	// every tool-call response fail closed (§7.3 tests this explicitly).
	ExtractText(body []byte) (text string, ok bool)

	// ReplaceText returns body with the assistant text replaced by fallback
	// and the shape's finish/stop-reason field set to its "blocked by
	// content filter" equivalent (table in §5.4). body is assumed to already
	// have parsed successfully via ExtractText (ok=true); ReplaceText itself
	// can still fail (e.g. unexpected nested structure) — see
	// BuildFallbackBody below for the last-resort path.
	ReplaceText(body []byte, fallback string) ([]byte, error)

	// ExtractStreamText concatenates all assistant-visible delta text from a
	// complete (fully-buffered) raw SSE byte stream. Same ok=false vs
	// ok=true+text="" distinction as ExtractText (e.g. a stream whose only
	// content is tool_call deltas is ok=true, text="").
	ExtractStreamText(raw []byte) (text string, ok bool)

	// BuildFallbackStream returns a minimal, well-formed SSE byte stream in
	// this shape's wire format that delivers fallback as the entire
	// assistant message and terminates the stream the way a normal
	// completion would (table in §5.4).
	BuildFallbackStream(fallback string) []byte

	// BuildFallbackBody returns a minimal, well-formed non-stream JSON
	// response body in this shape's wire format (mirrors the "非流式 dto
	// 类型" column of the table below) that delivers fallback as the entire
	// assistant message, with the finish/stop-reason field set per §5.4.
	// Unlike ReplaceText, this does NOT need an input body to parse — it is
	// the fail-closed path for non-stream responses whose raw bytes did not
	// parse at all (ExtractText ok=false) or whose ReplaceText itself failed
	// (§5.3 finalize()).
	BuildFallbackBody(fallback string) []byte
}

// FilterForShape returns the ShapeFilter implementation for shape. It panics
// for an unrecognised shape value — all 4 ResponseShape constants above are
// handled, and relay/airbotix_policy.go always passes one of them.
func FilterForShape(shape ResponseShape) ShapeFilter {
	switch shape {
	case ResponseShapeChatCompletions:
		return chatShapeFilter{}
	case ResponseShapeClaudeMessages:
		return claudeShapeFilter{}
	case ResponseShapeOpenAIResponses:
		return responsesShapeFilter{}
	case ResponseShapeGemini:
		return geminiShapeFilter{}
	default:
		panic("kids: FilterForShape: unknown ResponseShape")
	}
}

// Finish/stop-reason values used by ReplaceText / BuildFallbackStream /
// BuildFallbackBody to mark a response as blocked by the output filter
// (design doc §5.4). The fallback TEXT itself (kids.SafeFallbackText()) is
// what actually communicates the refusal to the end user — these
// finish/stop values are conservative, already-existing enum values for
// their respective shapes, chosen so well-behaved clients don't choke on an
// unrecognised value.
const (
	// chatContentFilterFinishReason is OpenAI's own enum value for
	// "response stopped due to content filter" (Chat Completions
	// choices[].finish_reason).
	chatContentFilterFinishReason = "content_filter"

	// claudeBlockedStopReason is the conservative stop_reason used for
	// blocked Claude Messages responses (confirmed §十二 / 阶段0核对2:
	// "end_turn" is an existing normal-completion value, reused here because
	// the fallback text itself conveys the refusal).
	claudeBlockedStopReason = "end_turn"

	// responsesOutputTextType is the OpenAI Responses output content type
	// used for both normal and fallback text (confirmed §十二 / 阶段0核对2:
	// "output_text" is reused as-is — the fallback text itself conveys the
	// refusal).
	responsesOutputTextType = "output_text"

	// geminiBlockedFinishReason is Gemini's own enum value for "response
	// stopped due to safety filtering" (candidates[].finishReason).
	geminiBlockedFinishReason = "SAFETY"
)

// maxSSELineBytes bounds a single SSE line scanned by sseLines. The overall
// raw stream is already capped at maxOutputFilterBufferBytes (1 MiB,
// relay/airbotix_policy.go) before reaching here; this is just the
// bufio.Scanner token-size limit for one line within that buffer.
const maxSSELineBytes = 1 << 20

// sseLines splits a fully-buffered SSE byte stream into its non-empty,
// whitespace-trimmed lines (e.g. "event: message_stop", "data: {...}").
// Blank lines (the SSE event separators) are dropped, which is sufficient
// for the simple "event:"/"data:" pairing the 4 ShapeFilter implementations
// need — none of them care about event boundaries beyond pairing each
// "data:" line with the most recently seen "event:" line.
func sseLines(raw []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), maxSSELineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// sseData returns the trimmed payload of a "data: ..." line, and whether the
// line was a data line at all. A bare "data:" with no payload (or
// "data: [DONE]", the Chat Completions stream terminator) returns ok=false
// from the *payload* perspective — callers skip it.
func sseData(line string) (data string, ok bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" || data == "[DONE]" {
		return "", false
	}
	return data, true
}

// writeSSEData appends a plain "data: <json>\n\n" frame (no "event:" line)
// to buf, marshalling v with common.Marshal (AGENTS.md Rule 1). Marshal
// errors are ignored: v is always a value type built by this package, never
// user-controlled, so common.Marshal cannot fail in practice.
func writeSSEData(buf *bytes.Buffer, v any) {
	data, err := common.Marshal(v)
	if err != nil {
		return
	}
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteString("\n\n")
}

// writeSSEEvent appends an "event: <event>\ndata: <json>\n\n" frame to buf,
// marshalling v with common.Marshal (AGENTS.md Rule 1). Marshal errors are
// ignored: v is always a value type built by this package, never
// user-controlled, so common.Marshal cannot fail in practice.
func writeSSEEvent(buf *bytes.Buffer, event string, v any) {
	data, err := common.Marshal(v)
	if err != nil {
		return
	}
	buf.WriteString("event: ")
	buf.WriteString(event)
	buf.WriteString("\ndata: ")
	buf.Write(data)
	buf.WriteString("\n\n")
}

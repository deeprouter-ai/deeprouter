# internal/kids

Hard-constraint helpers for `kids_mode` tenants. Pure transformations — no I/O, no DB, no side effects.

**Status**: ✅ Implemented + unit-tested + wired via `relay/airbotix_policy.go`.

## What it does

When a tenant has `kids_mode = true` (set on `model/user.go`), four constraints apply before the request is forwarded upstream:

1. **Model whitelist** — only specific safe models are allowed. Non-whitelisted models → 400.
2. **Metadata strip** — `user`, `metadata.user_id`, `metadata.kid_profile_id`, `metadata.family_id`, `metadata.kid_id` are removed so we don't leak child identifiers to the provider.
3. **OpenAI Zero-Data-Retention (ZDR)** — force `store: false` on OpenAI / Azure OpenAI calls so the upstream doesn't retain transcripts.
4. **Child-safe system prompt** — prepend a curated system message that constrains tone, topics, and refusals.

Two more constraints apply to the **response**, after the upstream reply is received (PRD §6.2 Step 1 "文本：再次过滤", ticket AC1 "Strict output filter applied"):

5. **Output filter** — the assistant's reply text is classified against a blocklist; a blocked reply never reaches the client.
6. **Response shape filters** — per-endpoint helpers that extract/replace that text in each of the 4 supported wire formats (Chat Completions, Claude Messages, OpenAI Responses, Gemini), stream and non-stream.

This package only provides the helper functions; the decision to apply them is made in `internal/policy/` and the orchestration (request-time constraints 1-4, and the `outputFilterWriter` buffering wrapper for 5-6) is in `relay/airbotix_policy.go`.

## Public API

```go
var EligibleModels map[string]bool   // whitelist (HasPrefix-matched for versioned variants)

func IsModelEligible(model string) bool
func StripIdentifyingMetadata(req map[string]any) map[string]any
func EnforceZeroDataRetention(req map[string]any, providerType string)
func ChildSafeSystemPrompt() string
```

## The whitelist

Hardcoded in this package. Stays deliberately narrow:

| Family | Models |
|---|---|
| OpenAI chat | `gpt-4o-mini`, `gpt-4o` |
| Anthropic | `claude-3-5-haiku`, `claude-3-5-sonnet` |

No image models are eligible. `gpt-image-2`, `gpt-image-1`, `flux-schnell`,
and `flux-1.1-pro` were removed on 2026-06-15: the strict output filter (§5
below) only covers the 4 text response shapes, so an image model on this
whitelist would let kids_mode tenants reach `/v1/images/generations` /
`/v1/images/edits` (`relay/image_handler.go`) with zero output filtering.
Re-add only once an image NSFW filter covers those endpoints.

DALL-E 3 was retired from the whitelist on 2026-05-12. Versioned variants (e.g. `gpt-4o-2024-08-06`) match via `HasPrefix`.

**Review before extending the whitelist** — each addition is a kids-safety decision, not a routine code change.

## 5. Output filter

`output_filter.go`. Classifies accumulated assistant-visible text against `StrictOutputBlocklist` (PRD §6.4-pre: NSFW + violence + hate, "强制最严档，不允许 tenant 配置宽松档").

```go
type OutputVerdict struct {
    Blocked    bool
    Categories []string // e.g. ["nsfw", "violence", "hate"]
}

type OutputFilter interface {
    Check(text string) OutputVerdict
}

const (
    OutputCategoryNSFW     = "nsfw"
    OutputCategoryViolence = "violence"
    OutputCategoryHate     = "hate"
)

var StrictOutputBlocklist []string // non-empty V0 seed list, see below

func CheckOutputText(text string) OutputVerdict

// V0 default outputFilter (relay/airbotix_policy.go), delegates to CheckOutputText.
type StrictKeywordFilter struct{}
func (StrictKeywordFilter) Check(text string) OutputVerdict

// D-DR6 static "polite refusal" template used to replace blocked output.
func SafeFallbackText() string
```

`StrictKeywordFilter{}` is the V0 default `outputFilter` (confirmed §3.1/§3.3) — a case-insensitive substring match against `StrictOutputBlocklist`. The blocklist is a **non-empty V0 seed list**: 1-2 representative terms per category (NSFW, violence, hate) so that AC1 "applied" is substantively true at V0, not just mechanically wired with a filter that can never match. The full/refined word list is a security/compliance review follow-up (§12 item 2) — extending it does not change the `OutputFilter` interface.

DRS-7 will replace `StrictKeywordFilter` with an LLM-backed or Airbotix-classifier-backed implementation behind the same `OutputFilter` interface — no other code needs to change when that lands (§3.3).

## 6. Response shape filters

`response_shape.go` + `response_shape_{chat,claude,responses,gemini}.go`. Each of the 4 relay endpoints in scope has a different non-stream JSON body and stream SSE framing; `ShapeFilter` abstracts "how to get assistant text out of / back into" each one, so `relay/airbotix_policy.go`'s `outputFilterWriter` (§5.3 of the design doc) only needs to know *when* to call it.

```go
type ResponseShape int

const (
    ResponseShapeChatCompletions ResponseShape = iota // /v1/chat/completions
    ResponseShapeClaudeMessages                       // /v1/messages
    ResponseShapeOpenAIResponses                      // /v1/responses
    ResponseShapeGemini                               // /v1/models/{m}:generateContent (+ streamGenerateContent)
)

type ShapeFilter interface {
    ExtractText(body []byte) (text string, ok bool)
    ReplaceText(body []byte, fallback string) ([]byte, error)
    ExtractStreamText(raw []byte) (text string, ok bool)
    BuildFallbackStream(fallback string) []byte
    BuildFallbackBody(fallback string) []byte
}

func FilterForShape(shape ResponseShape) ShapeFilter
```

**`ok=false` vs `ok=true, text==""`** — implementations MUST distinguish these:
- `ok=false`: body/stream does not parse as this shape's expected structure at all (malformed JSON, or doesn't match the shape's response struct). The caller treats this as "cannot verify safety" and fails closed (replaces the whole response with `SafeFallbackText()`).
- `ok=true, text==""`: body/stream DOES parse as expected but has no assistant-visible text in the recognised fields — e.g. a Chat Completions response whose message is a pure `tool_calls` call. This is a recognised "clean, nothing to filter" case and passes through unchanged.

Field mapping per shape (non-stream / stream / stream terminator):

| Shape | 非流式 dto 类型 / 文本路径 | 流式 dto 类型 / 累积方式 | 流式终止标记 |
|---|---|---|---|
| `ResponseShapeChatCompletions` | `dto.OpenAITextResponse.Choices[].Message`（`Content`，遍历**所有** choices）/ `FinishReason` | `dto.ChatCompletionsStreamResponse`，逐 `data:` 行按 `Choices[].Index` 累积**所有** choice 的 `Delta.Content`，最终按 index 顺序拼接 | `data: [DONE]\n\n` |
| `ResponseShapeClaudeMessages` | `dto.ClaudeResponse.Content[]`（`type:"text"` 块的 `Text`）/ `StopReason` | 同一 `dto.ClaudeResponse`，按 `event: content_block_delta` 累积 `Delta.Text`（`type:"text_delta"`） | `event: message_stop` |
| `ResponseShapeOpenAIResponses` | `dto.OpenAIResponsesResponse.Output[].Content[]`（`type:"output_text"` 的 `Text`） | `dto.ResponsesStreamResponse`，累积 `event: response.output_text.delta` 的 `Delta` 字段 | `event: response.completed` |
| `ResponseShapeGemini` | `dto.GeminiChatResponse.Candidates[]`（遍历**所有** candidates）的 `Content.Parts[].Text` / `FinishReason` | 同一 `dto.GeminiChatResponse`，逐 `data:` 行按 `Candidates[].Index` 累积**所有** candidate 的 `Content.Parts[].Text`，最终按 index 顺序拼接 | 流自然结束（无显式 sentinel） |

`ResponseShapeChatCompletions` / `ResponseShapeGemini` 的 `ExtractText`/`ExtractStreamText` 拼接**所有** choices/candidates 的文本后再交给 `OutputFilter.Check`——只要任意一个 choice/candidate 命中 blocklist，整个响应即视为 `Blocked`。对应地，命中阻断后 `ReplaceText`/`BuildFallbackBody`/`BuildFallbackStream` 会把 `Choices`/`Candidates` 整体重建为**恰好 1 个** fallback 条目（`index`/`Index`=0），原响应中的其它 choices/candidates 被丢弃，不会以未过滤的原文出现在输出中。

On a `Blocked` verdict, `ReplaceText`/`BuildFallbackStream`/`BuildFallbackBody` set the shape's finish/stop-reason field to a conservative "blocked" value (`content_filter` / `end_turn` / `output_text` / `SAFETY`) — the fallback *text* itself (`SafeFallbackText()`) is what actually communicates the refusal.

## Dependencies

- `output_filter.go`: stdlib `strings` only
- `response_shape*.go`: stdlib `bufio`, `bytes`, `encoding/json` (for `json.RawMessage` field types only — JSON (un)marshalling itself goes through `common.Marshal`/`common.Unmarshal`, AGENTS.md Rule 1), `errors`, `strings`, plus `github.com/QuantumNous/new-api/{common,dto}`

Zero imports from any other `internal/` package.

## Tests

`kids_test.go` (90 LOC) covers:
- Whitelist membership (incl. versioned variants and DALL-E removal)
- Metadata strip with selective removal + empty-metadata cleanup
- ZDR applied only for openai / azure / azure-openai provider types
- System prompt non-empty and stable

`output_filter_test.go` covers `CheckOutputText` (blocklist hit per category, case-insensitivity, clean text), `SafeFallbackText`, and `StrictKeywordFilter`'s delegation.

`response_shape_{chat,claude,responses,gemini}_test.go` each cover the same 7 cases: `ExtractText`/`ExtractStreamText` for a clean text response, a known non-text response (`ok=true, text==""`), and an unparseable body (`ok=false`); plus `ReplaceText`, `BuildFallbackStream`, `BuildFallbackBody`.

Run: `go test ./internal/kids/...`

## Versioning the whitelist

Date comments inside the source (`// 2026-05-12 dropped DALL-E 3`, `// 2026-06-15 dropped image models`) document real rollouts. When extending, add a similar dated comment and an explicit test case.

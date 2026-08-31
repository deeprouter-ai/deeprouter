package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// bridgeInfo is a RelayInfo for a Gemini-format request landing on a Claude
// channel — the combination that returned 500 not implemented.
func bridgeInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatGemini,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-sonnet-4-6"},
	}
}

func TestConvertGeminiRequest_ProducesClaudeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	got, err := (&Adaptor{}).ConvertGeminiRequest(c, bridgeInfo(), &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "hello"}}},
		},
		SystemInstructions: &dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{Text: "you are a translator"}},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			Temperature:     lo.ToPtr(0.7),
			MaxOutputTokens: lo.ToPtr(uint(256)),
			TopK:            lo.ToPtr(float64(40)),
		},
	})

	require.NoError(t, err, "this path used to return 'not implemented'")
	claudeRequest, ok := got.(*dto.ClaudeRequest)
	require.True(t, ok, "expected a *dto.ClaudeRequest, got %T", got)

	require.Equal(t, "claude-sonnet-4-6", claudeRequest.Model)
	require.NotNil(t, claudeRequest.MaxTokens)
	require.Equal(t, uint(256), *claudeRequest.MaxTokens)

	// Gemini's "model" role is Claude's "assistant".
	require.Len(t, claudeRequest.Messages, 2)
	require.Equal(t, "user", claudeRequest.Messages[0].Role)
	require.Equal(t, "assistant", claudeRequest.Messages[1].Role)

	// systemInstruction is a top-level Gemini field and must survive as Claude's
	// own top-level system field, not be dropped or folded into a turn. Claude
	// takes it as an array of blocks, which is what the converter emits.
	systemBlocks := claudeRequest.ParseSystem()
	require.Len(t, systemBlocks, 1)
	require.Contains(t, systemBlocks[0].GetText(), "you are a translator")

	// Claude does have top_k, so unlike the OpenAI path it is carried, not dropped.
	require.NotNil(t, claudeRequest.TopK)
	require.Equal(t, 40, *claudeRequest.TopK)
}

func TestConvertGeminiRequest_NilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := (&Adaptor{}).ConvertGeminiRequest(c, bridgeInfo(), nil)
	require.Error(t, err)
}

// Before this case existed the switch fell through, responseData stayed nil,
// and the caller got an empty 200 — worse than the 500 on the request side,
// because nothing reports it.
func TestHandleClaudeResponseData_GeminiFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	claudeBody, err := json.Marshal(dto.ClaudeResponse{
		Id:      "msg_1",
		Type:    "message",
		Role:    "assistant",
		Model:   "claude-sonnet-4-6",
		Content: []dto.ClaudeMediaMessage{{Type: "text", Text: lo.ToPtr("hello there")}},
		Usage:   &dto.ClaudeUsage{InputTokens: 10, OutputTokens: 3},
	})
	require.NoError(t, err)

	apiErr := HandleClaudeResponseData(c, bridgeInfo(), &ClaudeResponseInfo{Usage: &dto.Usage{}},
		&http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, claudeBody)
	require.Nil(t, apiErr)

	var geminiResponse dto.GeminiChatResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &geminiResponse),
		"body should be a Gemini response, got: %s", w.Body.String())
	require.NotEmpty(t, geminiResponse.Candidates, "empty candidates means the conversion produced nothing")
	require.Equal(t, "hello there", geminiResponse.Candidates[0].Content.Parts[0].Text)
	require.Equal(t, 10, geminiResponse.UsageMetadata.PromptTokenCount)
	require.Equal(t, 3, geminiResponse.UsageMetadata.CandidatesTokenCount)
}

func TestHandleStreamResponseData_GeminiFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	info := bridgeInfo()
	info.IsStream = true
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}

	for _, chunk := range []dto.ClaudeResponse{
		{Type: "message_start", Message: &dto.ClaudeMediaMessage{
			Id: "msg_1", Model: "claude-sonnet-4-6",
			Usage: &dto.ClaudeUsage{InputTokens: 10, OutputTokens: 1}}},
		{Type: "content_block_delta", Index: lo.ToPtr(0),
			Delta: &dto.ClaudeMediaMessage{Type: "text_delta", Text: lo.ToPtr("hello")}},
		{Type: "message_delta", Delta: &dto.ClaudeMediaMessage{StopReason: lo.ToPtr("end_turn")},
			Usage: &dto.ClaudeUsage{OutputTokens: 3}},
	} {
		data, err := json.Marshal(chunk)
		require.NoError(t, err)
		require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, string(data)))
	}

	body := w.Body.String()
	require.Contains(t, body, "candidates", "stream should carry Gemini-shaped chunks, got: %s", body)
	require.Contains(t, body, "hello")
	require.NotContains(t, body, "[DONE]", "Gemini's SSE stream has no [DONE] sentinel")

	// Claude only reports token counts on message_delta, and the per-chunk
	// conversion drops them unless they are attached to the finishing chunk.
	require.Contains(t, body, "usageMetadata")
}

// The OpenAI and Claude output formats must be untouched by the new branch.
func TestHandleClaudeResponseData_OtherFormatsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	claudeBody, err := json.Marshal(dto.ClaudeResponse{
		Id: "msg_1", Type: "message", Role: "assistant", Model: "claude-sonnet-4-6",
		Content: []dto.ClaudeMediaMessage{{Type: "text", Text: lo.ToPtr("hi")}},
		Usage:   &dto.ClaudeUsage{InputTokens: 5, OutputTokens: 2},
	})
	require.NoError(t, err)

	t.Run("claude passes through byte-for-byte", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		info := bridgeInfo()
		info.RelayFormat = types.RelayFormatClaude
		require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}},
			&http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, claudeBody))
		require.JSONEq(t, string(claudeBody), w.Body.String())
	})

	t.Run("openai still converts", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		info := bridgeInfo()
		info.RelayFormat = types.RelayFormatOpenAI
		require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}},
			&http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, claudeBody))
		var openaiResponse dto.OpenAITextResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &openaiResponse))
		require.NotEmpty(t, openaiResponse.Choices)
	})
}

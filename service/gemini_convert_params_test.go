package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// testRelayInfo is the minimum RelayInfo the converters read: the upstream
// model name, which lives on the embedded ChannelMeta.
func geminiTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o-mini"},
	}
}

// geminiReq builds a minimal Gemini request carrying the given generation config.
func geminiReq(cfg dto.GeminiChatGenerationConfig) *dto.GeminiChatRequest {
	return &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}},
		},
		GenerationConfig: cfg,
	}
}

// A Gemini request always carries topK, and OpenAI rejects the whole request
// with 400 on an argument it does not know. The neutral request still carries
// it — channels like ollama and vertex do accept it — so what this asserts is
// that the conversion keeps producing it and the drop happens per-channel in
// the OpenAI adaptor. See TestConvertOpenAIRequest_DropsTopKForOpenAI.
func TestGeminiToOpenAIRequest_CarriesTopKIntoNeutralRequest(t *testing.T) {
	got, err := GeminiToOpenAIRequest(
		geminiReq(dto.GeminiChatGenerationConfig{TopK: lo.ToPtr(float64(40))}),
		geminiTestRelayInfo(),
	)
	require.NoError(t, err)
	require.NotNil(t, got.TopK)
	require.Equal(t, 40, *got.TopK)
}

// Slicing is bounded by cap, and a slice decoded from JSON has cap == len, so
// stopSequences[:4] panicked on anything shorter than four entries — a 500 on
// a request that named a single stop word.
func TestGeminiToOpenAIRequest_StopSequencesShorterThanFour(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want int
	}{
		{"one", []string{"END"}, 1},
		{"three", []string{"a", "b", "c"}, 3},
		{"exactly four", []string{"a", "b", "c", "d"}, 4},
		{"five is clamped", []string{"a", "b", "c", "d", "e"}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GeminiToOpenAIRequest(
				geminiReq(dto.GeminiChatGenerationConfig{StopSequences: tc.in}),
				geminiTestRelayInfo(),
			)
			require.NoError(t, err)
			require.Len(t, got.Stop, tc.want)
		})
	}
}

// The rest of generationConfig, so the next field to go missing fails here
// rather than as a 400 from an upstream. Fields Gemini has and OpenAI does not
// are absent on purpose — being dropped is the correct outcome, not a gap.
func TestGeminiToOpenAIRequest_GenerationConfigMapping(t *testing.T) {
	got, err := GeminiToOpenAIRequest(geminiReq(dto.GeminiChatGenerationConfig{
		Temperature:      lo.ToPtr(0.7),
		TopP:             lo.ToPtr(0.9),
		MaxOutputTokens:  lo.ToPtr(uint(256)),
		CandidateCount:   lo.ToPtr(2),
		StopSequences:    []string{"STOP"},
		ResponseMimeType: "application/json",
		PresencePenalty:  lo.ToPtr(float32(0.5)),
		Seed:             lo.ToPtr(int64(42)),
	}), geminiTestRelayInfo())
	require.NoError(t, err)

	require.Equal(t, 0.7, *got.Temperature, "temperature -> temperature")
	require.Equal(t, 0.9, *got.TopP, "topP -> top_p")
	require.Equal(t, uint(256), *got.MaxTokens, "maxOutputTokens -> max_tokens")
	require.Equal(t, 2, *got.N, "candidateCount -> n")
	require.Equal(t, []string{"STOP"}, got.Stop, "stopSequences -> stop")

	// Not mapped, and that is the intended behaviour: OpenAI's response_format
	// is not interchangeable with Gemini's responseMimeType, and presence
	// penalty / seed have no converter today. They are dropped, never guessed.
	require.Nil(t, got.ResponseFormat, "responseMimeType is not response_format")
	require.Zero(t, got.PresencePenalty, "presencePenalty has no mapping yet")
	require.Zero(t, got.Seed, "seed has no mapping yet")
}

// systemInstruction is a top-level field outside contents; dropping it loses
// the system prompt entirely.
func TestGeminiToOpenAIRequest_SystemInstructionBecomesFirstMessage(t *testing.T) {
	req := geminiReq(dto.GeminiChatGenerationConfig{})
	req.SystemInstructions = &dto.GeminiChatContent{
		Parts: []dto.GeminiPart{{Text: "you are a translator"}},
	}
	got, err := GeminiToOpenAIRequest(req, geminiTestRelayInfo())
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)
	require.Equal(t, "system", got.Messages[0].Role)
	require.Contains(t, got.Messages[0].StringContent(), "you are a translator")
}

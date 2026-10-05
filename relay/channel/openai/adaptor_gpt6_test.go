package openai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// GPT-6 shares gpt-5's reasoning parameter contract: OpenAI rejects max_tokens
// and sampling knobs on it. Without the gpt-6 prefix the request is forwarded
// verbatim and the upstream answers 400.
func TestConvertOpenAIRequest_GPT6UsesReasoningParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, name := range []string{"gpt-6-astra", "gpt-6-sol", "gpt-6.1-sol"} {
		t.Run(name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			a := &Adaptor{ChannelType: constant.ChannelTypeOpenAI}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType:       constant.ChannelTypeOpenAI,
				UpstreamModelName: name,
			}}
			req := &dto.GeneralOpenAIRequest{
				Model:       name,
				MaxTokens:   lo.ToPtr(uint(16)),
				Temperature: lo.ToPtr(0.7),
				TopP:        lo.ToPtr(0.9),
				Messages:    []dto.Message{{Role: "user", Content: "hi"}},
			}

			got, err := a.ConvertOpenAIRequest(c, info, req)

			require.NoError(t, err)
			out := got.(*dto.GeneralOpenAIRequest)
			require.Nil(t, out.MaxTokens)
			require.EqualValues(t, 16, lo.FromPtr(out.MaxCompletionTokens))
			require.Nil(t, out.Temperature)
			require.Nil(t, out.TopP)
			require.Equal(t, "developer", out.GetSystemRoleName())
		})
	}
}

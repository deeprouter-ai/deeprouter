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

func topKInfo(channelType int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       channelType,
			UpstreamModelName: "gpt-4o-mini",
		},
	}
}

func topKRequest() *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model:    "gpt-4o-mini",
		TopK:     lo.ToPtr(40),
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
}

// Gemini and Claude both carry top_k and our converters put it in the neutral
// request; OpenAI has no such argument and answers 400 rather than ignoring it.
// One parameter took down the whole protocol, so this pins the drop per channel.
func TestConvertOpenAIRequest_DropsTopKForOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAzure} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		a := &Adaptor{ChannelType: channelType}

		got, err := a.ConvertOpenAIRequest(c, topKInfo(channelType), topKRequest())

		require.NoError(t, err)
		require.Nil(t, got.(*dto.GeneralOpenAIRequest).TopK,
			"channel type %d must not forward top_k", channelType)
	}
}

// The same adaptor serves OpenRouter and Xinference, which do accept top_k.
// Dropping it everywhere would silently degrade requests that were fine.
func TestConvertOpenAIRequest_KeepsTopKForCompatibleChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, channelType := range []int{constant.ChannelTypeOpenRouter, constant.ChannelTypeXinference} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		a := &Adaptor{ChannelType: channelType}

		got, err := a.ConvertOpenAIRequest(c, topKInfo(channelType), topKRequest())

		require.NoError(t, err)
		require.NotNil(t, got.(*dto.GeneralOpenAIRequest).TopK,
			"channel type %d accepts top_k and should keep it", channelType)
	}
}

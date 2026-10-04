package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestChannelTestImageModelsAutoRouteToImages(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	for _, name := range []string{"gpt-image-1", "gpt-image-1-mini", "gpt-image-1.5", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "dall-e-3"} {
		t.Run(name, func(t *testing.T) {
			endpoint := normalizeChannelTestEndpoint(channel, name, "")
			require.Equal(t, "image-generation", endpoint)
			image, ok := buildTestRequest(name, endpoint, channel, true).(*dto.ImageRequest)
			require.True(t, ok, "image model must be probed with an ImageRequest, not chat completions")
			require.Equal(t, name, image.Model)
		})
	}
	require.Equal(t, "openai-response", normalizeChannelTestEndpoint(channel, "o3-pro", ""))
	_, ok := buildTestRequest("gpt-4o-mini", "", channel, false).(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "plain chat models keep the chat-completions probe")
}

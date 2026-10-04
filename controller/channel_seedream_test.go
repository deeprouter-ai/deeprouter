package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/volcengine"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestSeedreamChannelTestImageContract(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeVolcEngine}
	for _, name := range []string{
		"doubao-seedream-4-0-250828", "doubao-seedream-4-5-251128",
		"doubao-seedream-5-0-260128", "doubao-seedream-5-0-pro-260628",
	} {
		t.Run(name, func(t *testing.T) {
			for _, override := range []string{"", "image-generation"} {
				endpoint := normalizeChannelTestEndpoint(channel, name, override)
				require.Equal(t, "image-generation", endpoint)
				path, ok := common.GetDefaultEndpointInfo(constant.EndpointType(endpoint))
				require.True(t, ok)
				require.Equal(t, "/v1/images/generations", path.Path)
				image, ok := buildTestRequest(name, override, channel, true).(*dto.ImageRequest)
				require.True(t, ok, "image route must receive an ImageRequest")
				require.Equal(t, "2048x2048", image.Size)
				require.Equal(t, name, image.Model)
				require.EqualValues(t, 1, *image.N)
				converted, err := (&volcengine.Adaptor{}).ConvertImageRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}, *image)
				require.NoError(t, err)
				require.NotNil(t, converted)
			}
		})
	}
}

func TestSeedreamChannelTestPreservesChatAndOverrides(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeVolcEngine}
	_, ok := buildTestRequest("doubao-seedream-4-5-251128", "openai", channel, false).(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	_, ok = buildTestRequest("doubao-seed-2-0-pro", "", channel, false).(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	image := buildTestRequest("dall-e-3", "image-generation", &model.Channel{Type: constant.ChannelTypeOpenAI}, false).(*dto.ImageRequest)
	require.Equal(t, "1024x1024", image.Size)
}

func TestChannelTestImageModelsAutoRouteToImages(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	for _, name := range []string{"gpt-image-1", "gpt-image-1-mini", "gpt-image-1.5", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "dall-e-3"} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, "image-generation", normalizeChannelTestEndpoint(channel, name, ""))
			image, ok := buildTestRequest(name, "", channel, true).(*dto.ImageRequest)
			require.True(t, ok, "image model must be probed with an ImageRequest, not chat completions")
			require.Equal(t, name, image.Model)
		})
	}
	require.Equal(t, "openai-response", normalizeChannelTestEndpoint(channel, "o3-pro", ""))
	_, ok := buildTestRequest("gpt-4o-mini", "", channel, false).(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "plain chat models keep the chat-completions probe")
}

func TestChannelTestModalityContract(t *testing.T) {
	for _, tc := range []struct {
		kind                     int
		name, override, endpoint string
	}{
		{constant.ChannelTypeMiniMax, "speech-02-hd", "", "audio-speech"},
		{constant.ChannelTypeMiniMax, "image-01", "", "image-generation"},
		{constant.ChannelTypeElevenLabs, "eleven_flash_v2_5", "openai", "audio-speech"},
		{constant.ChannelTypeOpenAI, "text-embed-v1", "", "embeddings"},
		{constant.ChannelTypeMokaAI, "custom-model", "", "embeddings"},
		{constant.ChannelTypeOpenAI, "bge-reranker", "", "jina-rerank"},
		{constant.ChannelTypeOpenAI, "gpt-5-codex", "", "openai-response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &model.Channel{Type: tc.kind}
			require.Equal(t, tc.endpoint, normalizeChannelTestEndpoint(channel, tc.name, tc.override))
			request := buildTestRequest(tc.name, tc.override, channel, true)
			switch tc.endpoint {
			case "audio-speech":
				audio, ok := request.(*dto.AudioRequest)
				require.True(t, ok)
				require.Equal(t, "mp3", audio.ResponseFormat)
				if tc.kind == constant.ChannelTypeMiniMax {
					require.NotEmpty(t, audio.Voice)
					require.Equal(t, 1.0, *audio.Speed)
				}
			case "image-generation":
				_, ok := request.(*dto.ImageRequest)
				require.True(t, ok)
			case "embeddings":
				_, ok := request.(*dto.EmbeddingRequest)
				require.True(t, ok)
			case "jina-rerank":
				_, ok := request.(*dto.RerankRequest)
				require.True(t, ok)
			case "openai-response":
				_, ok := request.(*dto.OpenAIResponsesRequest)
				require.True(t, ok)
			}
		})
	}
}

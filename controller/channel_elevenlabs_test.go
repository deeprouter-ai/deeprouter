package controller

import (
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/elevenlabs"
	"github.com/stretchr/testify/require"
)

func TestElevenLabsCatalogConnectionTestRequests(t *testing.T) {
	ch := &model.Channel{Type: constant.ChannelTypeElevenLabs}
	for _, modelID := range elevenlabs.ModelList {
		t.Run(modelID, func(t *testing.T) {
			req := buildTestRequest(modelID, "", ch, false)
			audio, ok := req.(*dto.AudioRequest)
			require.True(t, ok)
			require.Equal(t, modelID, audio.Model)
			converted, err := (&elevenlabs.Adaptor{}).ConvertAudioRequest(nil, nil, *audio)
			require.NoError(t, err)
			data, err := io.ReadAll(converted)
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, common.Unmarshal(data, &body))
			require.Equal(t, modelID, body["model_id"])
			if modelID == "eleven_v4" || modelID == "eleven_v4_turbo" || modelID == "eleven_v3_conversational" {
				require.NotEmpty(t, body["inputs"])
			} else {
				require.NotEmpty(t, body["text"])
			}
		})
	}
}

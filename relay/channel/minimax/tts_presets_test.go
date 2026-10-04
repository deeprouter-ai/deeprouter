package minimax

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTTSDefaultsAndBilledCharacters(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioSpeech}
	reader, err := (&Adaptor{}).ConvertAudioRequest(c, info, dto.AudioRequest{Model: "speech-2.8-turbo", Input: "你好"})
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload MiniMaxTTSRequest
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, "hex", payload.OutputFormat)
	require.Equal(t, "mp3", payload.AudioSetting.Format)
	require.Equal(t, 1.0, payload.VoiceSetting.Speed)
	require.NotEmpty(t, payload.VoiceSetting.VoiceID)
	require.Equal(t, "speech-2.8-turbo", payload.Model)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"data":{"audio":"00"},"extra_info":{"usage_characters":17},"base_resp":{"status_code":0}}`))}
	usage, apiErr := handleTTSResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.Equal(t, 17, usage.(*dto.Usage).PromptTokens)
	require.Equal(t, 17, usage.(*dto.Usage).TotalTokens)
}

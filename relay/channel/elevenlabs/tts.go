package elevenlabs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// convertTTSRequest maps the OpenAI-shaped AudioRequest to the ElevenLabs body.
// The voice id is carried in the URL (see Adaptor.GetRequestURL), so only
// text / model_id / voice_settings go in the body.
func convertTTSRequest(request dto.AudioRequest) (io.Reader, error) {
	modelID := request.Model
	if modelID == "" {
		modelID = defaultModelID
	}
	if modelID == "eleven_multilingual_v1" || modelID == "eleven_monolingual_v1" {
		return nil, fmt.Errorf("ElevenLabs model %s has been removed; use eleven_multilingual_v2, eleven_v3 or eleven_flash_v2_5", modelID)
	}
	if usesDialogueHTTP(modelID) || usesDialogueWebSocket(modelID) {
		voiceID := request.Voice
		if voiceID == "" {
			voiceID = defaultVoiceID
		}
		data, err := common.Marshal(dialogueRequest{
			ModelID: modelID,
			Inputs:  []dialogueInput{{Text: request.Input, VoiceID: voiceID}},
		})
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
	body := ttsRequest{
		Text:    request.Input,
		ModelID: modelID,
		VoiceSettings: &voiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
		},
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// elevenLabsTTSHandler streams the binary audio response back to the client and
// reports usage. ElevenLabs bills per INPUT CHARACTER, so we charge on the
// estimated prompt tokens (= character count of the input text) via the normal
// text-quota path — no audio-duration parsing needed, and CompletionTokens stay
// 0 so AudioHelper routes this to PostTextConsumeQuota.
func elevenLabsTTSHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) *dto.Usage {
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.TotalTokens = usage.PromptTokens

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}
	c.Writer.WriteHeader(resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.LogError(c, "failed to read ElevenLabs TTS response: "+err.Error())
		c.Writer.WriteHeaderNow()
		return usage
	}
	c.Writer.WriteHeaderNow()
	if _, werr := c.Writer.Write(bodyBytes); werr != nil {
		logger.LogError(c, "failed to write ElevenLabs TTS response: "+werr.Error())
	}
	return usage
}

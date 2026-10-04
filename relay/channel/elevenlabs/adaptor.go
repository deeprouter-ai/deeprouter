package elevenlabs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

var errNotSupported = errors.New("elevenlabs channel only supports text-to-speech (/v1/audio/speech)")

// Adaptor implements channel.Adaptor for ElevenLabs TTS. Only the audio-speech
// relay mode is handled; every other modality returns errNotSupported.
type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return "", errNotSupported
	}
	voiceID := defaultVoiceID
	modelID := info.UpstreamModelName
	if modelID == "" {
		if req, ok := info.Request.(*dto.AudioRequest); ok {
			modelID = req.Model
		}
	}
	if usesDialogueHTTP(modelID) {
		return strings.TrimRight(info.ChannelBaseUrl, "/") + "/v1/text-to-dialogue", nil
	}
	if usesDialogueWebSocket(modelID) {
		return dialogueWebSocketURL(info.ChannelBaseUrl, modelID)
	}
	if req, ok := info.Request.(*dto.AudioRequest); ok && req.Voice != "" {
		voiceID = req.Voice
	}
	return fmt.Sprintf("%s/v1/text-to-speech/%s", strings.TrimRight(info.ChannelBaseUrl, "/"), url.PathEscape(voiceID)), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("xi-api-key", info.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return convertTTSRequest(request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	data, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, err
	}
	var body struct {
		ModelID string `json:"model_id"`
	}
	if err := common.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	if usesDialogueWebSocket(body.ModelID) {
		return a.doDialogueWebSocket(c, info, data)
	}
	return channel.DoApiRequest(a, c, info, bytes.NewReader(data))
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, types.NewError(errNotSupported, types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	return elevenLabsTTSHandler(c, resp, info), nil
}

func (a *Adaptor) GetModelList() []string { return ModelList }
func (a *Adaptor) GetChannelName() string { return ChannelName }

// ── Unsupported modalities (ElevenLabs is TTS-only) ───────────────────────

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errNotSupported
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errNotSupported
}

package elevenlabs

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func speechTestContext(ctx context.Context, baseURL, modelID string) (*gin.Context, *relaycommon.RelayInfo, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		Request:     &dto.AudioRequest{Model: modelID, Input: "A short speech.", Voice: "VOICE123"},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: baseURL, ApiKey: "test-only"},
	}, w
}

func TestEveryCatalogModelRoutesAndReturnsAudio(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	ratio_setting.InitRatioSettings()
	for _, modelID := range ModelList {
		t.Run(modelID, func(t *testing.T) {
			ratio, ok, _ := ratio_setting.GetModelRatio(modelID)
			require.True(t, ok, "advertised speech model must have billing configured")
			require.Positive(t, ratio)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "test-only", r.Header.Get("xi-api-key"))
				if usesDialogueWebSocket(modelID) {
					require.Equal(t, "/v1/text-to-dialogue/stream-input", r.URL.Path)
					require.Equal(t, modelID, r.URL.Query().Get("model_id"))
					require.Equal(t, "mp3_44100_128", r.URL.Query().Get("output_format"))
					conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
					require.NoError(t, err)
					defer conn.Close()
					var setup struct {
						Voices []string `json:"voices"`
					}
					_, b, err := conn.ReadMessage()
					require.NoError(t, err)
					require.NoError(t, common.Unmarshal(b, &setup))
					require.Equal(t, []string{"VOICE123"}, setup.Voices)
					_, b, err = conn.ReadMessage()
					require.NoError(t, err)
					var input dialogueRequest
					require.NoError(t, common.Unmarshal(b, &input))
					require.Equal(t, []dialogueInput{{Text: "A short speech.", VoiceID: "VOICE123"}}, input.Inputs)
					_, b, err = conn.ReadMessage()
					require.NoError(t, err)
					require.JSONEq(t, `{"close_socket":true}`, string(b))
					// A turn marker is not the session-final marker; both chunks matter.
					require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"audio":"`+base64.StdEncoding.EncodeToString([]byte("audio-"))+`"}`)))
					require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"is_final_audio_for_turn":true}`)))
					require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"audio":"`+base64.StdEncoding.EncodeToString([]byte("bytes"))+`","is_final":true}`)))
					return
				}
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				require.Equal(t, modelID, body["model_id"])
				if usesDialogueHTTP(modelID) {
					require.Equal(t, "/v1/text-to-dialogue", r.URL.Path)
					require.Nil(t, body["text"])
					require.Equal(t, []any{map[string]any{"text": "A short speech.", "voice_id": "VOICE123"}}, body["inputs"])
				} else {
					require.Equal(t, "/v1/text-to-speech/VOICE123", r.URL.Path)
					require.Equal(t, "A short speech.", body["text"])
				}
				w.Header().Set("Content-Type", "audio/mpeg")
				_, _ = w.Write([]byte("audio-bytes"))
			}))
			defer server.Close()
			c, info, recorder := speechTestContext(context.Background(), server.URL, modelID)
			adapter := &Adaptor{}
			body, err := adapter.ConvertAudioRequest(c, info, *info.Request.(*dto.AudioRequest))
			require.NoError(t, err)
			result, err := adapter.DoRequest(c, info, body)
			require.NoError(t, err)
			response := result.(*http.Response)
			require.Equal(t, http.StatusOK, response.StatusCode)
			_, apiErr := adapter.DoResponse(c, response, info)
			require.Nil(t, apiErr)
			require.Equal(t, "audio-bytes", recorder.Body.String())
			require.Equal(t, "audio/mpeg", recorder.Header().Get("Content-Type"))
		})
	}
}

func TestDialogueWebSocketFailuresAreNotSuccessfulAudio(t *testing.T) {
	for _, tc := range []struct{ name, frame string }{
		{"upstream-error", `{"error":"unsupported_model","error_code":"unsupported_model"}`},
		{"invalid-json", `{`},
		{"invalid-audio", `{"audio":"invalid base64","is_final":true}`},
		{"empty-audio", `{"is_final":true}`},
		{"truncated-audio", `{"audio":"YXVkaW8="}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				require.NoError(t, err)
				defer conn.Close()
				for i := 0; i < 3; i++ {
					_, _, err := conn.ReadMessage()
					require.NoError(t, err)
				}
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(tc.frame)))
			}))
			defer server.Close()
			c, info, _ := speechTestContext(context.Background(), server.URL, "eleven_v3_conversational")
			a := &Adaptor{}
			body, err := a.ConvertAudioRequest(c, info, *info.Request.(*dto.AudioRequest))
			require.NoError(t, err)
			response, err := a.DoRequest(c, info, body)
			if tc.name == "upstream-error" {
				require.NoError(t, err)
				resp := response.(*http.Response)
				defer resp.Body.Close()
				require.Equal(t, http.StatusBadGateway, resp.StatusCode)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestDialogueWebSocketHandshakePreservesUpstreamStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"detail":{"status":"invalid_api_key"}}`)
	}))
	defer server.Close()
	c, info, _ := speechTestContext(context.Background(), server.URL, "eleven_v3_conversational")
	a := &Adaptor{}
	body, err := a.ConvertAudioRequest(c, info, *info.Request.(*dto.AudioRequest))
	require.NoError(t, err)
	r, err := a.DoRequest(c, info, body)
	require.NoError(t, err)
	resp := r.(*http.Response)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDialogueWebSocketCancellationClosesUpstream(t *testing.T) {
	connected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		close(connected)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, info, _ := speechTestContext(ctx, server.URL, "eleven_v3_conversational")
	a := &Adaptor{}
	body, err := a.ConvertAudioRequest(c, info, *info.Request.(*dto.AudioRequest))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := a.DoRequest(c, info, body); done <- err }()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("not connected")
	}
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not stop generation")
	}
}

func TestMappedDialogueModelChoosesCorrectProtocol(t *testing.T) {
	_, info, _ := speechTestContext(context.Background(), "https://api.elevenlabs.io", "public-alias")
	info.UpstreamModelName = "eleven_v4"
	u, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.elevenlabs.io/v1/text-to-dialogue", u)
	info.UpstreamModelName = "eleven_v4_turbo"
	u, err = (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u, "wss://api.elevenlabs.io/v1/text-to-dialogue/stream-input?"))
}

func TestRemovedModelsRejectedBeforeProviderCall(t *testing.T) {
	for _, modelID := range []string{"eleven_multilingual_v1", "eleven_monolingual_v1"} {
		_, err := convertTTSRequest(dto.AudioRequest{Model: modelID, Input: "hello"})
		require.ErrorContains(t, err, "has been removed")
	}
}

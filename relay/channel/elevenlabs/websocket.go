package elevenlabs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func dialogueWebSocketURL(baseURL, modelID string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/v1/text-to-dialogue/stream-input")
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("ElevenLabs base URL must use http or https")
	}
	q := u.Query()
	q.Set("model_id", modelID)
	q.Set("output_format", "mp3_44100_128")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Bridge one complete audio/speech request to the upstream dialogue WebSocket.
// Wait for the final frame before returning a successful HTTP response so a
// failed or truncated generation is not counted as a successful channel test.
func (a *Adaptor) doDialogueWebSocket(c *gin.Context, info *relaycommon.RelayInfo, data []byte) (*http.Response, error) {
	var req dialogueRequest
	if err := common.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	if len(req.Inputs) != 1 {
		return nil, fmt.Errorf("ElevenLabs audio/speech requires one dialogue input")
	}
	u, err := dialogueWebSocketURL(info.ChannelBaseUrl, req.ModelID)
	if err != nil {
		return nil, err
	}
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, fmt.Errorf("invalid ElevenLabs channel proxy")
	}
	dialer := *websocket.DefaultDialer
	// Reuse channel proxy, TLS and SOCKS transport settings for the handshake.
	if client != nil {
		if transport, ok := client.Transport.(*http.Transport); ok {
			dialer.Proxy = transport.Proxy
			dialer.NetDialContext = transport.DialContext
			dialer.TLSClientConfig = transport.TLSClientConfig
		}
	}
	header := http.Header{}
	if err := a.SetupRequestHeader(c, &header, info); err != nil {
		return nil, err
	}
	timeout := 180 * time.Second
	if common.RelayTimeout > 0 {
		timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	conn, resp, err := dialer.DialContext(ctx, u, header)
	if err != nil {
		// Preserve upstream HTTP status/errors (auth, unsupported model, etc.).
		if resp != nil {
			return resp, nil
		}
		return nil, fmt.Errorf("ElevenLabs dialogue handshake failed: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	_ = conn.SetReadDeadline(deadline)
	_ = conn.SetWriteDeadline(deadline)
	conn.SetReadLimit(8 << 20)
	frames := []any{
		struct {
			Voices []string `json:"voices"`
		}{[]string{req.Inputs[0].VoiceID}},
		struct {
			Inputs []dialogueInput `json:"inputs"`
		}{req.Inputs},
		struct {
			CloseSocket bool `json:"close_socket"`
		}{true},
	}
	for _, frame := range frames {
		b, err := common.Marshal(frame)
		if err != nil {
			return nil, err
		}
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return nil, fmt.Errorf("ElevenLabs dialogue send failed: %w", err)
		}
	}
	var audio bytes.Buffer
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("ElevenLabs dialogue ended before final audio: %w", err)
		}
		var frame struct {
			Audio     string          `json:"audio"`
			IsFinal   bool            `json:"is_final"`
			Error     json.RawMessage `json:"error"`
			ErrorCode json.RawMessage `json:"error_code"`
		}
		if err := common.Unmarshal(message, &frame); err != nil {
			return nil, fmt.Errorf("invalid ElevenLabs dialogue frame: %w", err)
		}
		if (len(frame.Error) > 0 && string(frame.Error) != "null") || (len(frame.ErrorCode) > 0 && string(frame.ErrorCode) != "null") {
			return audioHTTPResponse(http.StatusBadGateway, "application/json", message), nil
		}
		if frame.Audio != "" {
			chunk, err := base64.StdEncoding.DecodeString(frame.Audio)
			if err != nil {
				return nil, fmt.Errorf("invalid ElevenLabs audio encoding: %w", err)
			}
			if audio.Len()+len(chunk) > 64<<20 {
				return nil, fmt.Errorf("ElevenLabs dialogue audio exceeds 64 MiB")
			}
			_, _ = audio.Write(chunk)
		}
		if frame.IsFinal {
			if audio.Len() == 0 {
				return nil, fmt.Errorf("ElevenLabs dialogue returned no audio")
			}
			return audioHTTPResponse(http.StatusOK, "audio/mpeg", audio.Bytes()), nil
		}
	}
}

func audioHTTPResponse(status int, contentType string, data []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
	}
}

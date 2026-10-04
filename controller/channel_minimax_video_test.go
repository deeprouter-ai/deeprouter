package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxVideoChannelReadOnlyTest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"valid", 200, `{"items":[],"total":0}`, false},
		{"auth", 401, `{"base_resp":{"status_code":1004}}`, true},
		{"error on 200", 200, `{"items":[],"total":0,"base_resp":{"status_code":1004}}`, true},
		{"wrong API", 200, `{"success":true}`, true},
		{"HTML", 200, `<html>login</html>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v2/query/video_generation", r.URL.Path)
				require.Equal(t, "1", r.URL.Query().Get("page_size"))
				require.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			base, testModel := upstream.URL, "MiniMax-H3"
			channel := &model.Channel{Type: constant.ChannelTypeMiniMax, Key: "fixture-key", BaseURL: &base, Models: "MiniMax-H3", TestModel: &testModel}
			// Chat and stream overrides must not turn a video probe into generation.
			result := testChannel(channel, "", "openai", true)
			if tc.wantErr {
				require.Error(t, result.localErr)
			} else {
				require.NoError(t, result.localErr)
			}
			require.Equal(t, 1, calls)
			require.Equal(t, "connection_only", channelTestScope(channel))
		})
	}
	require.False(t, isMiniMaxVideoTest(&model.Channel{Type: 35, Models: "speech-2.8-hd"}, ""))
	require.False(t, isMiniMaxVideoTest(&model.Channel{Type: 35, Models: "MiniMax-M3"}, ""))
}

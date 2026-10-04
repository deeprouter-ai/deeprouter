package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSeedanceChannelTestUsesReadOnlyTaskAPI(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"empty list", 200, `{"items":[],"total":0}`, false},
		{"existing task", 200, `{"items":[{"id":"task-1"}],"total":1}`, false},
		{"invalid key", 401, `{"error":{"code":"AuthenticationError"}}`, true},
		{"no permission", 403, `{"error":{"code":"AccessDenied"}}`, true},
		{"rate limit", 429, `{"error":{"code":"RateLimit"}}`, true},
		{"wrong endpoint", 404, `{}`, true},
		{"server failure", 500, `{}`, true},
		{"HTML", 200, `<html>login</html>`, true},
		{"other JSON API", 200, `{"success":true}`, true},
		{"missing total", 200, `{"items":[]}`, true},
		{"null items", 200, `{"items":null,"total":0}`, true},
		{"error with HTTP 200", 200, `{"items":[],"total":0,"error":{"code":"Denied"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/api/v3/contents/generations/tasks", r.URL.Path)
				require.Equal(t, "1", r.URL.Query().Get("page_size"))
				require.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			base := upstream.URL
			channel := &model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "fixture-key", BaseURL: &base}
			// Exercise the real dispatcher, including ignored chat/stream overrides.
			result := testChannel(channel, "doubao-seedance-2-5-260628", "openai", true)
			if tc.wantErr {
				require.Error(t, result.localErr)
			} else {
				require.NoError(t, result.localErr)
			}
			require.Equal(t, 1, calls)
			require.Equal(t, "connection_only", channelTestScope(channel))
		})
	}
}

func TestSeedanceChannelTestMissingKey(t *testing.T) {
	result := testChannel(&model.Channel{Type: constant.ChannelTypeDoubaoVideo}, "", "", false)
	require.ErrorContains(t, result.localErr, "enabled API key")
}

package channeltest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedanceConnectionRejectsRedirectAndOversizedBody(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	require.ErrorContains(t, SeedanceConnection(nil, redirect.URL, "fixture-key"), "HTTP 307")
	require.Zero(t, redirected.Load())
	oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 1024*1024+1)))
	}))
	defer oversized.Close()
	require.ErrorContains(t, SeedanceConnection(nil, oversized.URL, "fixture-key"), "could not read")
}

func TestSeedanceConnectionRedactsProviderError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"fixture-key"}}`))
	}))
	defer upstream.Close()
	err := SeedanceConnection(nil, upstream.URL, "fixture-key")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "fixture-key")
	require.Contains(t, err.Error(), "[redacted]")
}

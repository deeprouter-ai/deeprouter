package smart_router_client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// recordingServer answers /route with reply and keeps the last request body.
func recordingServer(t *testing.T, status int, reply string) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestExplain_AsksWithExplainAndParsesBothReplyShapes(t *testing.T) {
	srv, sent := recordingServer(t, http.StatusOK, `{"primary":"claude-sonnet-5","fallback_chain":["gpt-4o"],"reason":"jev:code_hard",
		"explain":{"jev_status":"ok","jev_ms":287,"task":"code","difficulty":4,"rule":"jev_code_hard","proposed_primary":"claude-sonnet-5"}}`)
	c := NewClient(srv.URL, 100*time.Millisecond)

	res, err := c.Explain(context.Background(), "7", "design a lock-free map")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"explain":true`, `"tenant_id":"7"`, `"role":"user"`, `design a lock-free map`} {
		if !strings.Contains(*sent, want) {
			t.Errorf("request %s lacks %s", *sent, want)
		}
	}
	if res.Primary != "claude-sonnet-5" || res.Explain == nil || res.Explain.Task != "code" || res.Explain.Difficulty != 4 || res.Explain.JevMs != 287 {
		t.Errorf("parsed %+v / %+v", res, res.Explain)
	}

	srv2, _ := recordingServer(t, http.StatusOK, `{"error":"no_model_matches_constraints","fallback_to_default":"gpt-4o-mini",
		"explain":{"jev_status":"no_key","jev_ms":0,"rule":"long_context","proposed_primary":"claude-sonnet-5"}}`)
	res, err = NewClient(srv2.URL, 0).Explain(context.Background(), "7", "x")
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "no_model_matches_constraints" || res.FallbackToDefault != "gpt-4o-mini" || res.Explain.ProposedPrimary != "claude-sonnet-5" {
		t.Errorf("no-model reply parsed as %+v", res)
	}
}

func TestExplain_ReportsErrorsInsteadOfFallingBack(t *testing.T) {
	if _, err := NewClient("", 0).Explain(context.Background(), "1", "x"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no URL: err = %v, want ErrNotConfigured", err)
	}
	srv, _ := recordingServer(t, http.StatusInternalServerError, "boom")
	if _, err := NewClient(srv.URL, 0).Explain(context.Background(), "1", "x"); err == nil {
		t.Error("a 500 must come back as an error for the admin to see")
	}
}

func TestExplain_LeavesLiveRoutingAlone(t *testing.T) {
	srv, _ := recordingServer(t, http.StatusInternalServerError, "boom")
	c := NewClient(srv.URL, 100*time.Millisecond)
	for i := 0; i < breakerThreshold+1; i++ {
		_, _ = c.Explain(context.Background(), "1", "x")
	}
	if c.breakerOpen() {
		t.Error("failed tests must not open the breaker that live traffic uses")
	}

	ok, _ := recordingServer(t, http.StatusOK, `{"primary":"gpt-4o","explain":{"jev_status":"off","rule":"short","proposed_primary":"gpt-4o"}}`)
	c = NewClient(ok.URL, 100*time.Millisecond)
	c.breakerUntil = time.Now().Add(time.Minute)
	if res, err := c.Explain(context.Background(), "1", "x"); err != nil || res.Primary != "gpt-4o" {
		t.Errorf("with the breaker open the test should still reach smart-router: %v %+v", err, res)
	}
}

func TestRoute_NeverAsksForExplain(t *testing.T) {
	srv, sent := recordingServer(t, http.StatusOK, `{"primary":"gpt-4o"}`)
	if _, err := NewClient(srv.URL, 100*time.Millisecond).Route(context.Background(), RouteRequest{TenantID: "1", Messages: []Message{{Role: "user", Content: "hi"}}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(*sent, "explain") {
		t.Errorf("live routing requests must not carry explain: %s", *sent)
	}
}

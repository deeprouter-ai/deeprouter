package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignPayload_StableAndVerifiable(t *testing.T) {
	payload := []byte(`{"request_id":"r1","stars":3}`)
	secret := []byte("test-secret")

	sig := SignPayload(payload, secret)

	// Independent verification
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if sig != expected {
		t.Errorf("SignPayload mismatch: got %q, want %q", sig, expected)
	}
}

func TestDispatcher_Send_Success(t *testing.T) {
	var hits int32
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		gotSig = r.Header.Get("X-DeepRouter-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.Client.Timeout = 2 * time.Second
	status, err := d.Send(srv.URL, []byte("s"), &Event{RequestID: "r1", Stars: 3, Provider: "openai", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
	if gotSig == "" {
		t.Errorf("expected signature header set")
	}
}

func TestDispatcher_Send_RetriesOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.MaxRetries = 3
	d.Client.Timeout = 2 * time.Second
	status, err := d.Send(srv.URL, []byte("s"), &Event{RequestID: "r2"})
	if err != nil {
		t.Fatalf("expected eventual success, got error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected final 200, got %d", status)
	}
	if atomic.LoadInt32(&hits) < 3 {
		t.Errorf("expected at least 3 attempts, got %d", hits)
	}
}

func TestDispatcher_Send_StopsOn4xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	d := NewDispatcher()
	d.MaxRetries = 5
	d.Client.Timeout = 2 * time.Second
	status, err := d.Send(srv.URL, []byte("s"), &Event{RequestID: "r3"})
	if err == nil {
		t.Fatalf("expected error for 400, got nil")
	}
	if status != 400 {
		t.Errorf("expected status 400, got %d", status)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 hit (no retries on 4xx), got %d", got)
	}
}

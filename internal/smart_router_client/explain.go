package smart_router_client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// explainTimeout bounds one admin test: smart-router's 2s first catalog fetch plus a slow Jev call.
const explainTimeout = 3 * time.Second

// ErrNotConfigured means SMART_ROUTER_URL is empty, so there is no smart-router to test.
var ErrNotConfigured = errors.New("smart-router is not configured")

// explainHTTP is separate from the routing client so a test may wait longer than live traffic does.
var explainHTTP = &http.Client{Timeout: explainTimeout}

// Explanation mirrors smart-router's "explain" reply object (smart-router docs/PRD.md §6.1).
type Explanation struct {
	JevStatus       string `json:"jev_status"`
	JevMs           int64  `json:"jev_ms"`
	Task            string `json:"task,omitempty"`
	Difficulty      int    `json:"difficulty,omitempty"`
	Rule            string `json:"rule"`
	ProposedPrimary string `json:"proposed_primary"`
}

// ExplainResult is smart-router's answer to one test: a decision or the reason there is none, plus how it got there.
type ExplainResult struct {
	Primary           string       `json:"primary,omitempty"`
	FallbackChain     []string     `json:"fallback_chain,omitempty"`
	Reason            string       `json:"reason,omitempty"`
	Error             string       `json:"error,omitempty"`
	FallbackToDefault string       `json:"fallback_to_default,omitempty"`
	Explain           *Explanation `json:"explain,omitempty"`
}

// Explain asks smart-router how it would route text for tenantID, for the admin test panel.
// Unlike Route it returns errors, skips the circuit breaker and may take explainTimeout,
// so a test never changes how live traffic is routed.
func (c *Client) Explain(ctx context.Context, tenantID, text string) (*ExplainResult, error) {
	if !c.Enabled() {
		return nil, ErrNotConfigured
	}
	body, err := common.Marshal(RouteRequest{
		TenantID: tenantID,
		Messages: []Message{{Role: "user", Content: text}},
		Explain:  true,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/route", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := explainHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("smart-router status=%d", resp.StatusCode)
	}
	out := &ExplainResult{}
	if err := common.DecodeJson(resp.Body, out); err != nil {
		return nil, err
	}
	return out, nil
}

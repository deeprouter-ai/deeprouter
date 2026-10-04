package channeltest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// MiniMaxVideoConnection probes authentication and connectivity, never model generation.
// Listing tasks is read-only, so automatic channel tests cannot create paid videos.
func MiniMaxVideoConnection(client *http.Client, baseURL, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("MiniMax video connection test requires an enabled API key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := strings.TrimRight(baseURL, "/") + "/v2/query/video_generation?page_num=1&page_size=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("invalid MiniMax video base URL")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if client == nil {
		client = http.DefaultClient
	}
	probeClient := *client
	probeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := probeClient.Do(req)
	if err != nil {
		return fmt.Errorf("MiniMax video connection request failed: %s", strings.ReplaceAll(err.Error(), key, "[redacted]"))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return fmt.Errorf("could not read MiniMax video connection response")
	}
	if resp.StatusCode != http.StatusOK {
		var upstream struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = common.Unmarshal(body, &upstream)
		return fmt.Errorf("MiniMax video connection test failed (HTTP %d, code %s)", resp.StatusCode, strings.ReplaceAll(upstream.Error.Code, key, "[redacted]"))
	}
	var result struct {
		Items    *[]map[string]any `json:"items"`
		Total    *int              `json:"total"`
		Error    any               `json:"error"`
		BaseResp struct {
			StatusCode int `json:"status_code"`
		} `json:"base_resp"`
	}
	if err := common.Unmarshal(body, &result); err != nil || result.Items == nil || result.Total == nil || *result.Total < 0 || result.Error != nil || result.BaseResp.StatusCode != 0 {
		return fmt.Errorf("invalid MiniMax video task-list response; check the channel base URL")
	}
	return nil
}

// IsMiniMaxVideoModel identifies the models handled by the Hailuo task adaptor.
func IsMiniMaxVideoModel(model string) bool {
	name := strings.ToLower(strings.TrimSpace(model))
	for _, prefix := range []string{"minimax-h", "t2v-", "i2v-", "s2v-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

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

// SeedanceConnection probes authentication and connectivity, never model generation.
// Listing tasks is read-only, so automatic channel tests cannot create paid videos.
func SeedanceConnection(client *http.Client, baseURL, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("Seedance connection test requires an enabled API key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := strings.TrimRight(baseURL, "/") + "/api/v3/contents/generations/tasks?page_num=1&page_size=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("invalid Seedance base URL")
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
		return fmt.Errorf("Seedance connection request failed: %s", strings.ReplaceAll(err.Error(), key, "[redacted]"))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return fmt.Errorf("could not read Seedance connection response")
	}
	if resp.StatusCode != http.StatusOK {
		var upstream struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = common.Unmarshal(body, &upstream)
		return fmt.Errorf("Seedance connection test failed (HTTP %d, code %s)", resp.StatusCode, strings.ReplaceAll(upstream.Error.Code, key, "[redacted]"))
	}
	var result struct {
		Items *[]map[string]any `json:"items"`
		Total *int              `json:"total"`
		Error any               `json:"error"`
	}
	if err := common.Unmarshal(body, &result); err != nil || result.Items == nil || result.Total == nil || *result.Total < 0 || result.Error != nil {
		return fmt.Errorf("invalid Seedance task-list response; check the channel base URL")
	}
	return nil
}

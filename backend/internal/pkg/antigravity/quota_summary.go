package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
)

// quotaSummaryBodyLimit bounds the retrieveUserQuotaSummary response (a few hundred bytes today).
const quotaSummaryBodyLimit = 1 << 20

// QuotaSummaryBucket is one quota window of a group, e.g. "gemini-5h" or "gemini-weekly".
type QuotaSummaryBucket struct {
	BucketID          string  `json:"bucketId"`
	DisplayName       string  `json:"displayName,omitempty"`
	Window            string  `json:"window"` // "5h" or "weekly"
	ResetTime         string  `json:"resetTime,omitempty"`
	Description       string  `json:"description,omitempty"`
	RemainingFraction float64 `json:"remainingFraction"`
}

// QuotaSummaryGroup is a set of models that share their quota windows.
type QuotaSummaryGroup struct {
	DisplayName string               `json:"displayName"`
	Description string               `json:"description,omitempty"`
	Buckets     []QuotaSummaryBucket `json:"buckets"`
}

// QuotaSummaryResponse is the retrieveUserQuotaSummary response.
//
// Unlike fetchAvailableModels, which reports one fraction per model (whichever window is
// tighter), this reports every window of every group, so the 5-hour and the weekly limit can
// be told apart. Within a group all models share both windows.
type QuotaSummaryResponse struct {
	Groups      []QuotaSummaryGroup `json:"groups"`
	Description string              `json:"description,omitempty"`
}

// Keep the custom response names compatible with the shared upstream representation.
type UserQuotaBucket = QuotaSummaryBucket
type UserQuotaGroup = QuotaSummaryGroup
type RetrieveUserQuotaSummaryResponse = QuotaSummaryResponse

// RetrieveUserQuotaSummary reads the account's per-group 5-hour and weekly quota windows.
// Preserve the custom daily → sandbox → prod fallback even when an endpoint does not
// support the summary API. An optional positive body limit overrides the default.
func (c *Client) RetrieveUserQuotaSummary(ctx context.Context, accessToken, projectID string, bodyLimits ...int64) (*QuotaSummaryResponse, error) {
	bodyLimit := int64(quotaSummaryBodyLimit)
	if len(bodyLimits) > 0 && bodyLimits[0] > 0 {
		bodyLimit = bodyLimits[0]
	}
	endpoints := []string{
		antigravityDailyBaseURL,
		"https://daily-cloudcode-pa.sandbox.googleapis.com",
		antigravityProdBaseURL,
	}
	var lastErr error
	for _, baseURL := range endpoints {
		summary, _, err := c.retrieveUserQuotaSummary(ctx, baseURL, accessToken, projectID, bodyLimit)
		if err == nil {
			DefaultURLAvailability.MarkSuccess(baseURL)
			return summary, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// RetrieveUserQuotaSummaryFromURL reads the quota summary from one endpoint, without fallback.
func (c *Client) RetrieveUserQuotaSummaryFromURL(ctx context.Context, baseURL, accessToken, projectID string) (*QuotaSummaryResponse, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("retrieveUserQuotaSummary base URL is required")
	}
	summary, _, err := c.retrieveUserQuotaSummary(ctx, baseURL, accessToken, projectID, quotaSummaryBodyLimit)
	return summary, err
}

func (c *Client) retrieveUserQuotaSummary(ctx context.Context, baseURL, accessToken, projectID string, bodyLimit int64) (*QuotaSummaryResponse, int, error) {
	if c == nil || c.httpClient == nil {
		return nil, 0, errors.New("antigravity client is not configured")
	}
	bodyBytes, err := json.Marshal(FetchAvailableModelsRequest{Project: projectID})
	if err != nil {
		return nil, 0, fmt.Errorf("marshal retrieveUserQuotaSummary request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:retrieveUserQuotaSummary", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, 0, fmt.Errorf("create retrieveUserQuotaSummary request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", GetUserAgentForContext(ctx))

	resp, err := servertiming.Do(c.fetchAvailableModelsHTTPClient(), req)
	if err != nil {
		return nil, 0, fmt.Errorf("retrieveUserQuotaSummary request failed: %w", err)
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	_ = resp.Body.Close()
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read retrieveUserQuotaSummary response: %w", err)
	}
	if int64(len(respBody)) > bodyLimit {
		return nil, resp.StatusCode, fmt.Errorf("retrieveUserQuotaSummary response exceeds %d bytes", bodyLimit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("retrieveUserQuotaSummary failed (HTTP %d): %s", resp.StatusCode, truncateQuotaSummaryBody(respBody))
	}
	var summary QuotaSummaryResponse
	if err := json.Unmarshal(respBody, &summary); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse retrieveUserQuotaSummary response: %w", err)
	}
	return &summary, resp.StatusCode, nil
}

func truncateQuotaSummaryBody(body []byte) string {
	const max = 300
	if len(body) > max {
		return string(body[:max]) + "..."
	}
	return string(body)
}

//go:build unit

package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The shape Google returned for a consumer Ultra account on 2026-11-04.
const quotaSummaryBody = `{
  "groups": [
    {"buckets": [
       {"bucketId": "gemini-weekly", "displayName": "Weekly Limit Remaining", "window": "weekly",
        "resetTime": "2026-11-11T15:52:48Z", "remainingFraction": 0.75},
       {"bucketId": "gemini-5h", "displayName": "Five Hour Limit Remaining", "window": "5h",
        "resetTime": "2026-11-04T20:52:48Z", "remainingFraction": 0.4}],
     "displayName": "Gemini Models", "description": "Models within this group: Gemini Flash, Gemini Pro"},
    {"buckets": [
       {"bucketId": "3p-weekly", "window": "weekly", "resetTime": "2026-11-11T15:52:48Z", "remainingFraction": 1},
       {"bucketId": "3p-5h", "window": "5h", "remainingFraction": 1}],
     "displayName": "Claude and GPT models"}
  ],
  "description": "Within each group, models share a weekly limit and a 5-hour limit."
}`

func TestClient_RetrieveUserQuotaSummaryFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1internal:retrieveUserQuotaSummary" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("unexpected authorization header")
		}
		var body FetchAvailableModelsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Project != "proj" {
			t.Errorf("unexpected project: %+v, %v", body, err)
		}
		_, _ = w.Write([]byte(quotaSummaryBody))
	}))
	defer server.Close()

	client := mustNewClient(t, "")
	summary, err := client.RetrieveUserQuotaSummaryFromURL(context.Background(), server.URL, "tok", "proj")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Groups) != 2 || summary.Groups[0].DisplayName != "Gemini Models" {
		t.Fatalf("groups: %+v", summary.Groups)
	}
	weekly := summary.Groups[0].Buckets[0]
	if weekly.BucketID != "gemini-weekly" || weekly.Window != "weekly" || weekly.RemainingFraction != 0.75 ||
		weekly.ResetTime != "2026-11-11T15:52:48Z" {
		t.Fatalf("weekly bucket: %+v", weekly)
	}
	if summary.Groups[1].Buckets[1].ResetTime != "" {
		t.Fatalf("a bucket without resetTime keeps it empty")
	}
}

func TestClient_RetrieveUserQuotaSummaryFromURL_Errors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{http.StatusForbidden, `{"error":{"code":403}}`}, {http.StatusInternalServerError, `{}`}, {http.StatusOK, `not json`}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		client := mustNewClient(t, "")
		if _, err := client.RetrieveUserQuotaSummaryFromURL(context.Background(), server.URL, "tok", "proj"); err == nil {
			t.Errorf("status %d body %q must be an error", tc.status, tc.body)
		}
		server.Close()
	}
}

func TestClient_RetrieveUserQuotaSummary_CustomFallback(t *testing.T) {
	originalAvailability := DefaultURLAvailability
	DefaultURLAvailability = NewURLAvailability(URLAvailabilityTTL)
	t.Cleanup(func() { DefaultURLAvailability = originalAvailability })
	var requested []string
	client := &Client{httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.Scheme+"://"+req.URL.Host)
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/v1internal:retrieveUserQuotaSummary", req.URL.Path)
		require.Equal(t, "Bearer tok", req.Header.Get("Authorization"))
		var body FetchAvailableModelsRequest
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.Equal(t, "proj", body.Project)
		if len(requested) == 1 {
			return nil, errors.New("daily endpoint unavailable")
		}
		status, responseBody := http.StatusOK, quotaSummaryBody
		if len(requested) == 2 {
			status, responseBody = http.StatusForbidden, `{"error":"summary not available here"}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    req,
		}, nil
	})}}

	summary, err := client.RetrieveUserQuotaSummary(context.Background(), "tok", "proj")
	require.NoError(t, err)
	require.Len(t, summary.Groups, 2)
	require.Equal(t, []string{
		antigravityDailyBaseURL,
		"https://daily-cloudcode-pa.sandbox.googleapis.com",
		antigravityProdBaseURL,
	}, requested)
	require.Equal(t, antigravityProdBaseURL, DefaultURLAvailability.GetAvailableURLs()[0])
}

func TestClient_RetrieveUserQuotaSummary_BodyLimit(t *testing.T) {
	originalAvailability := DefaultURLAvailability
	DefaultURLAvailability = NewURLAvailability(URLAvailabilityTTL)
	t.Cleanup(func() { DefaultURLAvailability = originalAvailability })
	for _, tc := range []struct {
		name      string
		body      string
		limits    []int64
		exceeding bool
	}{
		{name: "default", body: "{}" + strings.Repeat(" ", quotaSummaryBodyLimit), exceeding: true},
		{name: "configured", body: "{} ", limits: []int64{2}, exceeding: true},
		{name: "exact limit", body: "{}", limits: []int64{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Request:    req,
				}, nil
			})}}
			summary, err := client.RetrieveUserQuotaSummary(context.Background(), "tok", "proj", tc.limits...)
			if tc.exceeding {
				require.ErrorContains(t, err, "response exceeds")
				require.Nil(t, summary)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, summary)
		})
	}
}

func TestClient_RetrieveUserQuotaSummaryFromURL_RestrictedRedirect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  string
		allowed bool
	}{
		{name: "known endpoint", target: antigravityProdBaseURL, allowed: true},
		{name: "unknown host", target: "https://untrusted.example", allowed: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			client := &Client{httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return &http.Response{
						StatusCode: http.StatusTemporaryRedirect,
						Header:     http.Header{"Location": []string{tc.target + "/v1internal:retrieveUserQuotaSummary"}},
						Body:       io.NopCloser(strings.NewReader("")),
						Request:    req,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(quotaSummaryBody)),
					Request:    req,
				}, nil
			})}}
			summary, err := client.RetrieveUserQuotaSummaryFromURL(context.Background(), antigravityDailyBaseURL, "tok", "proj")
			if !tc.allowed {
				require.ErrorContains(t, err, "redirect to unsupported host")
				require.Nil(t, summary)
				require.Equal(t, 1, requests, "a redirect must not contact an unknown host")
				return
			}
			require.NoError(t, err)
			require.Len(t, summary.Groups, 2)
			require.Equal(t, 2, requests)
		})
	}
}

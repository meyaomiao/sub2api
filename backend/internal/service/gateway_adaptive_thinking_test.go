package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const adaptiveThinkingTestSystem = `[{"type":"text","text":"Keep the client system."}]`

const adaptiveThinkingTestMessages = `[
  {"role":"user","content":"Look up the answer."},
  {"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"signed-history"},{"type":"redacted_thinking","data":"encrypted-history"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"n":1.00}}]},
  {"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}
]`

const adaptiveThinkingTestFormat = `{"type":"json_schema","schema":{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}}`

func adaptiveThinkingTestRequestBody(model string, stream bool, fields string) []byte {
	// Rewriting thinking shifts both raw ranges, so stale offsets cannot pass.
	return []byte(fmt.Sprintf(`{"model":%q,"stream":%t%s,"max_tokens":16384,"metadata":{"user_id":"adaptive-session"},"system":%s,"messages":%s}`, model, stream, fields, adaptiveThinkingTestSystem, adaptiveThinkingTestMessages))
}

func TestParsedRequestNormalizeClaudeAdaptiveThinking(t *testing.T) {
	for _, tc := range []struct {
		name         string
		model        string
		fields       string
		wantThinking string
		wantEffort   string
		changed      bool
	}{
		{
			name:         "opus manual budget defaults to high",
			model:        "claude-opus-5-5",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096,"display":"omitted"}`,
			wantThinking: `{"type":"adaptive","display":"omitted"}`,
			wantEffort:   "high",
			changed:      true,
		},
		{
			name:         "sonnet provider alias preserves explicit effort and format",
			model:        "anthropic/claude-sonnet-5.5",
			fields:       `,"thinking":{"type":"enabled"},"output_config":{"effort":"max","format":` + adaptiveThinkingTestFormat + `,"other_setting":"keep"}`,
			wantThinking: `{"type":"adaptive"}`,
			wantEffort:   "max",
			changed:      true,
		},
		{
			name:         "haiku adds effort without replacing output config",
			model:        "claude-haiku-5-5",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096},"output_config":{"format":` + adaptiveThinkingTestFormat + `,"other_setting":"keep"}`,
			wantThinking: `{"type":"adaptive"}`,
			wantEffort:   "high",
			changed:      true,
		},
		{
			name:         "adaptive removes budget without inventing effort",
			model:        "claude-opus-5-5",
			fields:       `,"thinking":{"type":"adaptive","budget_tokens":4096,"display":"omitted"},"output_config":{"format":` + adaptiveThinkingTestFormat + `}`,
			wantThinking: `{"type":"adaptive","display":"omitted"}`,
			changed:      true,
		},
		{
			name:   "adaptive without budget stays unchanged",
			model:  "claude-sonnet-5-5",
			fields: `,"thinking":{"type":"adaptive"}`,
		},
		{
			name:       "between tools budget is left for validation",
			model:      "claude-sonnet-5-5",
			fields:     `,"thinking":{"type":"between_tools","budget_tokens":4096},"output_config":{"effort":"low"}`,
			wantEffort: "low",
		},
		{
			name:       "disabled stays unchanged",
			model:      "claude-haiku-5-5",
			fields:     `,"thinking":{"type":"disabled","budget_tokens":4096},"output_config":{"effort":"high"}`,
			wantEffort: "high",
		},
		{
			name:   "older model keeps manual thinking",
			model:  "claude-sonnet-4-5-20250929",
			fields: `,"thinking":{"type":"enabled","budget_tokens":4096}`,
		},
		{
			name:   "other signed history family keeps manual thinking",
			model:  "claude-fable-5-2",
			fields: `,"thinking":{"type":"enabled","budget_tokens":4096}`,
		},
		{
			name:   "omitted thinking stays omitted",
			model:  "claude-opus-5-5",
			fields: `,"output_config":{"format":` + adaptiveThinkingTestFormat + `}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := adaptiveThinkingTestRequestBody("client-alias", true, tc.fields+`,"speed":"fast"`)
			original := string(body)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			originalThinkingEnabled := parsed.ThinkingEnabled

			require.NoError(t, parsed.normalizeClaudeAdaptiveThinking(tc.model))
			if tc.changed {
				require.NotEqual(t, original, string(parsed.Body.Bytes()))
				require.True(t, parsed.ThinkingEnabled)
			} else {
				require.Equal(t, original, string(parsed.Body.Bytes()))
				require.Equal(t, originalThinkingEnabled, parsed.ThinkingEnabled)
			}
			require.Equal(t, original, string(body), "normalization must not mutate the borrowed input")

			wantThinking := tc.wantThinking
			if wantThinking == "" {
				wantThinking = gjson.Get(original, "thinking").Raw
			}
			if wantThinking == "" {
				require.False(t, gjson.GetBytes(parsed.Body.Bytes(), "thinking").Exists())
			} else {
				require.JSONEq(t, wantThinking, gjson.GetBytes(parsed.Body.Bytes(), "thinking").Raw)
			}
			require.Equal(t, tc.wantEffort, parsed.OutputEffort)
			require.Equal(t, tc.wantEffort, gjson.GetBytes(parsed.Body.Bytes(), "output_config.effort").String())
			require.Equal(t, tc.wantEffort != "", gjson.GetBytes(parsed.Body.Bytes(), "output_config.effort").Exists())
			for _, path := range []string{"output_config.format", "output_config.other_setting"} {
				require.Equal(t, gjson.Get(original, path).Raw, gjson.GetBytes(parsed.Body.Bytes(), path).Raw, path)
			}
			require.Equal(t, "client-alias", parsed.Model)
			require.True(t, parsed.Stream)
			require.Equal(t, "adaptive-session", parsed.MetadataUserID)
			require.Equal(t, "fast", parsed.Speed)
			require.Equal(t, 16384, parsed.MaxTokens)
			require.True(t, parsed.HasSystem)
			require.Equal(t, adaptiveThinkingTestSystem, string(parsed.SystemRaw()))
			require.Equal(t, adaptiveThinkingTestMessages, string(parsed.MessagesRaw()), "signed history must remain byte-for-byte intact")

			clone, err := parsed.CloneForBody(parsed.Body.Bytes())
			require.NoError(t, err)
			require.Equal(t, tc.wantEffort, clone.OutputEffort, "cached body state must reflect normalized effort")
			require.Equal(t, parsed.ThinkingEnabled, clone.ThinkingEnabled)
			require.Equal(t, adaptiveThinkingTestSystem, string(clone.SystemRaw()))
			require.Equal(t, adaptiveThinkingTestMessages, string(clone.MessagesRaw()))
			normalized := string(parsed.Body.Bytes())
			require.NoError(t, parsed.normalizeClaudeAdaptiveThinking(tc.model))
			require.Equal(t, normalized, string(parsed.Body.Bytes()), "normalization must be idempotent")
		})
	}
}

func TestGatewayServiceAdaptiveThinkingEntryPoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name         string
		model        string
		mappedModel  string
		fields       string
		countTokens  bool
		stream       bool
		passthrough  bool
		wantThinking string
		wantEffort   string
		wantErr      string
	}{
		{
			name:         "messages normal path maps alias before normalizing opus",
			model:        "public-opus",
			mappedModel:  "claude-opus-5-5",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096}`,
			wantThinking: `{"type":"adaptive"}`,
			wantEffort:   "high",
		},
		{
			name:         "stream passthrough preserves sonnet effort and format",
			model:        "public-sonnet",
			mappedModel:  "anthropic/claude-sonnet-5.5",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096,"display":"omitted"},"output_config":{"effort":"low","format":` + adaptiveThinkingTestFormat + `}`,
			stream:       true,
			passthrough:  true,
			wantThinking: `{"type":"adaptive","display":"omitted"}`,
			wantEffort:   "low",
		},
		{
			name:         "count passthrough maps alias before normalizing haiku",
			model:        "public-haiku",
			mappedModel:  "claude-haiku-5-5",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096}`,
			countTokens:  true,
			passthrough:  true,
			wantThinking: `{"type":"adaptive"}`,
			wantEffort:   "high",
		},
		{
			name:         "count normal path cleans adaptive budget and keeps format",
			model:        "public-sonnet",
			mappedModel:  "claude-sonnet-5-5",
			fields:       `,"thinking":{"type":"adaptive","budget_tokens":4096},"output_config":{"format":` + adaptiveThinkingTestFormat + `}`,
			countTokens:  true,
			wantThinking: `{"type":"adaptive"}`,
		},
		{
			name:         "messages mapping to older model keeps manual thinking",
			model:        "claude-opus-5-5",
			mappedModel:  "claude-sonnet-4-5-20250929",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096}`,
			passthrough:  true,
			wantThinking: `{"type":"enabled","budget_tokens":4096}`,
		},
		{
			name:         "count mapping to older model keeps manual thinking",
			model:        "claude-sonnet-5-5",
			mappedModel:  "claude-sonnet-4-5-20250929",
			fields:       `,"thinking":{"type":"enabled","budget_tokens":4096}`,
			countTokens:  true,
			wantThinking: `{"type":"enabled","budget_tokens":4096}`,
		},
		{
			name:         "messages sonnet between tools stays unchanged",
			model:        "public-sonnet",
			mappedModel:  "claude-sonnet-5-5",
			fields:       `,"thinking":{"type":"between_tools"},"output_config":{"effort":"medium"}`,
			wantThinking: `{"type":"between_tools"}`,
			wantEffort:   "medium",
		},
		{
			name:         "count haiku disabled at high stays disabled",
			model:        "public-haiku",
			mappedModel:  "claude-haiku-5-5",
			fields:       `,"thinking":{"type":"disabled"},"output_config":{"effort":"high"}`,
			countTokens:  true,
			passthrough:  true,
			wantThinking: `{"type":"disabled"}`,
			wantEffort:   "high",
		},
		{
			name:        "count still rejects disabled haiku at xhigh",
			model:       "public-haiku",
			mappedModel: "claude-haiku-5-5",
			fields:      `,"thinking":{"type":"disabled"},"output_config":{"effort":"xhigh"}`,
			countTokens: true,
			passthrough: true,
			wantErr:     "supports only low, medium or high effort",
		},
		{
			name:        "messages still validates sonnet after manual thinking conversion",
			model:       "public-sonnet",
			mappedModel: "claude-sonnet-5-5",
			fields:      `,"thinking":{"type":"enabled","budget_tokens":4096},"top_k":1`,
			passthrough: true,
			wantErr:     "does not support top_k",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := adaptiveThinkingTestRequestBody(tc.model, tc.stream, tc.fields)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			path := "/v1/messages"
			if tc.countTokens {
				path += "/count_tokens"
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			account := newAnthropicAPIKeyAccountForTest()
			account.Extra["anthropic_passthrough"] = tc.passthrough
			account.Credentials["model_mapping"] = map[string]any{tc.model: tc.mappedModel}
			upstream := &anthropicHTTPUpstreamRecorder{resp: adaptiveThinkingTestResponse(tc.mappedModel, tc.stream, tc.countTokens)}
			cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
			svc := &GatewayService{
				cfg:                  cfg,
				responseHeaderFilter: compileResponseHeaderFilter(cfg),
				httpUpstream:         upstream,
				rateLimitService:     &RateLimitService{},
				deferredService:      &DeferredService{},
			}

			var result *ForwardResult
			if tc.countTokens {
				err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
			} else {
				result, err = svc.Forward(context.Background(), c, account, parsed)
			}
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
				require.Nil(t, upstream.lastReq, "invalid requests must not reach the upstream")
				return
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, path, upstream.lastReq.URL.Path)
			require.Equal(t, tc.mappedModel, gjson.GetBytes(upstream.lastBody, "model").String())
			require.JSONEq(t, tc.wantThinking, gjson.GetBytes(upstream.lastBody, "thinking").Raw)
			require.Equal(t, tc.wantEffort, gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
			require.Equal(t, tc.wantEffort != "", gjson.GetBytes(upstream.lastBody, "output_config.effort").Exists())
			require.Equal(t, gjson.GetBytes(body, "output_config.format").Raw, gjson.GetBytes(upstream.lastBody, "output_config.format").Raw)
			require.Equal(t, adaptiveThinkingTestMessages, gjson.GetBytes(upstream.lastBody, "messages").Raw, "forwarding must preserve signed history")
			require.Equal(t, tc.wantEffort, parsed.OutputEffort)
			require.Equal(t, adaptiveThinkingTestSystem, string(parsed.SystemRaw()))
			require.Equal(t, adaptiveThinkingTestMessages, string(parsed.MessagesRaw()))
			if tc.countTokens {
				require.JSONEq(t, `{"input_tokens":42}`, rec.Body.String())
			} else {
				require.NotNil(t, result)
				require.Equal(t, tc.stream, result.Stream)
				require.Equal(t, 5, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
			}
		})
	}
}

func TestGatewayServiceAdaptiveThinkingOAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, countTokens := range []bool{false, true} {
		t.Run(fmt.Sprintf("count_tokens=%t", countTokens), func(t *testing.T) {
			const model = "claude-opus-5-5"
			body := adaptiveThinkingTestRequestBody(model, false, `,"thinking":{"type":"enabled","budget_tokens":4096},"output_config":{"effort":"medium"}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			path := "/v1/messages"
			if countTokens {
				path += "/count_tokens"
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			c.Request.Header.Set("User-Agent", "dsh-regression-test")
			upstream := &anthropicHTTPUpstreamRecorder{resp: adaptiveThinkingTestResponse(model, false, countTokens)}
			cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
			svc := &GatewayService{
				cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg),
				httpUpstream: upstream, rateLimitService: &RateLimitService{}, deferredService: &DeferredService{},
			}
			account := &Account{
				ID: 301, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
				Credentials: map[string]any{"access_token": "oauth-test-token"},
				Status:      StatusActive, Schedulable: true,
			}
			if countTokens {
				err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
			} else {
				_, err = svc.Forward(context.Background(), c, account, parsed)
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "Bearer oauth-test-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
			require.Equal(t, "adaptive", gjson.GetBytes(upstream.lastBody, "thinking.type").String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
			require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
			require.Equal(t, "medium", parsed.OutputEffort)
			// OAuth may prepend system instruction turns; locate history by block
			// type instead of assuming its original message index survives.
			var signatures []string
			for _, message := range gjson.GetBytes(upstream.lastBody, "messages").Array() {
				for _, block := range message.Get("content").Array() {
					if block.Get("type").String() == "thinking" {
						signatures = append(signatures, block.Get("signature").String())
					}
				}
			}
			require.Equal(t, []string{"signed-history"}, signatures)
		})
	}
}

func adaptiveThinkingTestResponse(model string, stream, countTokens bool) *http.Response {
	contentType := "application/json"
	payload := fmt.Sprintf(`{"id":"msg_adaptive","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":3}}`, model)
	if countTokens {
		payload = `{"input_tokens":42}`
	} else if stream {
		contentType = "text/event-stream"
		payload = strings.Join([]string{
			"event: message_start",
			fmt.Sprintf(`data: {"type":"message_start","message":{"id":"msg_adaptive","type":"message","role":"assistant","model":%q,"content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`, model),
			"",
			"event: message_delta",
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
			"",
			"event: message_stop",
			`data: {"type":"message_stop"}`,
			"",
			"",
		}, "\n")
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}

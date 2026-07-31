package httpgw

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/activity"
	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

func TestLLMOpenAIRouteInjectsUpstreamKeyLimitsModelAndStreams(t *testing.T) {
	const upstreamKey = "upstream-openai-key-sentinel"
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		if request.URL.Path != "/v1/responses" {
			t.Errorf("upstream path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+upstreamKey {
			t.Errorf("upstream Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "gpt-test" || body["max_output_tokens"] != float64(4096) {
			t.Errorf("validated body = %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\"}\n\n")
	}))
	defer upstream.Close()

	handler, localKey := newLLMTestHandler(t, upstream.URL, routes.ProviderOpenAI, []string{"gpt-test"}, 4096, http.Header{"Authorization": {"Bearer " + upstreamKey}})
	request := httptest.NewRequest(http.MethodPost, "/r/coding/v1/responses", strings.NewReader(`{"model":"gpt-test","input":"hello","stream":true}`))
	request.Header.Set("Authorization", "Bearer "+localKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !response.Flushed || !strings.Contains(response.Body.String(), "response.completed") {
		t.Fatalf("response = %d, flushed=%v, body=%q", response.Code, response.Flushed, response.Body.String())
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d", hits.Load())
	}
}

func TestLLMAnthropicRouteUsesSecondaryAPIKey(t *testing.T) {
	const upstreamKey = "upstream-anthropic-key-sentinel"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("X-Api-Key"); got != upstreamKey {
			t.Errorf("upstream X-Api-Key = %q", got)
		}
		if got := request.Header.Get("Anthropic-Version"); got != "2023-06-01" {
			t.Errorf("Anthropic-Version = %q", got)
		}
		if request.Header.Get("Authorization") != "" {
			t.Errorf("local Authorization leaked upstream")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","content":[]}`)
	}))
	defer upstream.Close()

	handler, localKey := newLLMTestHandler(t, upstream.URL, routes.ProviderAnthropic, []string{"claude-test"}, 2048, http.Header{"X-Api-Key": {upstreamKey}, "Anthropic-Version": {"2023-06-01"}})
	request := httptest.NewRequest(http.MethodPost, "/r/writing/v1/messages", strings.NewReader(`{"model":"claude-test","messages":[]}`))
	request.Header.Set("X-Api-Key", localKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestDisabledRouteAttemptIsRecorded(t *testing.T) {
	route := routes.HTTPRoute{
		Name: "Downloads", Alias: "downloads", TargetSecretRef: "routes/downloads",
		CapabilityDigest: capability.Hash("local-capability"), Policy: routes.NewHTTPPolicy([]string{http.MethodGet}, nil),
		Enabled: false,
	}
	registry, err := routes.NewRegistry(route)
	if err != nil {
		t.Fatal(err)
	}
	recorder := activity.NewMemoryRecorder()
	handler := NewHandler(registry, secrets.NewMemoryStore(), nil, WithActivityRecorder(recorder))
	request := httptest.NewRequest(http.MethodGet, "/r/downloads/file.zip", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled route status = %d", response.Code)
	}
	events := recorder.List(10)
	if len(events) != 1 || events[0].RouteAlias != route.Alias || events[0].Category != routes.KindHTTP || events[0].EventType != "request" || events[0].Result != "blocked" {
		t.Fatalf("disabled route activity = %+v", events)
	}
}

func TestLLMRouteOptionallyRecordsCallsAndTokenUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"response","usage":{"input_tokens":120,"output_tokens":34}}`)
	}))
	defer upstream.Close()
	localKey, digest, err := capability.Generate()
	if err != nil {
		t.Fatal(err)
	}
	policy := routes.NewLLMPolicy(routes.ProviderOpenAI, []string{"gpt-test"}, 1024)
	policy.TrackUsage = true
	registry, err := routes.NewRegistry(routes.HTTPRoute{
		Name: "LLM", Alias: "coding", Kind: routes.KindLLM, Provider: routes.ProviderOpenAI,
		TargetSecretRef: "target/llm", CapabilityDigest: digest, Policy: policy, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(upstream.URL)
	store := secrets.NewMemoryStore()
	if err := store.PutHTTPTarget(t.Context(), "target/llm", secrets.HTTPTarget{BaseURL: parsed, Headers: http.Header{"Authorization": {"Bearer upstream"}}}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/r/coding/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))
	request.Header.Set("Authorization", "Bearer "+localKey)
	response := httptest.NewRecorder()
	NewHandler(registry, store, nil).ServeHTTP(response, request)

	route, err := registry.Get("coding")
	if err != nil || route.Usage.Requests != 1 || route.Usage.InputTokens != 120 || route.Usage.OutputTokens != 34 {
		t.Fatalf("recorded usage = %+v, %v", route.Usage, err)
	}
}

func TestLLMUsageCaptureParsesSSEWithoutKeepingContent(t *testing.T) {
	capture := newLLMUsageCapture()
	_, _ = capture.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":90,\"cache_read_input_tokens\":10}}}\n\n"))
	_, _ = capture.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":27}}\n\n"))
	inputTokens, outputTokens := capture.TokenUsage(true)
	if inputTokens != 100 || outputTokens != 27 {
		t.Fatalf("SSE usage = %d input, %d output", inputTokens, outputTokens)
	}
	capture.Clear()
	if capture.payload != nil {
		t.Fatal("usage capture retained response content")
	}
}

func TestLLMRouteRejectsEndpointModelOutputAndOversizedBodyBeforeUpstream(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer upstream.Close()
	handler, localKey := newLLMTestHandler(t, upstream.URL, routes.ProviderOpenAI, []string{"allowed-model"}, 1024, http.Header{"Authorization": {"Bearer upstream-secret"}})

	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "endpoint", path: "/r/coding/v1/files", body: `{"model":"allowed-model"}`, status: http.StatusNotFound},
		{name: "model", path: "/r/coding/v1/responses", body: `{"model":"forbidden-model"}`, status: http.StatusForbidden},
		{name: "output", path: "/r/coding/v1/responses", body: `{"model":"allowed-model","max_output_tokens":1025}`, status: http.StatusBadRequest},
		{name: "oversized", path: "/r/coding/v1/responses", body: `{"model":"allowed-model","input":"` + strings.Repeat("x", int(routes.DefaultLLMMaxRequestBytes)) + `"}`, status: http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer "+localKey)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("response = %d, content-type=%q, body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("rejected requests reached upstream %d times", hits.Load())
	}
}

func TestLLMRouteEnforcesConcurrencyAndRateLimits(t *testing.T) {
	policy := routes.NewLLMPolicy(routes.ProviderOpenAI, []string{"gpt-test"}, 1024)
	policy.MaxConcurrent = 1
	policy.RequestsPerMinute = 2
	route := routes.HTTPRoute{Alias: "limited", Kind: routes.KindLLM, Provider: routes.ProviderOpenAI, Policy: policy}
	handler := &Handler{limits: make(map[string]*llmLimitState)}

	releaseFirst, limitError := handler.acquireLLMRequest(route)
	if limitError != nil {
		t.Fatalf("first request rejected: %+v", limitError)
	}
	if release, limitError := handler.acquireLLMRequest(route); release != nil || limitError == nil || limitError.Code != "concurrency_limit" {
		t.Fatalf("concurrent request = release:%v error:%+v", release != nil, limitError)
	}
	releaseFirst()

	releaseSecond, limitError := handler.acquireLLMRequest(route)
	if limitError != nil {
		t.Fatalf("second request rejected: %+v", limitError)
	}
	releaseSecond()
	if release, limitError := handler.acquireLLMRequest(route); release != nil || limitError == nil || limitError.Code != "rate_limit" || limitError.RetryAfter == "" {
		t.Fatalf("rate-limited request = release:%v error:%+v", release != nil, limitError)
	}

	route.CapabilityDigest = capability.Hash("rotated-local-api-key")
	releaseRotated, limitError := handler.acquireLLMRequest(route)
	if limitError != nil {
		t.Fatalf("rotated capability inherited old quota: %+v", limitError)
	}
	releaseRotated()
	if len(handler.limits) != 1 {
		t.Fatalf("rotated capability retained %d limiter states", len(handler.limits))
	}
}

func TestHandlerGlobalCapacityRejectsWithoutBlocking(t *testing.T) {
	handler := &Handler{requests: make(chan struct{}, 1)}
	if !tryAcquireRequest(handler.requests) || tryAcquireRequest(handler.requests) {
		t.Fatal("global request capacity did not reject the second slot")
	}
	releaseRequest(handler.requests)
	if !tryAcquireRequest(handler.requests) {
		t.Fatal("global request capacity did not recover after release")
	}
}

func TestHandlerForwardsWithoutExposingUpstreamCredentials(t *testing.T) {
	const upstreamSecret = "upstream-secret-sentinel"
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer "+upstreamSecret {
			t.Errorf("upstream Authorization = %q", got)
		}
		if got := r.Header.Get("Range"); got != "bytes=10-" {
			t.Errorf("upstream Range = %q", got)
		}
		w.Header().Set("Server", "private-upstream")
		w.Header().Set("Set-Cookie", "session=private-upstream-secret")
		w.Header().Set("Content-Location", "https://private.example/downloads/file.zip")
		w.Header().Set("Content-Range", "bytes 10-16/17")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "payload")
	}))
	defer upstream.Close()

	handler, token := newTestHandler(t, upstream.URL+"/downloads/", []string{"channel"})
	request := httptest.NewRequest(http.MethodGet, "/r/manual/file.zip?channel=stable", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Range", "bytes=10-")
	request.Header.Set("Cookie", "must-not-pass=true")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if got := response.Body.String(); got != "payload" {
		t.Fatalf("body = %q", got)
	}
	if got := response.Header().Get("Server"); got != "" {
		t.Fatalf("Server header leaked: %q", got)
	}
	if got := response.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("Set-Cookie header leaked: %q", got)
	}
	if got := response.Header().Get("Content-Location"); got != "" {
		t.Fatalf("Content-Location header leaked: %q", got)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 10-16/17" {
		t.Fatalf("Content-Range header = %q", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d", hits.Load())
	}
}

func TestHandlerRejectsInvalidCapabilityBeforeUpstream(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer upstream.Close()

	handler, _ := newTestHandler(t, upstream.URL+"/", nil)
	request := httptest.NewRequest(http.MethodGet, "/r/manual/file.zip", nil)
	request.Header.Set("Authorization", "Bearer wrong")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream was called %d times", hits.Load())
	}
}

func TestHandlerRejectsPolicyViolations(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	handler, token := newTestHandler(t, upstream.URL+"/", []string{"allowed"})
	tests := []struct {
		name   string
		method string
		target string
		status int
	}{
		{name: "method", method: http.MethodPost, target: "/r/manual/file", status: http.StatusMethodNotAllowed},
		{name: "query", method: http.MethodGet, target: "/r/manual/file?forbidden=1", status: http.StatusBadRequest},
		{name: "encoded traversal", method: http.MethodGet, target: "/r/manual/%252e%252e/private", status: http.StatusNotFound},
		{name: "deeply encoded traversal", method: http.MethodGet, target: "/r/manual/%252525252e%252525252e/private", status: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestHandlerRewritesSameOriginRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/start") {
			http.Redirect(w, r, "next", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler, token := newTestHandler(t, upstream.URL+"/downloads/", nil)
	request := httptest.NewRequest(http.MethodGet, "/r/manual/start", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Location"); got != "/r/manual/next" {
		t.Fatalf("Location = %q", got)
	}
}

func TestHandlerRecordsRejectedRedirectAsFailed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://untrusted.example/private")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	handler, token := newTestHandler(t, upstream.URL+"/downloads/", nil)
	recorder := activity.NewMemoryRecorder()
	handler.activity = recorder
	request := httptest.NewRequest(http.MethodGet, "/r/manual/start", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	events := recorder.List(1)
	if len(events) != 1 || events[0].Result != "failed" {
		t.Fatalf("activity events = %+v", events)
	}
}

func TestHandlerRecordsOnlySanitizedCategorizedActivity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	handler, token := newTestHandler(t, upstream.URL+"/private-target/", nil)
	recorder := activity.NewMemoryRecorder()
	handler.activity = recorder

	allowed := httptest.NewRequest(http.MethodGet, "/r/manual/file?secret-query=hidden", nil)
	allowed.URL.RawQuery = ""
	allowed.RemoteAddr = "127.0.0.1:48123"
	allowed.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(httptest.NewRecorder(), allowed)

	blocked := httptest.NewRequest(http.MethodGet, "/r/manual/private-name", nil)
	blocked.RemoteAddr = "192.168.1.25:48124"
	blocked.Header.Set("Authorization", "Bearer wrong-local-token")
	handler.ServeHTTP(httptest.NewRecorder(), blocked)

	events := recorder.List(10)
	if len(events) != 2 || events[0].Category != "HTTP" || events[0].Result != "blocked" || events[0].Caller != "private-lan" || events[1].Result != "allowed" || events[1].Caller != "loopback" {
		t.Fatalf("activity events = %+v", events)
	}
	for _, event := range events {
		if strings.Contains(event.Action, "private") || strings.Contains(event.Action, "secret") || event.Action != "GET request" {
			t.Fatalf("activity leaked request details: %+v", event)
		}
	}
}

func newTestHandler(t *testing.T, baseURL string, queryKeys []string) (*Handler, string) {
	t.Helper()
	token, digest, err := capability.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	registry, err := routes.NewRegistry(routes.HTTPRoute{
		Alias:            "manual",
		TargetSecretRef:  "target/manual",
		CapabilityDigest: digest,
		Policy:           routes.NewHTTPPolicy([]string{http.MethodGet, http.MethodHead}, queryKeys),
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	store := secrets.NewMemoryStore()
	if err := store.PutHTTPTarget(t.Context(), "target/manual", secrets.HTTPTarget{
		BaseURL: parsed,
		Headers: http.Header{"Authorization": {"Bearer upstream-secret-sentinel"}},
	}); err != nil {
		t.Fatalf("PutHTTPTarget() error = %v", err)
	}
	return NewHandler(registry, store, nil), token
}

func newLLMTestHandler(t *testing.T, baseURL, provider string, models []string, maxOutputTokens int, headers http.Header) (*Handler, string) {
	t.Helper()
	localKey, digest, err := capability.Generate()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := routes.NewRegistry(routes.HTTPRoute{
		Name: "LLM", Alias: map[string]string{routes.ProviderOpenAI: "coding", routes.ProviderAnthropic: "writing"}[provider],
		Kind: routes.KindLLM, Provider: provider, TargetSecretRef: "target/llm",
		CapabilityDigest: digest, Policy: routes.NewLLMPolicy(provider, models, maxOutputTokens),
		Egress: "Direct", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.PutHTTPTarget(t.Context(), "target/llm", secrets.HTTPTarget{BaseURL: parsed, Headers: headers}); err != nil {
		t.Fatal(err)
	}
	return NewHandler(registry, store, nil), localKey
}

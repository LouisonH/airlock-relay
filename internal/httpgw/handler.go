package httpgw

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

const routePrefix = "/r/"

var safeRequestHeaders = map[string]struct{}{
	"Accept":            {},
	"Accept-Encoding":   {},
	"If-Modified-Since": {},
	"If-None-Match":     {},
	"If-Range":          {},
	"Range":             {},
	"User-Agent":        {},
}

var safeResponseHeaders = map[string]struct{}{
	"Accept-Ranges":       {},
	"Cache-Control":       {},
	"Content-Disposition": {},
	"Content-Encoding":    {},
	"Content-Language":    {},
	"Content-Length":      {},
	"Content-Range":       {},
	"Content-Type":        {},
	"Etag":                {},
	"Expires":             {},
	"Last-Modified":       {},
	"Location":            {},
	"Retry-After":         {},
	"Vary":                {},
}

type RouteLookup interface {
	Lookup(alias string) (routes.HTTPRoute, error)
}

type LLMUsageRecorder interface {
	RecordLLMUsage(alias string, requests, inputTokens, outputTokens uint64) error
}

type RouteTransport interface {
	RoundTrip(policy string, request *http.Request) (*http.Response, error)
}

type Handler struct {
	routes    RouteLookup
	secrets   secrets.Store
	transport RouteTransport
	limitsMu  sync.Mutex
	limits    map[string]*llmLimitState
}

type llmLimitState struct {
	capabilityDigest  capability.Digest
	requestsPerMinute int
	maxConcurrent     int
	windowStarted     time.Time
	requests          int
	current           int
}

func NewHandler(registry RouteLookup, secretStore secrets.Store, transport RouteTransport) *Handler {
	if transport == nil {
		transport = egress.NewManager(nil)
	}
	return &Handler{routes: registry, secrets: secretStore, transport: transport, limits: make(map[string]*llmLimitState)}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	alias, suffix, err := parseRoutePath(request.URL)
	if err != nil {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	route, err := h.routes.Lookup(alias)
	if err != nil {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}
	localCredential := bearerToken(request.Header.Get("Authorization"))
	if route.EffectiveKind() == routes.KindLLM && route.Provider == routes.ProviderAnthropic && strings.TrimSpace(request.Header.Get("X-Api-Key")) != "" {
		localCredential = strings.TrimSpace(request.Header.Get("X-Api-Key"))
	}
	if err := capability.Verify(localCredential, route.CapabilityDigest); err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="airlock"`)
		writeRouteError(w, route, http.StatusUnauthorized, "invalid_api_key", "invalid local API key")
		return
	}
	if !route.Policy.AllowsMethod(request.Method) {
		w.Header().Set("Allow", allowedMethods(route.Policy))
		writeRouteError(w, route, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	for key := range request.URL.Query() {
		if !route.Policy.AllowsQueryKey(key) {
			writeRouteError(w, route, http.StatusBadRequest, "query_not_allowed", "query parameter not allowed")
			return
		}
	}
	endpointPath := "/" + strings.TrimPrefix(suffix, "/")
	if route.EffectiveKind() == routes.KindLLM && !route.Policy.AllowsPath(endpointPath) {
		writeRouteError(w, route, http.StatusNotFound, "endpoint_not_allowed", "endpoint not allowed")
		return
	}
	release, limitError := h.acquireLLMRequest(route)
	if limitError != nil {
		w.Header().Set("Retry-After", limitError.RetryAfter)
		writeRouteError(w, route, http.StatusTooManyRequests, limitError.Code, limitError.Message)
		return
	}
	defer release()

	target, err := h.secrets.ResolveHTTPTarget(request.Context(), route.TargetSecretRef)
	if err != nil || target.BaseURL == nil || !validTargetScheme(target.BaseURL.Scheme) {
		writeRouteError(w, route, http.StatusBadGateway, "target_unavailable", "target unavailable")
		return
	}

	upstreamURL, err := buildUpstreamURL(target.BaseURL, suffix, request.URL.Query())
	if err != nil {
		writeRouteError(w, route, http.StatusBadRequest, "invalid_path", "invalid route path")
		return
	}

	var upstreamBody *bytes.Reader
	if route.EffectiveKind() == routes.KindLLM {
		payload, requestError := prepareLLMRequest(request, route, endpointPath)
		if requestError != nil {
			writeRouteError(w, route, requestError.Status, requestError.Code, requestError.Message)
			return
		}
		upstreamBody = bytes.NewReader(payload)
	}
	var bodyReader io.Reader
	if upstreamBody != nil {
		bodyReader = upstreamBody
	}
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, upstreamURL.String(), bodyReader)
	if err != nil {
		writeRouteError(w, route, http.StatusBadGateway, "target_unavailable", "target unavailable")
		return
	}
	copySafeRequestHeaders(upstreamRequest.Header, request.Header)
	if route.EffectiveKind() == routes.KindLLM {
		upstreamRequest.Header.Set("Content-Type", "application/json")
		upstreamRequest = egress.AllowConnectivityRetry(upstreamRequest)
	}
	for key, values := range target.Headers {
		upstreamRequest.Header.Del(key)
		for _, value := range values {
			upstreamRequest.Header.Add(key, value)
		}
	}

	response, err := h.transport.RoundTrip(route.Egress, upstreamRequest)
	if err != nil {
		writeRouteError(w, route, http.StatusBadGateway, "upstream_unavailable", "upstream request failed")
		return
	}
	defer response.Body.Close()

	if isRedirect(response.StatusCode) {
		if err := rewriteLocation(response, target.BaseURL, alias); err != nil {
			writeRouteError(w, route, http.StatusBadGateway, "redirect_rejected", "upstream redirect rejected")
			return
		}
	}

	copySafeResponseHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	if request.Method != http.MethodHead {
		streaming := route.EffectiveKind() == routes.KindLLM && strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream")
		var capture *llmUsageCapture
		responseBody := io.Reader(response.Body)
		if route.EffectiveKind() == routes.KindLLM && route.Policy.TrackUsage {
			capture = newLLMUsageCapture()
			responseBody = io.TeeReader(response.Body, capture)
		}
		if streaming {
			copyStreamingResponse(w, responseBody)
		} else {
			_, _ = io.Copy(w, responseBody)
		}
		if capture != nil {
			inputTokens, outputTokens := capture.TokenUsage(streaming)
			if recorder, ok := h.routes.(LLMUsageRecorder); ok {
				_ = recorder.RecordLLMUsage(route.Alias, 1, inputTokens, outputTokens)
			}
			capture.Clear()
		}
	}
}

type llmLimitError struct {
	Code       string
	Message    string
	RetryAfter string
}

func (h *Handler) acquireLLMRequest(route routes.HTTPRoute) (func(), *llmLimitError) {
	if route.EffectiveKind() != routes.KindLLM {
		return func() {}, nil
	}
	now := time.Now()
	h.limitsMu.Lock()
	state := h.limits[route.Alias]
	if state == nil || state.capabilityDigest != route.CapabilityDigest || state.requestsPerMinute != route.Policy.RequestsPerMinute || state.maxConcurrent != route.Policy.MaxConcurrent {
		state = &llmLimitState{
			capabilityDigest:  route.CapabilityDigest,
			requestsPerMinute: route.Policy.RequestsPerMinute,
			maxConcurrent:     route.Policy.MaxConcurrent,
			windowStarted:     now,
		}
		h.limits[route.Alias] = state
	}
	if now.Sub(state.windowStarted) >= time.Minute {
		state.windowStarted = now
		state.requests = 0
	}
	if state.current >= state.maxConcurrent {
		h.limitsMu.Unlock()
		return nil, &llmLimitError{Code: "concurrency_limit", Message: "concurrent request limit reached", RetryAfter: "1"}
	}
	if state.requests >= state.requestsPerMinute {
		retryAfter := int(time.Until(state.windowStarted.Add(time.Minute)).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		h.limitsMu.Unlock()
		return nil, &llmLimitError{Code: "rate_limit", Message: "request rate limit reached", RetryAfter: fmt.Sprintf("%d", retryAfter)}
	}
	state.requests++
	state.current++
	h.limitsMu.Unlock()
	return func() {
		h.limitsMu.Lock()
		if state.current > 0 {
			state.current--
		}
		h.limitsMu.Unlock()
	}, nil
}

func parseRoutePath(value *url.URL) (string, string, error) {
	if value == nil || !strings.HasPrefix(value.Path, routePrefix) {
		return "", "", errors.New("invalid route path")
	}
	remainder := strings.TrimPrefix(value.Path, routePrefix)
	alias, suffix, found := strings.Cut(remainder, "/")
	if !found {
		suffix = ""
	}
	if alias == "" || unsafePath(suffix, value.RawPath) {
		return "", "", errors.New("invalid route path")
	}
	return alias, suffix, nil
}

func unsafePath(suffix, rawPath string) bool {
	candidates := []string{suffix, rawPath}
	for _, candidate := range candidates {
		decoded := candidate
		stable := false
		for range 16 {
			if strings.Contains(decoded, "\\") || strings.ContainsRune(decoded, '\x00') {
				return true
			}
			for _, segment := range strings.Split(decoded, "/") {
				if segment == "." || segment == ".." {
					return true
				}
			}
			next, err := url.PathUnescape(decoded)
			if err != nil {
				return true
			}
			if next == decoded {
				stable = true
				break
			}
			decoded = next
		}
		if !stable {
			return true
		}
	}
	return false
}

func buildUpstreamURL(base *url.URL, suffix string, query url.Values) (*url.URL, error) {
	joined, err := url.JoinPath(base.String(), suffix)
	if err != nil {
		return nil, err
	}
	result, err := url.Parse(joined)
	if err != nil {
		return nil, err
	}
	result.RawQuery = query.Encode()
	return result, nil
}

func validTargetScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}

func bearerToken(header string) string {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

func allowedMethods(policy routes.HTTPPolicy) string {
	methods := make([]string, 0, len(policy.AllowedMethods))
	for method := range policy.AllowedMethods {
		methods = append(methods, method)
	}
	return strings.Join(methods, ", ")
}

func copySafeRequestHeaders(destination, source http.Header) {
	for key := range safeRequestHeaders {
		for _, value := range source.Values(key) {
			destination.Add(key, value)
		}
	}
}

func copySafeResponseHeaders(destination, source http.Header) {
	for key, values := range source {
		canonical := http.CanonicalHeaderKey(key)
		if _, allowed := safeResponseHeaders[canonical]; !allowed {
			continue
		}
		for _, value := range values {
			destination.Add(canonical, value)
		}
	}
}

func isRedirect(status int) bool {
	return status >= 300 && status <= 399
}

func rewriteLocation(response *http.Response, base *url.URL, alias string) error {
	location := response.Header.Get("Location")
	if location == "" {
		return nil
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return err
	}
	resolved := response.Request.URL.ResolveReference(parsed)
	if !strings.EqualFold(resolved.Scheme, base.Scheme) || !strings.EqualFold(resolved.Host, base.Host) {
		return errors.New("cross-origin redirect")
	}
	basePath := path.Clean("/" + strings.TrimPrefix(base.Path, "/"))
	resolvedPath := path.Clean("/" + strings.TrimPrefix(resolved.Path, "/"))
	if resolvedPath != basePath && !strings.HasPrefix(resolvedPath, basePath+"/") {
		return errors.New("redirect escaped target base path")
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(resolvedPath, basePath), "/")
	local := routePrefix + alias
	if relative != "" {
		local += "/" + relative
	}
	if resolved.RawQuery != "" {
		local += "?" + resolved.RawQuery
	}
	response.Header.Set("Location", local)
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "airlock: %s\n", message)
}

func writeRouteError(w http.ResponseWriter, route routes.HTTPRoute, status int, code, message string) {
	if route.EffectiveKind() == routes.KindLLM {
		writeLLMError(w, route.Provider, status, code, message)
		return
	}
	writeError(w, status, message)
}

func copyStreamingResponse(w http.ResponseWriter, source io.Reader) {
	buffer := make([]byte, 16<<10)
	flusher, canFlush := w.(http.Flusher)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, writeErr := w.Write(buffer[:read]); writeErr != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

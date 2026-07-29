package httpgw

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/LouisonH/airlock-relay/internal/capability"
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

type Handler struct {
	routes    RouteLookup
	secrets   secrets.Store
	transport http.RoundTripper
}

func NewHandler(registry RouteLookup, secretStore secrets.Store, transport http.RoundTripper) *Handler {
	if transport == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		transport = &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
	}
	return &Handler{routes: registry, secrets: secretStore, transport: transport}
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
	if err := capability.Verify(bearerToken(request.Header.Get("Authorization")), route.CapabilityDigest); err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="airlock"`)
		writeError(w, http.StatusUnauthorized, "invalid capability")
		return
	}
	if !route.Policy.AllowsMethod(request.Method) {
		w.Header().Set("Allow", allowedMethods(route.Policy))
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	for key := range request.URL.Query() {
		if !route.Policy.AllowsQueryKey(key) {
			writeError(w, http.StatusBadRequest, "query parameter not allowed")
			return
		}
	}

	target, err := h.secrets.ResolveHTTPTarget(request.Context(), route.TargetSecretRef)
	if err != nil || target.BaseURL == nil || !validTargetScheme(target.BaseURL.Scheme) {
		writeError(w, http.StatusBadGateway, "target unavailable")
		return
	}

	upstreamURL, err := buildUpstreamURL(target.BaseURL, suffix, request.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid route path")
		return
	}

	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, upstreamURL.String(), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "target unavailable")
		return
	}
	copySafeRequestHeaders(upstreamRequest.Header, request.Header)
	for key, values := range target.Headers {
		upstreamRequest.Header.Del(key)
		for _, value := range values {
			upstreamRequest.Header.Add(key, value)
		}
	}

	response, err := h.transport.RoundTrip(upstreamRequest)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer response.Body.Close()

	if isRedirect(response.StatusCode) {
		if err := rewriteLocation(response, target.BaseURL, alias); err != nil {
			writeError(w, http.StatusBadGateway, "upstream redirect rejected")
			return
		}
	}

	copySafeResponseHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	if request.Method != http.MethodHead {
		_, _ = io.Copy(w, response.Body)
	}
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

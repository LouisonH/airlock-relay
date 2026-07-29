package httpgw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

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

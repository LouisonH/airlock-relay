package egress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	Direct                 = "Direct"
	Proxy                  = "Proxy"
	Auto                   = "Auto"
	DefaultSecretReference = "egress/default"
)

var ErrProxyUnavailable = errors.New("proxy egress is not configured")

type Manager struct {
	mu       sync.RWMutex
	template *http.Transport
	direct   *http.Transport
	proxy    *http.Transport
}

func NewManager(template *http.Transport) *Manager {
	if template == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		template = &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
	}
	template = template.Clone()
	direct := template.Clone()
	direct.Proxy = nil
	return &Manager{template: template, direct: direct}
}

func ValidateProxyURL(proxyURL *url.URL) error {
	if proxyURL == nil || proxyURL.Host == "" || proxyURL.RawQuery != "" || proxyURL.Fragment != "" || (proxyURL.Path != "" && proxyURL.Path != "/") {
		return errors.New("invalid proxy URL")
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return nil
	default:
		return errors.New("unsupported proxy scheme")
	}
}

func (m *Manager) Configure(proxyURL *url.URL) error {
	if err := ValidateProxyURL(proxyURL); err != nil {
		return err
	}
	clonedURL := *proxyURL
	transport := m.template.Clone()
	transport.Proxy = http.ProxyURL(&clonedURL)

	m.mu.Lock()
	previous := m.proxy
	m.proxy = transport
	m.mu.Unlock()
	if previous != nil {
		previous.CloseIdleConnections()
	}
	return nil
}

func (m *Manager) Clear() {
	m.mu.Lock()
	previous := m.proxy
	m.proxy = nil
	m.mu.Unlock()
	if previous != nil {
		previous.CloseIdleConnections()
	}
}

func (m *Manager) Configured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxy != nil
}

func (m *Manager) RoundTrip(policy string, request *http.Request) (*http.Response, error) {
	switch policy {
	case "", Direct:
		return m.direct.RoundTrip(request)
	case Proxy:
		transport := m.proxyTransport()
		if transport == nil {
			return nil, ErrProxyUnavailable
		}
		return transport.RoundTrip(request)
	case Auto:
		return m.roundTripAuto(request)
	default:
		return nil, errors.New("invalid egress policy")
	}
}

func (m *Manager) roundTripAuto(request *http.Request) (*http.Response, error) {
	response, err := m.direct.RoundTrip(request)
	if err == nil || response != nil || !safeToRetry(request) || !retryableConnectivityError(err) {
		return response, err
	}
	transport := m.proxyTransport()
	if transport == nil {
		return nil, err
	}
	return transport.RoundTrip(request.Clone(request.Context()))
}

func (m *Manager) proxyTransport() *http.Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxy
}

func safeToRetry(request *http.Request) bool {
	if request == nil || (request.Method != http.MethodGet && request.Method != http.MethodHead) {
		return false
	}
	return request.Body == nil || request.Body == http.NoBody
}

func retryableConnectivityError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return true
	}
	var operationError *net.OpError
	return errors.As(err, &operationError) && (operationError.Op == "dial" || operationError.Op == "connect")
}

package egress

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	xproxy "golang.org/x/net/proxy"
)

const (
	Direct                 = "Direct"
	Proxy                  = "Proxy"
	Auto                   = "Auto"
	DefaultSecretReference = "egress/default"
)

var ErrProxyUnavailable = errors.New("proxy egress is not configured")

type connectivityRetryKey struct{}

// AllowConnectivityRetry marks a replayable request for direct-to-proxy retry.
// The retry still occurs only when direct dialing fails before any response.
func AllowConnectivityRetry(request *http.Request) *http.Request {
	if request == nil {
		return nil
	}
	return request.WithContext(context.WithValue(request.Context(), connectivityRetryKey{}, true))
}

type Manager struct {
	mu       sync.RWMutex
	template *http.Transport
	direct   *http.Transport
	proxy    *http.Transport
	proxyURL *url.URL
	dial     func(context.Context, string, string) (net.Conn, error)
}

func NewManager(template *http.Transport) *Manager {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if template == nil {
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
	dialContext := template.DialContext
	if dialContext == nil {
		dialContext = dialer.DialContext
		template.DialContext = dialContext
	}
	direct := template.Clone()
	direct.Proxy = nil
	return &Manager{template: template, direct: direct, dial: dialContext}
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
	m.proxyURL = &clonedURL
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
	m.proxyURL = nil
	m.mu.Unlock()
	if previous != nil {
		previous.CloseIdleConnections()
	}
}

func (m *Manager) Configured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxy != nil && m.proxyURL != nil
}

func (m *Manager) DialContext(ctx context.Context, policy, network, address string) (net.Conn, error) {
	if !strings.HasPrefix(network, "tcp") || address == "" {
		return nil, errors.New("unsupported egress dial")
	}
	switch policy {
	case "", Direct:
		return m.dial(ctx, network, address)
	case Proxy:
		return m.dialProxy(ctx, network, address)
	case Auto:
		connection, err := m.dial(ctx, network, address)
		if err == nil || !retryableConnectivityError(err) {
			return connection, err
		}
		if !m.Configured() {
			return nil, err
		}
		return m.dialProxy(ctx, network, address)
	default:
		return nil, errors.New("invalid egress policy")
	}
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
	retry := request.Clone(request.Context())
	if request.GetBody != nil {
		body, bodyErr := request.GetBody()
		if bodyErr != nil {
			return nil, err
		}
		retry.Body = body
	}
	return transport.RoundTrip(retry)
}

func (m *Manager) proxyTransport() *http.Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxy
}

func (m *Manager) configuredProxyURL() *url.URL {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proxyURL == nil {
		return nil
	}
	cloned := *m.proxyURL
	return &cloned
}

func (m *Manager) dialProxy(ctx context.Context, network, address string) (net.Conn, error) {
	proxyURL := m.configuredProxyURL()
	if proxyURL == nil {
		return nil, ErrProxyUnavailable
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case "http", "https":
		return m.dialHTTPConnect(ctx, network, address, proxyURL)
	case "socks5", "socks5h":
		return m.dialSOCKS5(ctx, network, address, proxyURL)
	default:
		return nil, errors.New("unsupported proxy scheme")
	}
}

func (m *Manager) dialHTTPConnect(ctx context.Context, network, address string, proxyURL *url.URL) (net.Conn, error) {
	proxyAddress := proxyURL.Host
	if proxyURL.Port() == "" {
		port := "80"
		if strings.EqualFold(proxyURL.Scheme, "https") {
			port = "443"
		}
		proxyAddress = net.JoinHostPort(proxyURL.Hostname(), port)
	}
	connection, err := m.dial(ctx, network, proxyAddress)
	if err != nil {
		return nil, err
	}
	closeOnError := func() {
		_ = connection.Close()
	}
	if strings.EqualFold(proxyURL.Scheme, "https") {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: proxyURL.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			closeOnError()
			return nil, errors.New("proxy TLS handshake failed")
		}
		connection = tlsConnection
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	}
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		credentials := proxyURL.User.Username() + ":" + password
		request.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	}
	if err := request.Write(connection); err != nil {
		closeOnError()
		return nil, errors.New("proxy CONNECT request failed")
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		closeOnError()
		return nil, errors.New("proxy CONNECT response failed")
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		closeOnError()
		return nil, fmt.Errorf("proxy CONNECT rejected with status %d", response.StatusCode)
	}
	_ = connection.SetDeadline(time.Time{})
	return &bufferedConn{Conn: connection, reader: reader}, nil
}

func (m *Manager) dialSOCKS5(ctx context.Context, network, address string, proxyURL *url.URL) (net.Conn, error) {
	var authentication *xproxy.Auth
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		authentication = &xproxy.Auth{User: proxyURL.User.Username(), Password: password}
	}
	forward := contextDialer{dial: m.dial}
	dialer, err := xproxy.SOCKS5(network, proxyURL.Host, authentication, forward)
	if err != nil {
		return nil, errors.New("initialize SOCKS5 proxy")
	}
	contextAware, ok := dialer.(xproxy.ContextDialer)
	if !ok {
		return nil, errors.New("SOCKS5 proxy does not support cancellation")
	}
	return contextAware.DialContext(ctx, network, address)
}

type contextDialer struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

func (d contextDialer) Dial(network, address string) (net.Conn, error) {
	return d.dial(context.Background(), network, address)
}

func (d contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(payload []byte) (int, error) {
	return c.reader.Read(payload)
}

func safeToRetry(request *http.Request) bool {
	if request == nil {
		return false
	}
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return request.Body == nil || request.Body == http.NoBody
	}
	marked, _ := request.Context().Value(connectivityRetryKey{}).(bool)
	return marked && request.GetBody != nil
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

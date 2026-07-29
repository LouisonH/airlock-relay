package egress

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyAndAutoUseProtectedHTTPProxy(t *testing.T) {
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		proxyHits.Add(1)
		if got := request.Header.Get("Proxy-Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("agent:secret")) {
			t.Errorf("Proxy-Authorization = %q", got)
		}
		_, _ = io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	proxyURL.User = url.UserPassword("agent", "secret")

	manager := NewManager(nil)
	if err := manager.Configure(proxyURL); err != nil {
		t.Fatal(err)
	}
	assertBody(t, manager, Proxy, "http://hidden.invalid/file", "proxied")

	closed := closedAddress(t)
	assertBody(t, manager, Auto, "http://"+closed+"/file", "proxied")
	if proxyHits.Load() != 2 {
		t.Fatalf("proxy hits = %d", proxyHits.Load())
	}
}

func TestAutoDoesNotRetryUnsafeOrTLSFailures(t *testing.T) {
	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	manager := NewManager(nil)
	if err := manager.Configure(proxyURL); err != nil {
		t.Fatal(err)
	}

	request, _ := http.NewRequest(http.MethodPost, "http://"+closedAddress(t)+"/submit", http.NoBody)
	if _, err := manager.RoundTrip(Auto, request); err == nil {
		t.Fatal("POST unexpectedly succeeded")
	}

	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tlsTarget.Close()
	request, _ = http.NewRequest(http.MethodGet, tlsTarget.URL, nil)
	if _, err := manager.RoundTrip(Auto, request); err == nil {
		t.Fatal("untrusted TLS unexpectedly succeeded")
	}
	if proxyHits.Load() != 0 {
		t.Fatalf("unsafe fallback used proxy %d times", proxyHits.Load())
	}
}

func TestHTTPSUsesHTTPConnectProxy(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "connected")
	}))
	defer target.Close()
	var connectHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		connectHits.Add(1)
		tunnelHTTPConnect(t, w, request)
	}))
	defer proxy.Close()

	template := http.DefaultTransport.(*http.Transport).Clone()
	template.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // Test server certificate only.
	manager := NewManager(template)
	proxyURL, _ := url.Parse(proxy.URL)
	if err := manager.Configure(proxyURL); err != nil {
		t.Fatal(err)
	}
	assertBody(t, manager, Proxy, target.URL, "connected")
	if connectHits.Load() != 1 {
		t.Fatalf("CONNECT hits = %d", connectHits.Load())
	}
}

func TestSOCKS5Proxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "socks")
	}))
	defer target.Close()
	socksAddress := startSOCKS5Server(t)
	manager := NewManager(nil)
	proxyURL, _ := url.Parse("socks5://" + socksAddress)
	if err := manager.Configure(proxyURL); err != nil {
		t.Fatal(err)
	}
	assertBody(t, manager, Proxy, target.URL, "socks")
}

func TestProxyConfigurationValidation(t *testing.T) {
	manager := NewManager(nil)
	for _, raw := range []string{"ftp://127.0.0.1:21", "http://127.0.0.1:7890/path", "http://"} {
		parsed, _ := url.Parse(raw)
		if err := manager.Configure(parsed); err == nil {
			t.Errorf("Configure(%q) succeeded", raw)
		}
	}
	request, _ := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if _, err := manager.RoundTrip(Proxy, request); err != ErrProxyUnavailable {
		t.Fatalf("proxy unavailable error = %v", err)
	}
}

func TestRawTCPDialUsesConnectSOCKS5AndAuto(t *testing.T) {
	targetAddress := startEchoServer(t)
	connectProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		tunnelHTTPConnect(t, w, request)
	}))
	defer connectProxy.Close()

	manager := NewManager(nil)
	connectURL, _ := url.Parse(connectProxy.URL)
	if err := manager.Configure(connectURL); err != nil {
		t.Fatal(err)
	}
	assertEchoDial(t, manager, Proxy, targetAddress)

	socksAddress := startSOCKS5Server(t)
	socksURL, _ := url.Parse("socks5://" + socksAddress)
	if err := manager.Configure(socksURL); err != nil {
		t.Fatal(err)
	}
	assertEchoDial(t, manager, Proxy, targetAddress)

	directDialer := &net.Dialer{Timeout: 5 * time.Second}
	template := http.DefaultTransport.(*http.Transport).Clone()
	template.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == targetAddress {
			return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("direct unavailable")}
		}
		return directDialer.DialContext(ctx, network, address)
	}
	manager = NewManager(template)
	if err := manager.Configure(connectURL); err != nil {
		t.Fatal(err)
	}
	assertEchoDial(t, manager, Auto, targetAddress)
}

func assertBody(t *testing.T, manager *Manager, policy, target, want string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := manager.RoundTrip(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != want {
		t.Fatalf("body = %q, want %q", payload, want)
	}
}

func assertEchoDial(t *testing.T, manager *Manager, policy, address string) {
	t.Helper()
	connection, err := manager.DialContext(t.Context(), policy, "tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	payload := []byte("airlock-echo")
	if _, err := connection.Write(payload); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(payload))
	if _, err := io.ReadFull(connection, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != string(payload) {
		t.Fatalf("echo = %q", response)
	}
}

func startEchoServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer connection.Close()
				_, _ = io.Copy(connection, connection)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		wait.Wait()
	})
	return listener.Addr().String()
}

func closedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func tunnelHTTPConnect(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		t.Error("proxy response cannot be hijacked")
		return
	}
	downstream, err := net.DialTimeout("tcp", request.Host, 5*time.Second)
	if err != nil {
		http.Error(writer, "connect failed", http.StatusBadGateway)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = downstream.Close()
		t.Error(err)
		return
	}
	_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = buffered.Flush()
	go func() {
		_, _ = io.Copy(downstream, client)
		_ = downstream.Close()
	}()
	_, _ = io.Copy(client, downstream)
	_ = client.Close()
}

func startSOCKS5Server(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		connection, err := listener.Accept()
		if err == nil {
			handleSOCKS5(connection)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		wait.Wait()
	})
	return listener.Addr().String()
}

func handleSOCKS5(client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(client)
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil || header[0] != 5 {
		return
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return
	}
	if _, err := client.Write([]byte{5, 0}); err != nil {
		return
	}
	request := make([]byte, 4)
	if _, err := io.ReadFull(reader, request); err != nil || request[0] != 5 || request[1] != 1 {
		return
	}
	host, err := readSOCKSHost(reader, request[3])
	if err != nil {
		return
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return
	}
	port := int(portBytes[0])<<8 | int(portBytes[1])
	upstream, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		_, _ = client.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	if _, err := client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	go func() {
		_, _ = io.Copy(upstream, reader)
		if tcp, ok := upstream.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}()
	_, _ = io.Copy(client, upstream)
}

func readSOCKSHost(reader *bufio.Reader, addressType byte) (string, error) {
	switch addressType {
	case 1:
		address := make([]byte, net.IPv4len)
		_, err := io.ReadFull(reader, address)
		return net.IP(address).String(), err
	case 3:
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		address := make([]byte, int(length))
		_, err = io.ReadFull(reader, address)
		return string(address), err
	case 4:
		address := make([]byte, net.IPv6len)
		_, err := io.ReadFull(reader, address)
		return net.IP(address).String(), err
	default:
		return "", fmt.Errorf("unsupported address type %d", addressType)
	}
}

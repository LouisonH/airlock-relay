package sshgw

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

const (
	localCapability = "airlock_local_capability_sentinel_32_bytes"
	upstreamUser    = "upstream-user-sentinel"
	upstreamPass    = "upstream-password-sentinel"
)

func TestGatewayPasswordIsolationAndRestrictedExec(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: upstreamUser, password: upstreamPass})
	route := testSSHRoute(NewPolicy([]string{"build --release"}, nil, false), egress.Direct)
	target := secrets.SSHTarget{
		Address:         upstream.address(),
		Username:        upstreamUser,
		Password:        []byte(upstreamPass),
		ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
	}
	gateway := startGateway(t, route, target, egress.NewManager(nil))
	client := dialGateway(t, gateway, ssh.Password(localCapability))
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	session.Stdin = strings.NewReader("input-must-not-be-forwarded")
	output, err := session.CombinedOutput("build --release")
	if err != nil {
		t.Fatalf("allowed command failed: %v", err)
	}
	if !bytes.Contains(output, []byte("ran:build --release")) {
		t.Fatalf("output = %q", output)
	}
	snapshot := upstream.snapshot()
	if snapshot.commands != 1 || snapshot.lastCommand != "build --release" || snapshot.lastStdin != "" {
		t.Fatalf("upstream execution = %+v", snapshot)
	}
	if snapshot.lastUser != upstreamUser || snapshot.lastPassword != upstreamPass {
		t.Fatalf("upstream authentication was not injected: %+v", snapshot)
	}

	denied, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := denied.Run("cat /etc/passwd"); err == nil {
		t.Fatal("disallowed command succeeded")
	}
	_ = denied.Close()
	if got := upstream.snapshot().commands; got != 1 {
		t.Fatalf("disallowed command reached upstream: %d commands", got)
	}

	assertRestrictedRequests(t, client)
	if got := upstream.snapshot().connections; got != 1 {
		t.Fatalf("restricted requests opened upstream connections: %d", got)
	}

	wrongConfig := gateway.clientConfig(ssh.Password("wrong-local-capability"))
	if wrong, err := ssh.Dial("tcp", gateway.address, wrongConfig); err == nil {
		_ = wrong.Close()
		t.Fatal("wrong local capability authenticated")
	} else {
		assertNoProtectedValues(t, err.Error(), target)
	}
	if got := upstream.snapshot().connections; got != 1 {
		t.Fatalf("failed local authentication reached upstream: %d connections", got)
	}
}

func TestGatewayAllowsAllExecAndRecordsCommands(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: upstreamUser, password: upstreamPass})
	route := testSSHRoute(NewPolicyWithOptions(nil, nil, false, true, true), egress.Direct)
	target := secrets.SSHTarget{
		Address: upstream.address(), Username: upstreamUser, Password: []byte(upstreamPass),
		ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
	}
	audit, err := OpenFileCommandAudit(filepath.Join(t.TempDir(), "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	gateway := startGateway(t, route, target, egress.NewManager(nil), audit)
	client := dialGateway(t, gateway, ssh.Password(localCapability))
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.CombinedOutput("uname -a"); err != nil {
		t.Fatalf("allow-all exec failed: %v", err)
	}
	events := audit.List(10)
	if len(events) != 1 || events[0].Command != "uname -a" || events[0].Result != "allowed" {
		t.Fatalf("command events = %+v", events)
	}
	assertRestrictedRequests(t, client)
}

func TestGatewayLocalPublicKeyAndUpstreamPrivateKey(t *testing.T) {
	_, upstreamPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	upstreamClientSigner, err := ssh.NewSignerFromKey(upstreamPrivate)
	if err != nil {
		t.Fatal(err)
	}
	upstream := startUpstream(t, upstreamAuth{
		username:             upstreamUser,
		publicKeyFingerprint: ssh.FingerprintSHA256(upstreamClientSigner.PublicKey()),
	})
	privateBlock, err := ssh.MarshalPrivateKeyWithPassphrase(upstreamPrivate, "airlock upstream", []byte("private-key-passphrase-sentinel"))
	if err != nil {
		t.Fatal(err)
	}

	localClientSigner := generateSigner(t)
	policy := NewPolicy(
		[]string{"exit 7"},
		[]string{ssh.FingerprintSHA256(localClientSigner.PublicKey())},
		false,
	)
	route := testSSHRoute(policy, egress.Direct)
	target := secrets.SSHTarget{
		Address:            upstream.address(),
		Username:           upstreamUser,
		PrivateKey:         pem.EncodeToMemory(privateBlock),
		PrivateKeyPassword: []byte("private-key-passphrase-sentinel"),
		ExpectedHostKey:    upstream.hostSigner.PublicKey().Marshal(),
	}
	gateway := startGateway(t, route, target, egress.NewManager(nil))
	client := dialGateway(t, gateway, ssh.PublicKeys(localClientSigner))
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	err = session.Run("exit 7")
	var exitError *ssh.ExitError
	if !errors.As(err, &exitError) || exitError.ExitStatus() != 7 {
		t.Fatalf("exit error = %v", err)
	}
	snapshot := upstream.snapshot()
	if snapshot.lastUser != upstreamUser || snapshot.publicKeyAuths != 1 || snapshot.commands != 1 {
		t.Fatalf("private-key upstream authentication = %+v", snapshot)
	}
}

func TestGatewayFailsClosedWithoutLeakingProtectedTarget(t *testing.T) {
	tests := []struct {
		name   string
		target func(*upstreamHarness) secrets.SSHTarget
	}{
		{
			name: "host key mismatch",
			target: func(upstream *upstreamHarness) secrets.SSHTarget {
				return secrets.SSHTarget{
					Address:         upstream.address(),
					Username:        upstreamUser,
					Password:        []byte(upstreamPass),
					ExpectedHostKey: generateSigner(t).PublicKey().Marshal(),
				}
			},
		},
		{
			name: "upstream authentication failure",
			target: func(upstream *upstreamHarness) secrets.SSHTarget {
				return secrets.SSHTarget{
					Address:         upstream.address(),
					Username:        upstreamUser,
					Password:        []byte("wrong-upstream-password-sentinel"),
					ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := startUpstream(t, upstreamAuth{username: upstreamUser, password: upstreamPass})
			target := test.target(upstream)
			dialer := &countingDialer{next: egress.NewManager(nil)}
			route := testSSHRoute(NewPolicy([]string{"build --release"}, nil, false), egress.Auto)
			gateway := startGateway(t, route, target, dialer)
			client := dialGateway(t, gateway, ssh.Password(localCapability))
			defer client.Close()
			session, err := client.NewSession()
			if err != nil {
				t.Fatal(err)
			}
			err = session.Run("build --release")
			if err == nil {
				t.Fatal("protected upstream failure succeeded")
			}
			assertNoProtectedValues(t, err.Error(), target)
			if dialer.calls.Load() != 1 {
				t.Fatalf("SSH handshake failure triggered another dial: %d", dialer.calls.Load())
			}
			if upstream.snapshot().commands != 0 {
				t.Fatal("failed handshake reached upstream exec")
			}
		})
	}
}

func TestGatewayUsesProtectedHTTPConnectEgress(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: upstreamUser, password: upstreamPass})
	proxy, proxyHits := startConnectProxy(t)
	manager := egress.NewManager(nil)
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Configure(proxyURL); err != nil {
		t.Fatal(err)
	}
	route := testSSHRoute(NewPolicy([]string{"build --release"}, nil, false), egress.Proxy)
	target := secrets.SSHTarget{
		Address:         upstream.address(),
		Username:        upstreamUser,
		Password:        []byte(upstreamPass),
		ExpectedHostKey: upstream.hostSigner.PublicKey().Marshal(),
	}
	gateway := startGateway(t, route, target, manager)
	client := dialGateway(t, gateway, ssh.Password(localCapability))
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Run("build --release"); err != nil {
		t.Fatal(err)
	}
	if proxyHits.Load() != 1 || upstream.snapshot().commands != 1 {
		t.Fatalf("proxy hits = %d, upstream commands = %d", proxyHits.Load(), upstream.snapshot().commands)
	}
}

func TestProbeHostKeyStopsBeforeAuthentication(t *testing.T) {
	upstream := startUpstream(t, upstreamAuth{username: upstreamUser, password: upstreamPass})
	key, err := ProbeHostKey(t.Context(), egress.NewManager(nil), egress.Direct, upstream.address())
	if err != nil {
		t.Fatal(err)
	}
	if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(upstream.hostSigner.PublicKey()) {
		t.Fatal("probed SSH host key does not match upstream")
	}
	snapshot := upstream.snapshot()
	if snapshot.passwordAuths != 0 || snapshot.publicKeyAuths != 0 {
		t.Fatalf("host key probe attempted authentication: %+v", snapshot)
	}
}

func TestServerRejectsNonLoopbackListener(t *testing.T) {
	registry, err := NewRegistry(testSSHRoute(NewPolicy([]string{"build"}, nil, false), egress.Direct))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(registry, secrets.NewMemoryStore(), egress.NewManager(nil), generateSigner(t))
	if err != nil {
		t.Fatal(err)
	}
	listener := staticListener{address: &net.TCPAddr{IP: net.IPv4zero, Port: 2222}}
	if err := server.Serve(listener); !errors.Is(err, ErrNonLoopbackListen) {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestServerAllowsPrivateListenerOnlyWhenLANIsEnabled(t *testing.T) {
	registry, err := NewRegistry(testSSHRoute(NewPolicy([]string{"build"}, nil, false), egress.Direct))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(registry, secrets.NewMemoryStore(), egress.NewManager(nil), generateSigner(t), WithLANAccess())
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []net.IP{net.IPv4zero, net.ParseIP("192.168.1.10")} {
		err := server.Serve(staticListener{address: &net.TCPAddr{IP: ip, Port: 2222}})
		if errors.Is(err, ErrNonLoopbackListen) {
			t.Fatalf("LAN listener %s was rejected: %v", ip, err)
		}
	}
	public := staticListener{address: &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 2222}}
	if err := server.Serve(public); !errors.Is(err, ErrNonLoopbackListen) {
		t.Fatalf("public listener error = %v", err)
	}
}

func assertRestrictedRequests(t *testing.T, client *ssh.Client) {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestPty("xterm", 24, 80, ssh.TerminalModes{}); err == nil {
		t.Error("PTY request succeeded")
	}
	if err := session.Shell(); err == nil {
		t.Error("shell request succeeded")
	}
	_ = session.Close()

	session, err = client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestSubsystem("sftp"); err == nil {
		t.Error("SFTP subsystem request succeeded")
	}
	if accepted, err := session.SendRequest("auth-agent-req@openssh.com", true, nil); err != nil || accepted {
		t.Errorf("agent forwarding response = %v, %v", accepted, err)
	}
	if accepted, err := session.SendRequest("x11-req", true, nil); err != nil || accepted {
		t.Errorf("X11 forwarding response = %v, %v", accepted, err)
	}
	_ = session.Close()

	if connection, err := client.Dial("tcp", "127.0.0.1:1"); err == nil {
		_ = connection.Close()
		t.Error("direct-tcpip forwarding succeeded")
	}
	if listener, err := client.Listen("tcp", "127.0.0.1:0"); err == nil {
		_ = listener.Close()
		t.Error("remote forwarding succeeded")
	}
}

type gatewayHarness struct {
	address      string
	localHostKey ssh.PublicKey
}

func startGateway(t *testing.T, route Route, target secrets.SSHTarget, dialer EgressDialer, audits ...CommandAudit) gatewayHarness {
	t.Helper()
	store := secrets.NewMemoryStore()
	if err := store.PutSSHTarget(t.Context(), route.TargetSecretRef, target); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(route)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner := generateSigner(t)
	var options []ServerOption
	if len(audits) > 0 {
		options = append(options, WithCommandAudit(audits[0]))
	}
	server, err := NewServer(registry, store, dialer, hostSigner, options...)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	return gatewayHarness{address: listener.Addr().String(), localHostKey: hostSigner.PublicKey()}
}

func (g gatewayHarness) clientConfig(authentication ssh.AuthMethod) *ssh.ClientConfig {
	algorithms := ssh.SupportedAlgorithms()
	return &ssh.ClientConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		User:              "build",
		Auth:              []ssh.AuthMethod{authentication},
		HostKeyCallback:   ssh.FixedHostKey(g.localHostKey),
		HostKeyAlgorithms: algorithms.HostKeys,
		Timeout:           5 * time.Second,
	}
}

func dialGateway(t *testing.T, gateway gatewayHarness, authentication ssh.AuthMethod) *ssh.Client {
	t.Helper()
	client, err := ssh.Dial("tcp", gateway.address, gateway.clientConfig(authentication))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func testSSHRoute(policy Policy, egressPolicy string) Route {
	return Route{
		Name:             "Build",
		Alias:            "build",
		TargetSecretRef:  "ssh/build",
		CapabilityDigest: capability.Hash(localCapability),
		Policy:           policy,
		Egress:           egressPolicy,
		Enabled:          true,
	}
}

func assertNoProtectedValues(t *testing.T, message string, target secrets.SSHTarget) {
	t.Helper()
	for _, protected := range []string{target.Address, target.Username, string(target.Password), string(target.PrivateKeyPassword)} {
		if protected != "" && strings.Contains(message, protected) {
			t.Fatalf("client-visible error leaked protected value %q: %q", protected, message)
		}
	}
}

type countingDialer struct {
	next  EgressDialer
	calls atomic.Int32
}

type staticListener struct{ address net.Addr }

func (l staticListener) Accept() (net.Conn, error) { return nil, errors.New("unexpected accept") }
func (l staticListener) Close() error              { return nil }
func (l staticListener) Addr() net.Addr            { return l.address }

func (d *countingDialer) DialContext(ctx context.Context, policy, network, address string) (net.Conn, error) {
	d.calls.Add(1)
	return d.next.DialContext(ctx, policy, network, address)
}

type upstreamAuth struct {
	username             string
	password             string
	publicKeyFingerprint string
}

type upstreamSnapshot struct {
	connections    int
	passwordAuths  int
	publicKeyAuths int
	commands       int
	lastUser       string
	lastPassword   string
	lastCommand    string
	lastStdin      string
}

type upstreamHarness struct {
	listener   net.Listener
	hostSigner ssh.Signer
	auth       upstreamAuth
	mu         sync.Mutex
	state      upstreamSnapshot
}

func startUpstream(t *testing.T, auth upstreamAuth) *upstreamHarness {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	harness := &upstreamHarness{listener: listener, hostSigner: generateSigner(t), auth: auth}
	t.Cleanup(func() { _ = listener.Close() })
	go harness.serve()
	return harness
}

func (h *upstreamHarness) address() string { return h.listener.Addr().String() }

func (h *upstreamHarness) snapshot() upstreamSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *upstreamHarness) serve() {
	for {
		raw, err := h.listener.Accept()
		if err != nil {
			return
		}
		h.mu.Lock()
		h.state.connections++
		h.mu.Unlock()
		go h.serveConnection(raw)
	}
}

func (h *upstreamHarness) serveConnection(raw net.Conn) {
	defer raw.Close()
	algorithms := ssh.SupportedAlgorithms()
	config := &ssh.ServerConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		PublicKeyAuthAlgorithms: algorithms.PublicKeyAuths,
		MaxAuthTries:            3,
		PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			h.mu.Lock()
			h.state.passwordAuths++
			h.state.lastUser = metadata.User()
			h.state.lastPassword = string(password)
			h.mu.Unlock()
			if metadata.User() != h.auth.username || subtle.ConstantTimeCompare(password, []byte(h.auth.password)) != 1 || h.auth.password == "" {
				return nil, ErrAuthentication
			}
			return nil, nil
		},
		PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			h.mu.Lock()
			h.state.publicKeyAuths++
			h.state.lastUser = metadata.User()
			h.mu.Unlock()
			if metadata.User() != h.auth.username || ssh.FingerprintSHA256(key) != h.auth.publicKeyFingerprint || h.auth.publicKeyFingerprint == "" {
				return nil, ErrAuthentication
			}
			return nil, nil
		},
	}
	config.AddHostKey(h.hostSigner)
	connection, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	defer connection.Close()
	go rejectGlobalRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.UnknownChannelType, "session required")
			continue
		}
		accepted, channelRequests, err := channel.Accept()
		if err != nil {
			continue
		}
		go h.serveSession(accepted, channelRequests)
	}
}

func (h *upstreamHarness) serveSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for request := range requests {
		if request.Type != "exec" {
			_ = request.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		if ssh.Unmarshal(request.Payload, &payload) != nil {
			_ = request.Reply(false, nil)
			return
		}
		h.mu.Lock()
		h.state.commands++
		h.state.lastCommand = payload.Command
		h.mu.Unlock()
		_ = request.Reply(true, nil)
		stdin, _ := io.ReadAll(channel)
		h.mu.Lock()
		h.state.lastStdin = string(stdin)
		h.mu.Unlock()
		_, _ = channel.Write([]byte("ran:" + payload.Command + "\n"))
		status := uint32(0)
		if payload.Command == "exit 7" {
			status = 7
		}
		_ = sendExitStatus(channel, status)
		return
	}
}

func startConnectProxy(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect {
			http.Error(writer, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		hits.Add(1)
		upstream, err := net.DialTimeout("tcp", request.Host, 5*time.Second)
		if err != nil {
			http.Error(writer, "connect failed", http.StatusBadGateway)
			return
		}
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			_ = upstream.Close()
			return
		}
		client, buffered, err := hijacker.Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
		go func() {
			_, _ = io.Copy(upstream, client)
			if tcp, ok := upstream.(*net.TCPConn); ok {
				_ = tcp.CloseWrite()
			}
		}()
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
		_ = upstream.Close()
	}))
	t.Cleanup(proxy.Close)
	return proxy, hits
}

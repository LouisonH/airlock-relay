package sshgw

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/LouisonH/airlock-relay/internal/activity"
	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

const (
	authenticatedRouteExtension = "airlock-route"
	defaultMaxConnections       = 128
	defaultMaxSessions          = 256
)

var (
	ErrAuthentication    = errors.New("SSH authentication failed")
	ErrNonLoopbackListen = errors.New("SSH listener must use a loopback IP")
)

type RouteLookup interface {
	Lookup(alias string) (Route, error)
	LookupByUsername(username string) (Route, error)
	GetByUsername(username string) (Route, error)
}

type Server struct {
	routes           RouteLookup
	secrets          secrets.Store
	dialer           EgressDialer
	commandAudit     CommandAudit
	activity         activity.Recorder
	config           *ssh.ServerConfig
	handshakeTimeout time.Duration
	allowLAN         bool
	connections      chan struct{}
	sessions         chan struct{}
}

type ServerOption func(*Server)

func WithCommandAudit(audit CommandAudit) ServerOption {
	return func(server *Server) { server.commandAudit = audit }
}

func WithActivityRecorder(recorder activity.Recorder) ServerOption {
	return func(server *Server) { server.activity = recorder }
}

func WithLANAccess() ServerOption {
	return func(server *Server) { server.allowLAN = true }
}

// WithResourceLimits bounds authenticated and unauthenticated SSH work so a
// reachable listener cannot consume an unbounded number of goroutines or file
// descriptors. Values below one keep the secure defaults.
func WithResourceLimits(maxConnections, maxSessions int) ServerOption {
	return func(server *Server) {
		if maxConnections > 0 {
			server.connections = make(chan struct{}, maxConnections)
		}
		if maxSessions > 0 {
			server.sessions = make(chan struct{}, maxSessions)
		}
	}
}

func NewServer(routes RouteLookup, secretStore secrets.Store, dialer EgressDialer, hostSigner ssh.Signer, options ...ServerOption) (*Server, error) {
	if routes == nil || secretStore == nil || dialer == nil || hostSigner == nil {
		return nil, errors.New("invalid SSH server configuration")
	}
	server := &Server{
		routes:           routes,
		secrets:          secretStore,
		dialer:           dialer,
		handshakeTimeout: 15 * time.Second,
		connections:      make(chan struct{}, defaultMaxConnections),
		sessions:         make(chan struct{}, defaultMaxSessions),
	}
	for _, option := range options {
		if option != nil {
			option(server)
		}
	}
	algorithms := ssh.SupportedAlgorithms()
	server.config = &ssh.ServerConfig{
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		PublicKeyAuthAlgorithms: algorithms.PublicKeyAuths,
		MaxAuthTries:            3,
		PasswordCallback:        server.authenticatePassword,
		PublicKeyCallback:       server.authenticatePublicKey,
		ServerVersion:           "SSH-2.0-Airlock",
	}
	server.config.AddHostKey(hostSigner)
	return server, nil
}

func (s *Server) Serve(listener net.Listener) error {
	if !isAllowedTCPListener(listener, s.allowLAN) {
		return ErrNonLoopbackListen
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		if !tryAcquire(s.connections) {
			_ = connection.Close()
			continue
		}
		go func() {
			defer release(s.connections)
			s.serveConnection(connection)
		}()
	}
}

func isAllowedTCPListener(listener net.Listener, allowLAN bool) bool {
	if listener == nil {
		return false
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.IP == nil {
		return false
	}
	if address.IP.IsLoopback() {
		return true
	}
	return allowLAN && (address.IP.IsUnspecified() || address.IP.IsPrivate())
}

func (s *Server) serveConnection(raw net.Conn) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(s.handshakeTimeout))
	connection, channels, globalRequests, err := ssh.NewServerConn(raw, s.config)
	if err != nil {
		return
	}
	defer connection.Close()
	_ = raw.SetDeadline(time.Time{})
	go rejectGlobalRequests(globalRequests)

	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.Prohibited, "channel type prohibited by policy")
			continue
		}
		if !tryAcquire(s.sessions) {
			_ = channel.Reject(ssh.ResourceShortage, "session capacity reached")
			continue
		}
		local, requests, err := channel.Accept()
		if err != nil {
			release(s.sessions)
			continue
		}
		go func() {
			defer release(s.sessions)
			s.handleSession(connection, local, requests)
		}()
	}
}

func tryAcquire(slots chan struct{}) bool {
	if slots == nil {
		return true
	}
	select {
	case slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func release(slots chan struct{}) {
	if slots == nil {
		return
	}
	select {
	case <-slots:
	default:
	}
}

func (s *Server) authenticatePassword(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	route, err := s.routes.LookupByUsername(metadata.User())
	if err != nil {
		s.recordDisabledAuthentication(metadata)
		_ = capability.Verify(string(password), capability.Digest{})
		return nil, ErrAuthentication
	}
	if err := capability.Verify(string(password), route.CapabilityDigest); err != nil {
		return nil, ErrAuthentication
	}
	return routePermissions(route.Alias), nil
}

func (s *Server) authenticatePublicKey(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	route, err := s.routes.LookupByUsername(metadata.User())
	if err != nil {
		s.recordDisabledAuthentication(metadata)
		return nil, ErrAuthentication
	}
	fingerprint := ssh.FingerprintSHA256(key)
	if _, ok := route.Policy.LocalPublicKeyFingerprints[fingerprint]; !ok {
		return nil, ErrAuthentication
	}
	return routePermissions(route.Alias), nil
}

func (s *Server) recordDisabledAuthentication(metadata ssh.ConnMetadata) {
	if s.activity == nil || metadata == nil {
		return
	}
	route, err := s.routes.GetByUsername(metadata.User())
	if err != nil || route.Enabled {
		return
	}
	caller := route.EffectiveLocalUsername() + "@network"
	if address := metadata.RemoteAddr(); address != nil {
		if host, _, splitErr := net.SplitHostPort(address.String()); splitErr == nil {
			if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
				caller = route.EffectiveLocalUsername() + "@loopback"
			} else if ip != nil && ip.IsPrivate() {
				caller = route.EffectiveLocalUsername() + "@private-lan"
			}
		}
	}
	_ = s.activity.Record(activity.Event{
		RouteAlias: route.Alias,
		Category:   "SSH",
		EventType:  "request",
		Caller:     caller,
		Action:     "SSH connection to disabled route",
		Result:     "blocked",
		Egress:     route.Egress,
	})
}

func routePermissions(alias string) *ssh.Permissions {
	return &ssh.Permissions{Extensions: map[string]string{authenticatedRouteExtension: alias}}
}

func rejectGlobalRequests(requests <-chan *ssh.Request) {
	for request := range requests {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
	}
}

func (s *Server) handleSession(connection *ssh.ServerConn, local ssh.Channel, requests <-chan *ssh.Request) {
	alias := ""
	if connection.Permissions != nil {
		alias = connection.Permissions.Extensions[authenticatedRouteExtension]
	}
	if alias == "" {
		_ = local.Close()
		return
	}

	var done <-chan struct{}
	for {
		select {
		case <-done:
			return
		case request, ok := <-requests:
			if !ok {
				if done == nil {
					_ = local.Close()
				}
				return
			}
			if done != nil {
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
				continue
			}

			route, err := s.routes.Lookup(alias)
			if err != nil || route.EffectiveLocalUsername() != connection.User() {
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
				continue
			}

			switch request.Type {
			case "exec":
				var payload struct{ Command string }
				if ssh.Unmarshal(request.Payload, &payload) != nil || !route.Policy.AllowsCommand(payload.Command) {
					if validCommand(payload.Command) {
						s.recordCommand(route, payload.Command, "blocked", 0)
					}
					if request.WantReply {
						_ = request.Reply(false, nil)
					}
					_ = sendExitStatus(local, 126)
					_ = local.Close()
					return
				}

				executionDone := make(chan struct{})
				done = executionDone
				go func() {
					defer close(executionDone)
					s.execute(local, request, route, payload.Command)
				}()
			case "subsystem":
				var payload struct{ Name string }
				if ssh.Unmarshal(request.Payload, &payload) != nil || payload.Name != "sftp" {
					if request.WantReply {
						_ = request.Reply(false, nil)
					}
					continue
				}
				caller := sshActivityCaller(route, connection.RemoteAddr())
				if !route.Policy.AllowSFTP {
					s.recordSFTP(route, caller, "blocked", 0)
					if request.WantReply {
						_ = request.Reply(false, nil)
					}
					continue
				}
				executionDone := make(chan struct{})
				done = executionDone
				go func() {
					defer close(executionDone)
					s.executeSFTP(local, request, route, caller)
				}()
			default:
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
			}
		}
	}
}

func (s *Server) execute(local ssh.Channel, request *ssh.Request, route Route, command string) {
	defer local.Close()
	started := time.Now()
	result := "failed"
	defer func() { s.recordCommand(route, command, result, time.Since(started)) }()
	ctx, cancel := context.WithTimeout(context.Background(), s.handshakeTimeout)
	defer cancel()
	target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer clearTarget(&target)

	upstream, err := dialUpstream(ctx, s.dialer, route, target)
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer upstream.Close()
	session, err := upstream.NewSession()
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer session.Close()

	if route.Policy.AllowStdin {
		session.Stdin = local
	}
	session.Stdout = local
	session.Stderr = local.Stderr()
	upstreamCommand := ApplyKeywordReplacements(command, route.KeywordReplacements)
	if err := session.Start(upstreamCommand); err != nil {
		_ = request.Reply(false, nil)
		return
	}
	if request.WantReply {
		if err := request.Reply(true, nil); err != nil {
			return
		}
	}
	waitError := session.Wait()
	if waitError == nil {
		result = "allowed"
	}
	_ = sendExitStatus(local, exitStatus(waitError))
}

func (s *Server) executeSFTP(local ssh.Channel, request *ssh.Request, route Route, caller string) {
	defer local.Close()
	started := time.Now()
	result := "failed"
	defer func() { s.recordSFTP(route, caller, result, time.Since(started)) }()
	ctx, cancel := context.WithTimeout(context.Background(), s.handshakeTimeout)
	defer cancel()
	target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer clearTarget(&target)

	upstream, err := dialUpstream(ctx, s.dialer, route, target)
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer upstream.Close()
	remote, remoteRequests, err := upstream.OpenChannel("session", nil)
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	defer remote.Close()

	accepted, err := remote.SendRequest("subsystem", true, ssh.Marshal(struct{ Name string }{Name: "sftp"}))
	if err != nil || !accepted {
		_ = request.Reply(false, nil)
		return
	}
	if request.WantReply {
		if err := request.Reply(true, nil); err != nil {
			return
		}
	}
	status := make(chan uint32, 1)
	go func() { status <- receiveExitStatus(remoteRequests) }()
	var copies sync.WaitGroup
	copies.Add(3)
	go func() {
		defer copies.Done()
		_, _ = io.Copy(remote, local)
		_ = remote.CloseWrite()
	}()
	go func() {
		defer copies.Done()
		_, _ = io.Copy(local, remote)
	}()
	go func() {
		defer copies.Done()
		_, _ = io.Copy(local.Stderr(), remote.Stderr())
	}()
	copies.Wait()
	exitCode := <-status
	if exitCode == 0 {
		result = "allowed"
	}
	_ = sendExitStatus(local, exitCode)
}

func receiveExitStatus(requests <-chan *ssh.Request) uint32 {
	status := uint32(255)
	for request := range requests {
		if request.Type == "exit-status" {
			var payload struct{ Status uint32 }
			if ssh.Unmarshal(request.Payload, &payload) == nil {
				status = payload.Status
			}
		}
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
	}
	return status
}

func (s *Server) recordCommand(route Route, command, result string, duration time.Duration) {
	if s.commandAudit == nil || !route.Policy.RecordCommands {
		return
	}
	_ = s.commandAudit.Record(CommandEvent{
		RouteAlias: route.Alias,
		Command:    command,
		Result:     result,
		DurationMS: duration.Milliseconds(),
		Egress:     route.Egress,
	})
}

func (s *Server) recordSFTP(route Route, caller, result string, duration time.Duration) {
	if s.activity == nil {
		return
	}
	_ = s.activity.Record(activity.Event{
		RouteAlias: route.Alias,
		Category:   "SSH",
		EventType:  "command",
		Caller:     caller,
		Action:     "SFTP subsystem",
		Result:     result,
		DurationMS: duration.Milliseconds(),
		Egress:     route.Egress,
	})
}

func sshActivityCaller(route Route, address net.Addr) string {
	caller := route.EffectiveLocalUsername() + "@network"
	if address == nil {
		return caller
	}
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return caller
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return caller
	}
	if ip.IsLoopback() {
		return route.EffectiveLocalUsername() + "@loopback"
	}
	if ip.IsPrivate() {
		return route.EffectiveLocalUsername() + "@private-lan"
	}
	return caller
}

func exitStatus(err error) uint32 {
	if err == nil {
		return 0
	}
	var exitError *ssh.ExitError
	if errors.As(err, &exitError) {
		status := exitError.ExitStatus()
		if status >= 0 {
			return uint32(status)
		}
	}
	return 255
}

func sendExitStatus(channel ssh.Channel, status uint32) error {
	_, err := channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: status}))
	return err
}

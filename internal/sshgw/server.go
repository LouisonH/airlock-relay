package sshgw

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sort"
	"strings"
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
	ErrRouteDisabled     = errors.New("route disabled: access denied")
	ErrNonLoopbackListen = errors.New("SSH listener must use a loopback IP")
)

type RouteLookup interface {
	Lookup(alias string) (Route, error)
	LookupByUsername(username string) (Route, error)
	GetByUsername(username string) (Route, error)
}

type ptyRequest struct {
	Term    string
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
	Modes   []byte
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
		BannerCallback:          server.authenticationBanner,
		PasswordCallback:        server.authenticatePassword,
		PublicKeyCallback:       server.authenticatePublicKey,
		ServerVersion:           "SSH-2.0-Airlock",
	}
	server.config.AddHostKey(hostSigner)
	return server, nil
}

// authenticationBanner gives a disabled local identity an actionable client
// message while keeping the SSH protocol's required authentication failure.
// The actual upstream target and credential state are never included.
func (s *Server) authenticationBanner(metadata ssh.ConnMetadata) string {
	if metadata == nil {
		return ""
	}
	route, err := s.routes.GetByUsername(metadata.User())
	if err == nil && !route.Enabled {
		return "Airlock: 路由未开启：access denied\n"
	}
	return ""
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
		if disabledRoute, lookupErr := s.routes.GetByUsername(metadata.User()); lookupErr == nil && !disabledRoute.Enabled {
			return nil, ErrRouteDisabled
		}
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
		if disabledRoute, lookupErr := s.routes.GetByUsername(metadata.User()); lookupErr == nil && !disabledRoute.Enabled {
			return nil, ErrRouteDisabled
		}
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
	var pty *ptyRequest
	startCommand := func(request *ssh.Request, route Route, command string) {
		executionDone := make(chan struct{})
		done = executionDone
		go func() {
			defer close(executionDone)
			s.execute(local, request, route, command)
		}()
	}
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
			case "pty-req":
				// Remember the terminal request so an enabled interactive
				// shell can replay it upstream; other routes only acknowledge
				// it so `ssh -t host command` reaches the command policy.
				var payload ptyRequest
				if ssh.Unmarshal(request.Payload, &payload) == nil {
					pty = &payload
				}
				if request.WantReply {
					_ = request.Reply(true, nil)
				}
			case "window-change":
				if request.WantReply {
					_ = request.Reply(true, nil)
				}
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

				startCommand(request, route, payload.Command)
			case "shell":
				// A plain `ssh user@airlock` asks for an interactive shell.
				// Routes with the interactive-shell switch replay the terminal
				// upstream with stored credentials. Everything else stays
				// non-interactive: single-command routes run their exact
				// command, and other routes return guidance instead of a
				// refused-shell error.
				if route.Policy.AllowInteractiveShell && route.Policy.AllowAllCommands {
					interactiveDone := make(chan struct{})
					done = interactiveDone
					caller := sshActivityCaller(route, connection.RemoteAddr())
					go func() {
						defer close(interactiveDone)
						s.interactiveShell(local, request, route, pty, requests, caller)
					}()
					<-interactiveDone
					return
				}
				if !route.Policy.AllowAllCommands && len(route.Policy.AllowedCommands) == 1 {
					var command string
					for allowed := range route.Policy.AllowedCommands {
						command = allowed
					}
					startCommand(request, route, command)
					continue
				}
				if request.WantReply {
					_ = request.Reply(true, nil)
				}
				writeShellUnavailable(local, route)
				_ = sendExitStatus(local, 1)
				_ = local.Close()
				return
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

func (s *Server) interactiveShell(local ssh.Channel, request *ssh.Request, route Route, pty *ptyRequest, requests <-chan *ssh.Request, caller string) {
	defer local.Close()
	started := time.Now()
	result := "failed"
	defer func() { s.recordCommand(route, "interactive-shell", result, time.Since(started)) }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(route.EffectiveAuthenticationTimeoutSeconds())*time.Second)
	defer cancel()
	target, err := s.secrets.ResolveSSHTarget(ctx, route.TargetSecretRef)
	if err != nil {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
		return
	}
	defer clearTarget(&target)

	upstream, err := dialUpstream(ctx, s.dialer, route, target)
	if err != nil {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
		return
	}
	defer upstream.Close()
	session, err := upstream.NewSession()
	if err != nil {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
		return
	}
	defer session.Close()

	session.Stdin = local
	session.Stdout = local
	session.Stderr = local.Stderr()
	if pty != nil {
		if err := session.RequestPty(pty.Term, int(pty.Columns), int(pty.Rows), parseTerminalModes(pty.Modes)); err != nil {
			if request.WantReply {
				_ = request.Reply(false, nil)
			}
			return
		}
	}
	if err := session.Shell(); err != nil {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
		return
	}
	if request.WantReply {
		if err := request.Reply(true, nil); err != nil {
			return
		}
	}

	forwardDone := make(chan struct{})
	go func() {
		defer close(forwardDone)
		for forwardRequest := range requests {
			forwardSessionRequest(session, forwardRequest)
		}
		_ = session.Close()
	}()

	waitError := session.Wait()
	if waitError == nil {
		result = "allowed"
	}
	_ = sendExitStatus(local, exitStatus(waitError))
	_ = local.Close()
	select {
	case <-forwardDone:
	case <-time.After(2 * time.Second):
	}
	if s.activity != nil {
		_ = s.activity.Record(activity.Event{
			RouteAlias: route.Alias,
			Category:   "SSH",
			EventType:  "command",
			Caller:     caller,
			Action:     "SSH interactive shell",
			Result:     result,
			DurationMS: time.Since(started).Milliseconds(),
			Egress:     route.Egress,
		})
	}
}

func parseTerminalModes(raw []byte) ssh.TerminalModes {
	modes := ssh.TerminalModes{}
	for len(raw) >= 5 {
		opcode := raw[0]
		value := binary.BigEndian.Uint32(raw[1:5])
		raw = raw[5:]
		if opcode == 0 {
			break
		}
		modes[opcode] = value
	}
	return modes
}

func forwardSessionRequest(session *ssh.Session, request *ssh.Request) {
	switch request.Type {
	case "window-change":
		var payload struct {
			Columns uint32
			Rows    uint32
			Width   uint32
			Height  uint32
		}
		if ssh.Unmarshal(request.Payload, &payload) == nil {
			_, _ = session.SendRequest("window-change", false, request.Payload)
		}
	case "signal":
		_, _ = session.SendRequest("signal", false, request.Payload)
	default:
		if request.Type != "pty-req" {
			_, _ = session.SendRequest(request.Type, false, request.Payload)
		}
	}
	if request.WantReply {
		_ = request.Reply(true, nil)
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

func writeShellUnavailable(channel ssh.Channel, route Route) {
	var message strings.Builder
	message.WriteString("Airlock: 该路由不允许交互式 Shell（interactive shell disabled）。\n")
	if route.Policy.AllowAllCommands {
		message.WriteString("该路由只允许非交互 exec 命令；请在 PuTTY 登录命令或 ssh 参数中填写要执行的命令，或配置单条精确命令。\n")
	} else {
		commands := make([]string, 0, len(route.Policy.AllowedCommands))
		for command := range route.Policy.AllowedCommands {
			commands = append(commands, command)
		}
		sort.Strings(commands)
		message.WriteString("允许的命令：\n")
		for _, command := range commands {
			message.WriteString("  " + command + "\n")
		}
	}
	_, _ = channel.Write([]byte(message.String()))
}

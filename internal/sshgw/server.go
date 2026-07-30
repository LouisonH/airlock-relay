package sshgw

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/LouisonH/airlock-relay/internal/capability"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"golang.org/x/crypto/ssh"
)

const authenticatedRouteExtension = "airlock-route"

var (
	ErrAuthentication    = errors.New("SSH authentication failed")
	ErrNonLoopbackListen = errors.New("SSH listener must use a loopback IP")
)

type RouteLookup interface {
	Lookup(alias string) (Route, error)
}

type Server struct {
	routes           RouteLookup
	secrets          secrets.Store
	dialer           EgressDialer
	commandAudit     CommandAudit
	config           *ssh.ServerConfig
	handshakeTimeout time.Duration
	allowLAN         bool
}

type ServerOption func(*Server)

func WithCommandAudit(audit CommandAudit) ServerOption {
	return func(server *Server) { server.commandAudit = audit }
}

func WithLANAccess() ServerOption {
	return func(server *Server) { server.allowLAN = true }
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
		go s.serveConnection(connection)
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
		local, requests, err := channel.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(connection, local, requests)
	}
}

func (s *Server) authenticatePassword(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	route, err := s.routes.Lookup(metadata.User())
	if err != nil {
		_ = capability.Verify(string(password), capability.Digest{})
		return nil, ErrAuthentication
	}
	if err := capability.Verify(string(password), route.CapabilityDigest); err != nil {
		return nil, ErrAuthentication
	}
	return routePermissions(route.Alias), nil
}

func (s *Server) authenticatePublicKey(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	route, err := s.routes.Lookup(metadata.User())
	if err != nil {
		return nil, ErrAuthentication
	}
	fingerprint := ssh.FingerprintSHA256(key)
	if _, ok := route.Policy.LocalPublicKeyFingerprints[fingerprint]; !ok {
		return nil, ErrAuthentication
	}
	return routePermissions(route.Alias), nil
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
	if alias == "" || alias != connection.User() {
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
			if done != nil || request.Type != "exec" {
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
				continue
			}

			var payload struct{ Command string }
			route, err := s.routes.Lookup(alias)
			if err != nil || ssh.Unmarshal(request.Payload, &payload) != nil || !route.Policy.AllowsCommand(payload.Command) {
				if err == nil && validCommand(payload.Command) {
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
	if err := session.Start(command); err != nil {
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

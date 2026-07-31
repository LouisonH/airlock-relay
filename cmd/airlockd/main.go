package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/LouisonH/airlock-relay/internal/activity"
	"github.com/LouisonH/airlock-relay/internal/control"
	"github.com/LouisonH/airlock-relay/internal/egress"
	"github.com/LouisonH/airlock-relay/internal/httpgw"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
	"github.com/LouisonH/airlock-relay/internal/securefile"
	"github.com/LouisonH/airlock-relay/internal/sshgw"
	"github.com/LouisonH/airlock-relay/internal/webui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the airlockd version")
	mode := flag.String("mode", "desktop", "runtime mode: desktop or server")
	listenAddress := flag.String("listen", "127.0.0.1:4768", "HTTP relay listen address")
	sshListenAddress := flag.String("ssh-listen", "127.0.0.1:4770", "SSH relay listen address")
	networkScope := flag.String("network-scope", "loopback", "ingress network scope: loopback or lan")
	secretStoreMode := flag.String("secret-store", "", "secret store: keychain or local_file (mode default applies)")
	controlTokenStdin := flag.Bool("control-token-stdin", false, "read the ephemeral desktop control token from stdin")
	dataDir := flag.String("data-dir", "", "absolute protected state directory (required in server mode)")
	controlTokenFile := flag.String("control-token-file", "", "protected 0600 control token file (required in server mode)")
	webListen := flag.String("web-listen", "", "optional loopback Web UI listen address")
	webTokenFile := flag.String("web-token-file", "", "protected 0600 Web UI bearer token file")
	flag.Parse()
	if *showVersion {
		fmt.Printf("airlockd %s\n", version)
		return
	}

	config := runtimeConfig{
		HTTPAddress: *listenAddress, SSHAddress: *sshListenAddress, NetworkScope: *networkScope,
		SecretStoreMode: *secretStoreMode, WebListen: *webListen,
	}
	var err error
	switch *mode {
	case "desktop":
		if !*controlTokenStdin || *controlTokenFile != "" || *dataDir != "" || *webListen != "" || *webTokenFile != "" {
			slog.Error("desktop mode requires --control-token-stdin and does not accept server-mode options")
			os.Exit(2)
		}
		config.ControlToken, err = readControlToken(os.Stdin)
		if err == nil {
			config.ControlPaths, err = control.DefaultPaths()
		}
		if config.SecretStoreMode == "" {
			config.SecretStoreMode = secrets.StoreModeKeychain
		}
	case "server":
		if *controlTokenStdin || *dataDir == "" || *controlTokenFile == "" || !filepath.IsAbs(*dataDir) || (*webListen == "") != (*webTokenFile == "") {
			slog.Error("server mode requires absolute --data-dir and --control-token-file; --web-listen and --web-token-file must be supplied together")
			os.Exit(2)
		}
		config.ControlToken, err = securefile.ReadToken(*controlTokenFile)
		config.ControlPaths, err = control.PathsForDirectory(*dataDir)
		if *webTokenFile != "" && err == nil {
			config.WebToken, err = securefile.ReadToken(*webTokenFile)
		}
		if config.SecretStoreMode == "" {
			config.SecretStoreMode = secrets.StoreModeLocalFile
		}
	default:
		slog.Error("invalid runtime mode")
		os.Exit(2)
	}
	if err != nil {
		slog.Error("airlockd could not load protected control configuration", "error", err)
		os.Exit(2)
	}
	if *networkScope == "lan" {
		if config.HTTPAddress == "127.0.0.1:4768" {
			config.HTTPAddress = "0.0.0.0:4768"
		}
		if config.SSHAddress == "127.0.0.1:4770" {
			config.SSHAddress = "0.0.0.0:4770"
		}
	}
	if err := runWithConfig(config); err != nil {
		slog.Error("airlockd stopped", "error", err)
		os.Exit(1)
	}
}

type runtimeConfig struct {
	HTTPAddress     string
	SSHAddress      string
	NetworkScope    string
	SecretStoreMode string
	ControlToken    string
	ControlPaths    control.Paths
	WebListen       string
	WebToken        string
}

// run remains the desktop-compatible entry point used by existing callers and
// tests. Server deployments use runWithConfig with an explicit data directory.
func run(address, sshAddress, networkScope, secretStoreMode, controlToken string) error {
	controlPaths, err := control.DefaultPaths()
	if err != nil {
		return err
	}
	return runWithConfig(runtimeConfig{
		HTTPAddress: address, SSHAddress: sshAddress, NetworkScope: networkScope,
		SecretStoreMode: secretStoreMode, ControlToken: controlToken, ControlPaths: controlPaths,
	})
}

func runWithConfig(config runtimeConfig) error {
	address, sshAddress, networkScope := config.HTTPAddress, config.SSHAddress, config.NetworkScope
	secretStoreMode, controlToken, controlPaths := config.SecretStoreMode, config.ControlToken, config.ControlPaths
	allowLAN := networkScope == "lan"
	if networkScope != "loopback" && !allowLAN {
		return errors.New("invalid network scope")
	}
	if err := requireAllowedListen(address, allowLAN); err != nil {
		return err
	}
	if err := requireAllowedListen(sshAddress, allowLAN); err != nil {
		return fmt.Errorf("invalid SSH listener: %w", err)
	}
	if err := control.ValidatePaths(controlPaths); err != nil {
		return err
	}
	if len(controlToken) < 32 || len(controlToken) > 128 {
		return errors.New("invalid control token")
	}
	if config.WebListen != "" {
		if err := requireLoopback(config.WebListen); err != nil {
			return fmt.Errorf("invalid Web UI listener: %w", err)
		}
		if len(config.WebToken) < 32 || len(config.WebToken) > 128 {
			return errors.New("invalid Web UI token")
		}
	}
	metadata := routes.NewFileStore(filepath.Join(controlPaths.Directory, "routes.json"))
	persistedRoutes, err := metadata.Load()
	if err != nil {
		return fmt.Errorf("load route metadata: %w", err)
	}
	registry, err := routes.NewRegistry(persistedRoutes...)
	if err != nil {
		return err
	}
	localSecretPath := filepath.Join(controlPaths.Directory, "protected-targets.json")
	secretStore, err := secrets.OpenStore(secretStoreMode, localSecretPath)
	if err != nil {
		return fmt.Errorf("initialize platform secret store: %w", err)
	}
	sshMetadata := sshgw.NewFileStore(filepath.Join(controlPaths.Directory, "ssh-routes.json"))
	persistedSSHRoutes, err := sshMetadata.Load()
	if err != nil {
		return fmt.Errorf("load SSH route metadata: %w", err)
	}
	sshRegistry, err := sshgw.NewRegistry(persistedSSHRoutes...)
	if err != nil {
		return err
	}
	for _, route := range persistedSSHRoutes {
		if _, err := registry.Get(route.Alias); err == nil {
			return errors.New("HTTP and SSH route aliases must be unique")
		}
	}
	hostSigner, err := sshgw.LoadOrCreateHostSigner(context.Background(), secretStore)
	if err != nil {
		return fmt.Errorf("initialize SSH host identity: %w", err)
	}
	commandAudit, err := sshgw.OpenFileCommandAudit(filepath.Join(controlPaths.Directory, "ssh-command-audit.json"))
	if err != nil {
		return fmt.Errorf("initialize SSH command audit: %w", err)
	}
	egressManager := egress.NewManager(nil)
	proxyConfig, err := secretStore.ResolveProxyConfig(context.Background(), egress.DefaultSecretReference)
	if err == nil {
		if err := egressManager.Configure(proxyConfig.URL); err != nil {
			return fmt.Errorf("initialize proxy egress: %w", err)
		}
	} else if !errors.Is(err, secrets.ErrNotFound) {
		return fmt.Errorf("load protected proxy: %w", err)
	}
	activityRecorder := activity.NewMemoryRecorder()
	gateway := httpgw.NewHandler(registry, secretStore, egressManager, httpgw.WithActivityRecorder(activityRecorder))
	sshOptions := []sshgw.ServerOption{sshgw.WithCommandAudit(commandAudit), sshgw.WithActivityRecorder(activityRecorder)}
	if allowLAN {
		sshOptions = append(sshOptions, sshgw.WithLANAccess())
	}
	sshGateway, err := sshgw.NewServer(sshRegistry, secretStore, egressManager, hostSigner, sshOptions...)
	if err != nil {
		return fmt.Errorf("initialize SSH gateway: %w", err)
	}
	sshListener, err := net.Listen("tcp", sshAddress)
	if err != nil {
		return fmt.Errorf("listen for local SSH: %w", err)
	}
	defer sshListener.Close()
	controlListener, controlServer, err := control.ListenWithSSH(
		controlPaths, controlToken, registry, secretStore, metadata, egressManager,
		control.SSHConfiguration{
			Registry: sshRegistry, Persistence: sshMetadata,
			ListenAddress: advertisedAddress(sshListener.Addr()), DialAddress: loopbackAddress(sshListener.Addr()),
			HTTPAddress: advertisedAddressString(address), NetworkScope: networkScope, HostKey: hostSigner.PublicKey(),
			CommandAudit: commandAudit, Activity: activityRecorder,
		},
	)
	if err != nil {
		return fmt.Errorf("initialize control channel: %w", err)
	}
	if err := controlServer.ConfigureSecretStoreMigration(secretStoreMode, func(mode string) (secrets.MutableStore, error) {
		return secrets.OpenStore(mode, localSecretPath)
	}); err != nil {
		_ = controlListener.Close()
		return fmt.Errorf("initialize secret store migration: %w", err)
	}
	defer func() {
		_ = controlListener.Close()
		control.Cleanup(controlPaths)
	}()

	mux := http.NewServeMux()
	mux.Handle("/r/", gateway)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	var webServer *http.Server
	var webErrors chan error
	var webAddress string
	if config.WebListen != "" {
		controlClient, err := control.NewClient(controlPaths.Socket, controlToken)
		if err != nil {
			return fmt.Errorf("initialize Web UI control client: %w", err)
		}
		webHandler, err := webui.New(config.WebToken, controlClient)
		if err != nil {
			return fmt.Errorf("initialize Web UI: %w", err)
		}
		webListener, err := net.Listen("tcp", config.WebListen)
		if err != nil {
			return fmt.Errorf("listen for Web UI: %w", err)
		}
		defer webListener.Close()
		webServer = &http.Server{
			Addr:              config.WebListen,
			Handler:           webHandler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    16 << 10,
		}
		webErrors = make(chan error, 1)
		go func() {
			webErrors <- webServer.Serve(webListener)
		}()
		webAddress = webListener.Addr().String()
	}
	slog.Info("airlockd listening", "http", listener.Addr().String(), "ssh", sshListener.Addr().String(), "web", webAddress)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()
	controlErrors := make(chan error, 1)
	go func() {
		controlErrors <- controlServer.Serve(controlListener)
	}()
	sshErrors := make(chan error, 1)
	go func() {
		sshErrors <- sshGateway.Serve(sshListener)
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), shutdownSignals()...)
	defer stop()
	select {
	case <-shutdownContext.Done():
		_ = sshListener.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if webServer != nil {
			_ = webServer.Shutdown(ctx)
		}
		return server.Shutdown(ctx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-controlErrors:
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("control channel stopped: %w", err)
	case err := <-sshErrors:
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("SSH gateway stopped: %w", err)
	case err := <-webErrors:
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("Web UI stopped: %w", err)
	}
}

func readControlToken(reader io.Reader) (string, error) {
	buffered := bufio.NewReader(io.LimitReader(reader, 256))
	tokenBytes, err := buffered.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("read control token")
	}
	tokenBytes = bytes.TrimSpace(tokenBytes)
	if len(tokenBytes) < 32 || len(tokenBytes) > 128 {
		clear(tokenBytes)
		return "", errors.New("invalid control token")
	}
	token := string(tokenBytes)
	clear(tokenBytes)
	return token, nil
}

func requireLoopback(address string) error {
	return requireAllowedListen(address, false)
}

func requireAllowedListen(address string, allowLAN bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsLoopback() && (!allowLAN || (!ip.IsUnspecified() && !ip.IsPrivate()))) {
		return fmt.Errorf("listen address must use loopback or an allowed private LAN IP")
	}
	return nil
}

func loopbackAddress(address net.Addr) string {
	_, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return address.String()
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func advertisedAddress(address net.Addr) string {
	return advertisedAddressString(address.String())
}

func advertisedAddressString(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsUnspecified() {
		return address
	}
	if lanIP := firstPrivateLANIP(); lanIP != "" {
		return net.JoinHostPort(lanIP, port)
	}
	return net.JoinHostPort("127.0.0.1", port)
}

func firstPrivateLANIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && ip.IsPrivate() {
				return ip.String()
			}
		}
	}
	return ""
}

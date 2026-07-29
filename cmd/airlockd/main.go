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
	"syscall"
	"time"

	"github.com/LouisonH/airlock-relay/internal/control"
	"github.com/LouisonH/airlock-relay/internal/httpgw"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:4768", "HTTP relay listen address")
	controlTokenStdin := flag.Bool("control-token-stdin", false, "read the ephemeral desktop control token from stdin")
	flag.Parse()

	if !*controlTokenStdin {
		slog.Error("airlockd requires an ephemeral desktop control token")
		os.Exit(2)
	}
	controlToken, err := readControlToken(os.Stdin)
	if err != nil {
		slog.Error("airlockd could not read the desktop control token", "error", err)
		os.Exit(2)
	}
	if err := run(*listenAddress, controlToken); err != nil {
		slog.Error("airlockd stopped", "error", err)
		os.Exit(1)
	}
}

func run(address, controlToken string) error {
	if err := requireLoopback(address); err != nil {
		return err
	}
	registry, err := routes.NewRegistry()
	if err != nil {
		return err
	}
	secretStore, err := secrets.NewPlatformStore()
	if err != nil {
		return fmt.Errorf("initialize platform secret store: %w", err)
	}
	gateway := httpgw.NewHandler(registry, secretStore, nil)
	controlPaths, err := control.DefaultPaths()
	if err != nil {
		return err
	}
	controlListener, controlServer, err := control.Listen(controlPaths, controlToken, registry, secretStore)
	if err != nil {
		return fmt.Errorf("initialize control channel: %w", err)
	}
	defer func() {
		_ = controlListener.Close()
		_ = os.Remove(controlPaths.Socket)
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
	slog.Info("airlockd listening", "address", listener.Addr().String())

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()
	controlErrors := make(chan error, 1)
	go func() {
		controlErrors <- controlServer.Serve(controlListener)
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address must use a loopback IP")
	}
	return nil
}

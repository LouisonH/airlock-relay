package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LouisonH/airlock-relay/internal/httpgw"
	"github.com/LouisonH/airlock-relay/internal/routes"
	"github.com/LouisonH/airlock-relay/internal/secrets"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:4768", "HTTP relay listen address")
	flag.Parse()

	if err := run(*listenAddress); err != nil {
		slog.Error("airlockd stopped", "error", err)
		os.Exit(1)
	}
}

func run(address string) error {
	if err := requireLoopback(address); err != nil {
		return err
	}
	registry, err := routes.NewRegistry()
	if err != nil {
		return err
	}
	secretStore := secrets.NewMemoryStore()
	gateway := httpgw.NewHandler(registry, secretStore, nil)

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
	}
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

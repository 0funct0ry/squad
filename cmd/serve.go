package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/0funct0ry/squad/internal/server"
	"github.com/spf13/cobra"
)

// runServeWithGracefulShutdown starts srv's main HTTP listener and blocks
// until SIGINT/SIGTERM, then shuts it down gracefully and stops the REST
// listener (if it was ever started) so its goroutine/socket doesn't leak
// past Ctrl+C. onShutdown runs after both have been stopped, e.g. to close a
// registry or clean up a temp dir.
func runServeWithGracefulShutdown(srv *server.Server, addr string, onShutdown func()) {
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	srv.StopRest()

	if onShutdown != nil {
		onShutdown()
	}
}

// warnIfBroadcastBind prints the same class of warning for any listener
// bound to 0.0.0.0 (all interfaces), recommending a firewall/reverse proxy
// in front of it. label identifies which flag is responsible (e.g. "--addr",
// "--rest-bind-addr").
func warnIfBroadcastBind(label, addr string) {
	if addr == "0.0.0.0" {
		fmt.Printf("  warning: %s is bound to 0.0.0.0 (all interfaces) — consider a firewall rule or reverse proxy, and using --token\n", label)
	}
}

// applyRestEnvOverrides applies SQUAD_REST/SQUAD_REST_PORT/
// SQUAD_REST_BIND_ADDR for any --rest* flag not explicitly passed on the
// command line, preserving flags > env > defaults.
func applyRestEnvOverrides(cmd *cobra.Command, r *restFlags) {
	if !cmd.Flags().Changed("rest") {
		if v := os.Getenv("SQUAD_REST"); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				r.Rest = b
			}
		}
	}
	if !cmd.Flags().Changed("rest-port") {
		if v := os.Getenv("SQUAD_REST_PORT"); v != "" {
			if p, err := strconv.Atoi(v); err == nil {
				r.RestPort = p
			}
		}
	}
	if !cmd.Flags().Changed("rest-bind-addr") {
		if v := os.Getenv("SQUAD_REST_BIND_ADDR"); v != "" {
			r.RestBindAddr = v
		}
	}
}

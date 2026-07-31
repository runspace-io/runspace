package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runspace/runspace/internal/hostagent"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp-proxy" {
		os.Exit(runMCPProxy(os.Args[2:]))
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := hostagent.NewServer(nil)
	if err != nil {
		logger.Error("initialize host agent", "error", err)
		os.Exit(1)
	}
	if err := server.EnableApprovalPersistence(); err != nil {
		logger.Error("load approved host folders", "error", err)
		os.Exit(1)
	}
	go server.RunPresence(ctx, 15*time.Second)
	httpServer := &http.Server{
		Addr:              "127.0.0.1:7799",
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	logger.Info("Runspace host agent listening", "address", "http://127.0.0.1:7799")
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("host agent stopped", "error", err)
		os.Exit(1)
	}
}

func runMCPProxy(args []string) int {
	flags := flag.NewFlagSet("mcp-proxy", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	endpoint := flags.String("url", "", "Runspace workspace MCP endpoint")
	userID := flags.String("user-id", "", "Runspace user identity")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if err := hostagent.RunMCPProxy(
		context.Background(), os.Stdin, os.Stdout, *endpoint, *userID,
		&http.Client{Timeout: 2 * time.Minute},
	); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

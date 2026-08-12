package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jangolova/internal/grimlock"
)

func serveGrimlockMCPCommand(args []string) error {
	flags := flag.NewFlagSet("serve-grimlock-mcp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bind := flags.String("bind", "", "optional MCP Streamable HTTP bind address; empty uses stdio")
	storeDir := flags.String("session-store", envOrDefault("JANGOLOVA_GRIMLOCK_SESSION_STORE", ""), "directory for persistent Grimlock session metadata and events")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve-grimlock-mcp accepts flags only")
	}
	service, err := newGrimlockService(grimlockStoreOption(*storeDir))
	if err != nil {
		return err
	}
	defer service.Close(context.Background())
	mcp, err := grimlock.NewMCPServer(service)
	if err != nil {
		return err
	}
	if *bind == "" {
		return mcp.ServeStdio(context.Background(), os.Stdin, os.Stdout)
	}
	return serveGrimlockProtocolHTTP(*bind, mcp.Routes(), service.Close, "MCP")
}

func serveGrimlockACPCommand(args []string) error {
	flags := flag.NewFlagSet("serve-grimlock-acp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	storeDir := flags.String("session-store", envOrDefault("JANGOLOVA_GRIMLOCK_SESSION_STORE", ""), "directory for persistent Grimlock session metadata and events")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve-grimlock-acp accepts flags only")
	}
	service, err := newGrimlockService(grimlockStoreOption(*storeDir))
	if err != nil {
		return err
	}
	defer service.Close(context.Background())
	acp, err := grimlock.NewACPServer(service)
	if err != nil {
		return err
	}
	return acp.ServeStdio(context.Background(), os.Stdin, os.Stdout)
}

func serveGrimlockProtocolHTTP(bind string, handler http.Handler, closeService func(context.Context) error, name string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: bind, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = closeService(shutdownCtx)
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(os.Stderr, "jangolova Grimlock %s service listening on %s\n", name, bind)
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve Grimlock %s: %w", name, err)
	}
	return nil
}

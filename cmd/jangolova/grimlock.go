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
	"strings"
	"syscall"
	"time"

	"jangolova/internal/builtin"
	"jangolova/internal/grimlock"
	"jangolova/targetconn"
)

func serveGrimlockCommand(args []string) error {
	flags := flag.NewFlagSet("serve-grimlock", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bind := flags.String(
		"bind",
		envOrDefault("JANGOLOVA_GRIMLOCK_BIND", "127.0.0.1:7392"),
		"Grimlock HTTP bind address",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve-grimlock accepts flags only")
	}
	service, err := newGrimlockService()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr: *bind, Handler: service.Routes(), ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = service.Close(shutdownCtx)
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(os.Stderr, "jangolova Grimlock HTTP service listening on %s\n", *bind)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve Grimlock: %w", err)
	}
	return nil
}

func newGrimlockService() (*grimlock.Service, error) {
	token := strings.TrimSpace(os.Getenv("JANGOLOVA_GRIMLOCK_TOKEN"))
	if token == "" {
		return nil, errors.New("JANGOLOVA_GRIMLOCK_TOKEN is required")
	}
	resolver := targetconn.DefaultResolver()
	runtime, err := grimlock.NewDefaultRuntime(resolver)
	if err != nil {
		return nil, err
	}
	registry, err := builtin.EngineRegistry()
	if err != nil {
		return nil, err
	}
	return grimlock.NewService(runtime, registry, token, grimlock.WithTargetResolver(resolver))
}

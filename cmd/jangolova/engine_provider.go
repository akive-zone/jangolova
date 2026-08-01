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
	"jangolova/internal/engineprovider"
)

func serveEngineProviderCommand(args []string) error {
	flags := flag.NewFlagSet("serve-engine-provider", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bind := flags.String(
		"bind",
		envOrDefault("JANGOLOVA_PROVIDER_BIND", "127.0.0.1:7391"),
		"engine-provider HTTP bind address",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve-engine-provider accepts flags only")
	}
	token := strings.TrimSpace(os.Getenv("JANGOLOVA_PROVIDER_TOKEN"))
	if token == "" {
		return errors.New("JANGOLOVA_PROVIDER_TOKEN is required")
	}
	registry, err := builtin.EngineRegistry()
	if err != nil {
		return err
	}
	provider, err := engineprovider.NewService(registry, token)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	server := &http.Server{
		Addr:              *bind,
		Handler:           provider.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
		defer cancel()
		_ = provider.Close(shutdownCtx)
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(os.Stderr, "jangolova engine provider listening on %s\n", *bind)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve engine provider: %w", err)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

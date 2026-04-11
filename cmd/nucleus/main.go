package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/blueai2022/nucleus/internal/config"
	"github.com/blueai2022/nucleus/internal/service"
	"github.com/blueai2022/nucleus/internal/session"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	settings, err := config.New()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("failed to create settings")

		return
	}

	log.Debug().
		Any("settings", settings).
		Msg("loaded configuration")

	mux := http.NewServeMux()

	// Create Anthropic client (reads ANTHROPIC_API_KEY from env)
	client := anthropic.NewClient()

	// Create session manager with client and workspace root
	sessionManager := session.NewManager(client, settings.WorkspaceRoot)

	svc, err := service.New(sessionManager)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create service")
		return
	}

	path, connectHandler := svc.ConnectHandler()
	mux.Handle(path, connectHandler)

	path, vanguardHandler := svc.VanguardHandler()
	mux.Handle(path, vanguardHandler)

	httpServer := &http.Server{
		Addr:    settings.HTTP.Address(),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go func() {
		log.Info().
			Str("address", settings.HTTP.Address()).
			Msg("starting HTTP server")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().
				Err(err).
				Msg("HTTP server exited with error")
		}
	}()

	defer cancel()

	<-ctx.Done()

	log.Warn().
		Str("event.action", "shutdown").
		Msg("shutting down servers")

	if err := httpServer.Shutdown(context.Background()); err != nil {
		log.Error().
			Err(err).
			Msg("HTTP server shutdown error")
	}

	log.Info().
		Str("event.action", "shutdown").
		Msg("closing active WebSocket connections")

	log.Info().
		Str("event.action", "shutdown").
		Msg("shutdown complete")

}

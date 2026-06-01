package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/config"
)

type namedServer struct {
	name   string
	server *http.Server
}

func newHTTPServers(cfg config.Config, mainHandler, adminHandler http.Handler) []namedServer {
	servers := make([]namedServer, 0, len(cfg.MainBindAddrs)+len(cfg.AdminBindAddrs))
	for _, addr := range cfg.MainBindAddrs {
		servers = append(servers, namedServer{
			name: "main api and viewer",
			server: &http.Server{
				Addr:              addr,
				Handler:           mainHandler,
				ReadHeaderTimeout: cfg.MainTimeouts.ReadHeaderTimeout,
				ReadTimeout:       cfg.MainTimeouts.ReadTimeout,
				WriteTimeout:      cfg.MainTimeouts.WriteTimeout,
				IdleTimeout:       cfg.MainTimeouts.IdleTimeout,
			},
		})
	}
	for _, addr := range cfg.AdminBindAddrs {
		servers = append(servers, namedServer{
			name: "private admin",
			server: &http.Server{
				Addr:              addr,
				Handler:           adminHandler,
				ReadHeaderTimeout: cfg.AdminTimeouts.ReadHeaderTimeout,
				ReadTimeout:       cfg.AdminTimeouts.ReadTimeout,
				WriteTimeout:      cfg.AdminTimeouts.WriteTimeout,
				IdleTimeout:       cfg.AdminTimeouts.IdleTimeout,
			},
		})
	}
	return servers
}

func startServer(errCh chan<- error, logger *slog.Logger, named namedServer) {
	go func() {
		logger.Info("starting "+named.name+" server", "addr", named.server.Addr)
		if err := named.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("%s server %s: %w", named.name, named.server.Addr, err)
		}
	}()
}

func shutdownServers(servers []namedServer) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var shutdownErr error
	for _, named := range servers {
		if err := named.server.Shutdown(shutdownCtx); err != nil && shutdownErr == nil {
			shutdownErr = err
		}
	}
	return shutdownErr
}

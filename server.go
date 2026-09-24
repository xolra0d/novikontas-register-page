package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xolra0d/novikontas-register-page/shared/pkg/middleware"
)

// RunServer starts HTTP server and handles exit.
func RunServer(mux *http.ServeMux, csrf, cors middleware.Middleware, logger *slog.Logger, runningAddr string, shutdownTimeout time.Duration, dbClose func()) {
	const op = "server.RunServer"

	server := &http.Server{
		Addr: runningAddr,
		Handler: middleware.Chain(
			mux,
			csrf,
			cors,
			middleware.Logging(logger),
		),
	}

	go func() {
		logger.Info("starting HTTP server", "addr", runningAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen error", "error", err, "op", op)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	<-shutdown
	logger.Info("shutdown initiated")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "op", op, "error", err)
	}

	dbClose()

	logger.Info("HTTP server stopped")
}

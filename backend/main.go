package main

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/xolra0d/novikontas-register-page/backend/shared/pkg/logger"
	"github.com/xolra0d/novikontas-register-page/backend/shared/pkg/middleware"
)

func main() {
	cfg := LoadConfig()
	l := slog.New(logger.NewHandler(nil))
	h := NewHandles(l)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ok", h.Ping)
	cors := middleware.NewCors(
		strings.Split(cfg.AllowedOrigins, ","),
		[]string{"GET", "POST", "OPTIONS"},
		[]string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		true,
	)
	csrf := middleware.NewCSRF(strings.Split(cfg.AllowedOrigins, ","))

	RunServer(mux, csrf, cors, l, cfg.RunningAddr, cfg.ShutdownTimeout)

}

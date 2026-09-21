package main

import (
	"log/slog"
	"net/http"

	"github.com/xolra0d/novikontas-register-page/backend/shared/pkg/api"
)

type Handles struct {
	logger *slog.Logger
}

func NewHandles(logger *slog.Logger) *Handles {
	return &Handles{logger: logger}
}

// Ping handles /ok requests
func (h *Handles) Ping(w http.ResponseWriter, _ *http.Request) {
	const op = "main.Ping"

	err := api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	if err != nil {
		h.logger.Error("could not write response", "op", op, "err", err)
		return
	}
}

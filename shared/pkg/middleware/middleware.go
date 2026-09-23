package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/xolra0d/novikontas-register-page/shared/pkg/api"
)

type Middleware func(http.Handler) http.Handler

// Chain chains multiple m middlewares before h handler
func Chain(h http.Handler, m ...Middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

func NewCors(allowedOrigins, allowedMethods, allowedHeaders []string, allowCredentials bool) Middleware {
	return cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   allowedMethods,
		AllowedHeaders:   allowedHeaders,
		AllowCredentials: allowCredentials,
	}).Handler
}

func NewCSRF(allowedOrigins []string) Middleware {
	base := http.NewCrossOriginProtection()
	for _, origin := range allowedOrigins {
		base.AddTrustedOrigin(origin)
	}
	return base.Handler
}

const (
	LoginCookieName = "login_token"
	LoginContextKey = "login"
)

// AuthJWT reads `Authorization` cookie and checks if it is valid.
func AuthJWT(logger *slog.Logger, validate func(tokenString string) (username string, err error)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(LoginCookieName)
			if err != nil {
				if err == http.ErrNoCookie {
					err := api.WriteJSON(w, http.StatusUnauthorized, map[string]any{"err": "unauthorized"})
					if err != nil {
						logger.Error("could not write response", "err", err)
					}
					return
				}
				writeErr := api.WriteJSON(w, http.StatusBadRequest, map[string]any{"err": err})
				if writeErr != nil {
					logger.Error("could not write response", "err", writeErr)
				}
				return
			}
			token := cookie.Value
			username, err := validate(token)
			if err != nil {
				writeErr := api.WriteJSON(w, http.StatusBadRequest, map[string]any{"err": err})
				if writeErr != nil {
					logger.Error("could not write response", "err", writeErr)
				}
				return
			}

			ctx := context.WithValue(r.Context(), LoginContextKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Logging Logs http each request.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("got request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
		})
	}
}

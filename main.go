package main

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/stripe/stripe-go/v86"
	"github.com/xolra0d/novikontas-register-page/shared/pkg/logger"
	"github.com/xolra0d/novikontas-register-page/shared/pkg/middleware"
)

func main() {
	cfg := LoadConfig()
	stripe.Key = cfg.StripeSecretKey
	l := slog.New(logger.NewHandler(nil))
	d := NewDatabase(cfg.PostgresURL, l)
	h := NewHandles(d, l, cfg.HomeTemplate, cfg.CourseDetailsTemplate, cfg.PublicURL, cfg.StripeWebhookSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ok", h.Ping)
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.FS(*cfg.ImagesFS))))
	mux.HandleFunc("GET /favicon.ico", http.NotFound)
	mux.HandleFunc("GET /", h.ListCourses)
	mux.HandleFunc("GET /course/{id}", h.CourseDetails)
	mux.HandleFunc("GET /courses/{id}/enroll", h.EnrollmentForm)
	mux.HandleFunc("POST /courses/{id}/enroll", h.SubmitEnrollment)
	mux.HandleFunc("POST /stripe/webhook", h.StripeWebhook)

	cors := middleware.NewCors(
		strings.Split(cfg.AllowedOrigins, ","),
		[]string{"GET", "POST", "OPTIONS"},
		[]string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		true,
	)
	csrf := middleware.NewCSRF(strings.Split(cfg.AllowedOrigins, ","))

	RunServer(mux, csrf, cors, l, cfg.RunningAddr, cfg.ShutdownTimeout)

}

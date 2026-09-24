package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"time"

	"github.com/xolra0d/novikontas-register-page/shared/pkg/config"
)

//go:embed static/images
var imagesFS embed.FS

//go:embed static/html/base.tmpl
var BaseTemplate string

//go:embed static/html/home.tmpl
var HomeTemplate string

type Course struct {
	ID          int64
	Name        string
	Description string
	Image       string
	Locations   []string
	Price       int64
	Available   bool
}

type HomeTemplateConfig struct {
	Title   string
	Courses []Course
}

//go:embed static/html/course_details.tmpl
var CourseDetailsTemplate string

type CourseDetailsConfig struct {
	Title          string
	Course         Course
	PaymentSuccess bool
}

type Config struct {
	// DATABASE
	PostgresURL string // Env name: `POSTGRES_URL`. PostgreSQL connection string. Default: none, will panic, if not set.

	// STRIPE
	StripeSecretKey     string // Env name: `STRIPE_SECRET_KEY`.
	StripeWebhookSecret string // Env name: `STRIPE_WEBHOOK_SECRET`.
	PublicURL           string // Env name: `PUBLIC_URL`.

	// GOOGLE SHEETS
	GoogleCredentialsFile string // Env name: `GOOGLE_CREDENTIALS_FILE`.
	GoogleSpreadsheetID   string // Env name: `GOOGLE_SPREADSHEET_ID`.
	GoogleSheetRange      string // Env name: `GOOGLE_SHEET_RANGE`.

	// HTTP
	AllowedOrigins  string        // Env name: `ALLOWED_ORIGINS`. Origins to respond to (e.g., http://website.com:12), separated by comma. Default: none, will exit, if not set.
	RunningAddr     string        // Env name: `RUNNING_ADDR`. Address to run web on. Default: `:8080`.
	ShutdownTimeout time.Duration // Env name: `SHUTDOWN_TIMEOUT`. Time for transport server to shut down in seconds. Default: 10.

	// STATIC
	HomeTemplate          *template.Template // built from static files.
	CourseDetailsTemplate *template.Template // built from static files.
	ImagesFS              *fs.FS             // built from static files.
}

func LoadConfig() *Config {
	postgresURL := config.GetEnvOrExit("POSTGRES_URL")
	stripeSecretKey := config.GetEnvOrExit("STRIPE_SECRET_KEY")
	stripeWebhookSecret := config.GetEnvOrExit("STRIPE_WEBHOOK_SECRET")
	publicURL := config.GetEnvOrExit("PUBLIC_URL")
	googleCredentialsFile := config.GetEnvOrExit("GOOGLE_CREDENTIALS_FILE")
	googleSpreadsheetID := config.GetEnvOrExit("GOOGLE_SPREADSHEET_ID")
	googleSheetRange := config.GetEnvOrFallback("GOOGLE_SHEET_RANGE", "Sheet1!A:E")
	allowedOrigins := config.GetEnvOrExit("ALLOWED_ORIGINS")
	runningAddr := config.GetEnvOrFallback("RUNNING_ADDR", ":8080")
	shutdownTimeout := config.StringToSeconds("SHUTDOWN_TIMEOUT", config.GetEnvOrFallback("SHUTDOWN_TIMEOUT", "10"))

	base := template.Must(template.New("Base").Parse(BaseTemplate))

	homeTmpl := template.Must(base.Clone())
	funcMap := template.FuncMap{
		"formatPrice": func(price int64) string {
			return fmt.Sprintf("%d.%02d", price/100, price%100)
		},
	}
	homeTmpl = template.Must(homeTmpl.Funcs(funcMap).Parse(HomeTemplate))

	courseDetailsTmpl := template.Must(base.Clone())
	funcMap = template.FuncMap{
		"formatPrice": func(price int64) string {
			return fmt.Sprintf("%d.%02d", price/100, price%100)
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}
	courseDetailsTmpl = template.Must(courseDetailsTmpl.Funcs(funcMap).Parse(CourseDetailsTemplate))

	sub, err := fs.Sub(imagesFS, "static/images")
	if err != nil {
		log.Fatalf("Failed to initiate images fs. Error: %v", err.Error())
	}

	return &Config{
		PostgresURL: postgresURL,

		StripeSecretKey:     stripeSecretKey,
		StripeWebhookSecret: stripeWebhookSecret,
		PublicURL:           publicURL,

		GoogleCredentialsFile: googleCredentialsFile,
		GoogleSpreadsheetID:   googleSpreadsheetID,
		GoogleSheetRange:      googleSheetRange,

		AllowedOrigins:  allowedOrigins,
		RunningAddr:     runningAddr,
		ShutdownTimeout: shutdownTimeout,

		HomeTemplate:          homeTmpl,
		CourseDetailsTemplate: courseDetailsTmpl,
		ImagesFS:              &sub,
	}

}

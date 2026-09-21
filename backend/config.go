package main

import (
	"time"

	"github.com/xolra0d/novikontas-register-page/backend/shared/pkg/config"
)

type Config struct {
	AllowedOrigins  string        // Env name: `ALLOWED_ORIGINS`. Origins to respond to (e.g., http://website.com:12), separated by comma. Default: none, will exit, if not set.
	RunningAddr     string        // Env name: `RUNNING_ADDR`. Address to run web on. Default: `:8080`.
	ShutdownTimeout time.Duration // Env name: `SHUTDOWN_TIMEOUT`. Time for transport server to shut down in seconds. Default: 10.
}

func LoadConfig() *Config {
	allowedOrigins := config.GetEnvOrExit("ALLOWED_ORIGINS")
	runningAddr := config.GetEnvOrFallback("RUNNING_ADDR", ":8080")
	shutdownTimeout := config.StringToSeconds("SHUTDOWN_TIMEOUT", config.GetEnvOrFallback("SHUTDOWN_TIMEOUT", "10"))

	return &Config{
		AllowedOrigins:  allowedOrigins,
		RunningAddr:     runningAddr,
		ShutdownTimeout: shutdownTimeout,
	}

}

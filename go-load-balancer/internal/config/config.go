// Package config parses the load balancer's runtime configuration from
// CLI flags and environment variables (stdlib flag package only — no
// CLI framework). This is plumbing, not decision logic, so it is
// deliberately not a TDD seam (see README.md §3) — it's implemented
// directly rather than test-first.
package config

import (
	"flag"
	"os"
	"strings"
	"time"
)

// Config is the load balancer's runtime configuration.
type Config struct {
	// ListenAddr is the address the load balancer listens on, e.g. ":8080".
	ListenAddr string
	// Backends is the static list of backend base URLs, e.g.
	// []string{"http://backend-1:8080", "http://backend-2:8080"}.
	Backends []string
	// HealthCheckPath is the path polled on each backend, e.g. "/health".
	HealthCheckPath string
	// HealthCheckInterval is how often each backend is polled.
	HealthCheckInterval time.Duration
}

// Load parses configuration from args (typically os.Args[1:]).
//
// Precedence for every setting is: explicit CLI flag > environment
// variable > built-in default.
//
//	flag                     env var                default
//	-listen                  LISTEN_ADDR            ":8080"
//	-backends                BACKENDS               "" (required, comma-separated)
//	-health-check-path       HEALTH_CHECK_PATH       "/health"
//	-health-check-interval   HEALTH_CHECK_INTERVAL   "5s" (time.ParseDuration syntax)
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("loadbalancer", flag.ContinueOnError)

	listenAddr := fs.String("listen", getEnv("LISTEN_ADDR", ":8080"), "address to listen on")
	backends := fs.String("backends", getEnv("BACKENDS", ""), "comma-separated backend URLs")
	healthCheckPath := fs.String("health-check-path", getEnv("HEALTH_CHECK_PATH", "/health"), "path polled on each backend")
	healthCheckInterval := fs.String("health-check-interval", getEnv("HEALTH_CHECK_INTERVAL", "5s"), "interval between active health checks")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	interval, err := time.ParseDuration(*healthCheckInterval)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:          *listenAddr,
		HealthCheckPath:     *healthCheckPath,
		HealthCheckInterval: interval,
	}
	for _, b := range strings.Split(*backends, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			cfg.Backends = append(cfg.Backends, b)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Command loadbalancer runs the Application Load Balancer: it reads its
// backend list from config, starts the active health checker, and
// serves incoming HTTP requests via the reverse-proxy handler.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-load-balancer/internal/balancer"
	"go-load-balancer/internal/config"
	"go-load-balancer/internal/healthcheck"
	"go-load-balancer/internal/proxy"
)

const shutdownGrace = 5 * time.Second

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Fatalf("config: %v", err)
	}
	if len(cfg.Backends) == 0 {
		log.Fatal("config: no BACKENDS configured")
	}

	var backends []*balancer.Backend
	for _, raw := range cfg.Backends {
		b, err := balancer.NewBackend(raw)
		if err != nil {
			log.Fatalf("config: invalid backend %q: %v", raw, err)
		}
		backends = append(backends, b)
	}
	pool := balancer.NewPool(backends)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	checker := healthcheck.NewChecker(pool, cfg.HealthCheckPath, cfg.HealthCheckInterval)
	go checker.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", proxy.NewHandler(pool))
	mux.HandleFunc("/lb-health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("load balancer listening on %s, backends=%v", cfg.ListenAddr, cfg.Backends)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

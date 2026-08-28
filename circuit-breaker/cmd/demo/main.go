// Command demo wires an independent Breaker around each of several
// real, separately-running upstream services (see cmd/upstream) and
// logs every breaker's state on every call. Because
// breaker.Breaker.Execute is still a stub (see internal/breaker),
// every call currently logs a "not implemented" error — that's
// expected until the breaker's TDD seams are implemented. Each
// breaker is also wired to a translog.Logger (see internal/translog)
// via Config.OnTransition, so state changes are recorded to Postgres
// once that seam is implemented too; until then, LogTransition's own
// "not implemented" error is logged, not fatal.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"circuit-breaker/internal/breaker"
	"circuit-breaker/internal/translog"
)

// target pairs one upstream with its own breaker, so a healthy, a
// flaky, and a down upstream can be watched independently in the same
// run instead of sharing state.
type target struct {
	name string
	url  string
	cb   *breaker.Breaker
}

func main() {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	upstreamsRaw := fs.String("upstreams", getEnv("UPSTREAMS", "default=http://localhost:8080"),
		"comma-separated name=url pairs; each gets its own breaker")
	failureThreshold := fs.Int("failure-threshold", 3, "consecutive failures before a breaker opens")
	openTimeout := fs.Duration("open-timeout", 2*time.Second, "how long a breaker stays open before a half-open trial")
	calls := fs.Int("calls", 30, "number of calls to make per upstream")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Fatalf("config: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatalf("config: DATABASE_URL is required")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer conn.Close(ctx)

	logger := translog.NewLogger(conn)

	cfg := breaker.NewConfig(*failureThreshold, *openTimeout)

	targets, err := parseUpstreams(ctx, *upstreamsRaw, cfg, logger)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := &http.Client{Timeout: 1 * time.Second}

	for i := 1; i <= *calls; i++ {
		for _, t := range targets {
			err := t.cb.Execute(func() error {
				resp, err := client.Get(t.url)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				io.Copy(io.Discard, resp.Body)
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("upstream returned %s", resp.Status)
				}
				return nil
			})

			if err != nil {
				log.Printf("upstream=%s call=%d state=%s result=error(%v)", t.name, i, t.cb.State(), err)
			} else {
				log.Printf("upstream=%s call=%d state=%s result=ok", t.name, i, t.cb.State())
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// parseUpstreams builds one target per name=url pair, each with its own
// Breaker. cfg is shared as a starting point, but each breaker gets its
// own copy with OnTransition set to a closure over that breaker's name,
// so the audit log can tell breakers apart.
func parseUpstreams(ctx context.Context, raw string, cfg breaker.Config, logger *translog.Logger) ([]*target, error) {
	var targets []*target
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, url, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid -upstreams entry %q, want name=url", pair)
		}

		targetCfg := cfg
		targetCfg.OnTransition = func(from, to breaker.State) {
			if err := logger.LogTransition(ctx, name, from.String(), to.String()); err != nil {
				log.Printf("upstream=%s translog: %v", name, err)
			}
		}

		targets = append(targets, &target{name: name, url: url, cb: breaker.NewBreaker(targetCfg)})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no upstreams configured")
	}
	return targets, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

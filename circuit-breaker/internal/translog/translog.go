// Package translog is the persistence side of the circuit breaker's
// audit trail: it records, in Postgres, that a breaker's state
// actually changed. It is deliberately separate from package breaker —
// the breaker's decision-making stays pure and in-memory; this package
// only writes down what already happened, after the fact.
package translog

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Logger writes breaker state transitions to Postgres, one row per
// transition, into the breaker_transitions table (see
// infra/postgres/init/03_circuit_breaker.sql).
type Logger struct {
	conn *pgx.Conn
}

// NewLogger wraps an already-connected *pgx.Conn in a Logger. It does
// not own the connection's lifecycle — the caller is responsible for
// closing conn.
func NewLogger(conn *pgx.Conn) *Logger {
	return &Logger{conn: conn}
}

// LogTransition records that breakerName moved from one state to
// another.
//
// TODO(TDD): implement test-first against a real Postgres instance
// (see README.md §3, seam 4). Once implemented, this should execute:
//
//	INSERT INTO breaker_transitions (breaker_name, from_state, to_state)
//	VALUES ($1, $2, $3)
func (l *Logger) LogTransition(ctx context.Context, breakerName string, from, to string) error {
	return errors.New("not implemented")
}

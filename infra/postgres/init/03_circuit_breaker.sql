-- circuit-breaker: append-only audit log of state transitions. The
-- breaker's live decision-making (Execute, State) stays entirely
-- in-memory for speed — this table is a side-effect log written
-- whenever a breaker's state actually changes, not the source of
-- truth for its current state.
-- Insert-only table: rows are never updated, so updated_at will
-- typically just equal created_at. Kept anyway for consistency with
-- every other table in this schema.
CREATE TABLE IF NOT EXISTS breaker_transitions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    breaker_name TEXT NOT NULL,
    from_state   TEXT NOT NULL,
    to_state     TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_breaker_transitions_name_time
    ON breaker_transitions (breaker_name, occurred_at);

CREATE INDEX IF NOT EXISTS idx_breaker_transitions_created_at ON breaker_transitions (created_at);
CREATE INDEX IF NOT EXISTS idx_breaker_transitions_updated_at ON breaker_transitions (updated_at);

DROP TRIGGER IF EXISTS trg_breaker_transitions_set_updated_at ON breaker_transitions;
CREATE TRIGGER trg_breaker_transitions_set_updated_at
    BEFORE UPDATE ON breaker_transitions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

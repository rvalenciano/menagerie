-- consistent-hash-ring: persisted node membership registry. Append-only
-- audit log, not a mutable row per node: AddNode inserts a row;
-- RemoveNode sets removed_at on the currently-active row for that
-- name (never deletes). The ring's actual hash structure is still
-- computed in memory on startup from the currently-active rows here —
-- only membership is persisted, not the derived ring math.
CREATE TABLE IF NOT EXISTS ring_nodes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A node name can only be active (removed_at IS NULL) once at a time,
-- but can be re-added later as a new row after being removed.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ring_nodes_active_name
    ON ring_nodes (name)
    WHERE removed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ring_nodes_created_at ON ring_nodes (created_at);
CREATE INDEX IF NOT EXISTS idx_ring_nodes_updated_at ON ring_nodes (updated_at);

DROP TRIGGER IF EXISTS trg_ring_nodes_set_updated_at ON ring_nodes;
CREATE TRIGGER trg_ring_nodes_set_updated_at
    BEFORE UPDATE ON ring_nodes
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

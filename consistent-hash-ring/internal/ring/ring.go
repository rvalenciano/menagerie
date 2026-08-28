// Package ring implements a consistent-hashing ring: it maps string keys
// to one of a set of registered node names, using virtual nodes so that
// adding or removing a node only remaps a small, roughly 1/N fraction of
// keys instead of the whole keyspace.
//
// Node membership (which names are currently registered) is persisted to
// a Postgres ring_nodes table so it survives process restarts; the ring's
// hash math/lookup itself (sortedHashes, hashToNode, Get) stays entirely
// in-memory and pure — only AddNode/RemoveNode gain a persistence
// side-effect. On startup, callers rebuild the in-memory ring by loading
// the active node list (LoadActiveNodeNames) and replaying it through
// AddNode.
package ring

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNoNodes is returned by Get when the ring has no nodes registered.
var ErrNoNodes = errors.New("ring: no nodes registered")

// Ring is a consistent-hashing ring over a set of named nodes. Each node
// is represented on the ring by virtualNodesPerNode points, which
// smooths out the key distribution across nodes. Not safe for
// concurrent use.
type Ring struct {
	virtualNodesPerNode int
	sortedHashes        []uint32
	hashToNode          map[uint32]string
	db                  *pgx.Conn
}

// NewRing builds an empty Ring where every node added is represented by
// virtualNodesPerNode points on the ring. The ring has no database
// connection: AddNode/RemoveNode will only ever mutate in-memory state,
// which is what unit tests want. Use NewRingWithDB for a ring whose
// membership changes are also persisted.
func NewRing(virtualNodesPerNode int) *Ring {
	return &Ring{
		virtualNodesPerNode: virtualNodesPerNode,
		hashToNode:          make(map[uint32]string),
	}
}

// NewRingWithDB builds an empty Ring like NewRing, but wires it to db so
// that AddNode/RemoveNode also persist membership changes to the
// ring_nodes table. Callers are still responsible for populating the
// ring on startup, e.g. by loading LoadActiveNodeNames and calling
// AddNode for each name.
func NewRingWithDB(virtualNodesPerNode int, db *pgx.Conn) *Ring {
	return &Ring{
		virtualNodesPerNode: virtualNodesPerNode,
		hashToNode:          make(map[uint32]string),
		db:                  db,
	}
}

// LoadActiveNodeNames returns the names of all currently-active nodes
// (removed_at IS NULL) from the ring_nodes table, ordered by added_at.
// It does not touch the in-memory ring at all — wiring the returned
// names into a Ring via AddNode is the caller's job.
func LoadActiveNodeNames(ctx context.Context, db *pgx.Conn) ([]string, error) {
	rows, err := db.Query(ctx, "SELECT name FROM ring_nodes WHERE removed_at IS NULL ORDER BY added_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

// AddNode registers name on the ring, adding virtualNodesPerNode points
// for it.
//
// TODO(TDD): implement test-first (seam 2 in README.md §3). Once
// implemented, this should do two things:
//  1. In-memory (pure, existing responsibility, unchanged): add
//     virtualNodesPerNode points for name into sortedHashes/hashToNode.
//  2. Persistence (new): if r.db is non-nil,
//     `INSERT INTO ring_nodes (name) VALUES ($1)`. Re-adding a
//     previously-removed name is expected to succeed as a new row —
//     the partial unique index on ring_nodes only guards one *active*
//     row per name (removed_at IS NULL), so a prior removed row doesn't
//     block a fresh insert.
func (r *Ring) AddNode(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

// RemoveNode removes name and all of its virtual points from the ring.
//
// TODO(TDD): implement test-first (seam 2 in README.md §3). Once
// implemented, this should do two things:
//  1. In-memory (pure, existing responsibility, unchanged): remove all
//     of name's points from sortedHashes/hashToNode.
//  2. Persistence (new): if r.db is non-nil,
//     `UPDATE ring_nodes SET removed_at = now() WHERE name = $1 AND
//     removed_at IS NULL` to close out the currently-active row for
//     name without deleting history.
func (r *Ring) RemoveNode(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

// Get returns the name of the node that owns key: the node whose
// nearest virtual point on the ring is at or after hash(key), walking
// clockwise. Returns ErrNoNodes if the ring has no nodes registered.
//
// TODO(TDD): implement test-first against internal/ring/ring_test.go
// (seam 1 in README.md §3). This is the shell's first red test.
//
// Expected shape: return ErrNoNodes if len(r.sortedHashes) == 0.
// Otherwise hash(key) with the same hash function used to place
// virtual nodes in AddNode, then binary-search r.sortedHashes (it's
// kept sorted) for the first entry >= that hash — sort.Search is the
// standard-library tool for this. If every entry is smaller (the hash
// falls past the end), wrap around to index 0 — that's the "ring" part.
// Look up r.hashToNode[found hash] and return it.
func (r *Ring) Get(key string) (string, error) {
	return "", errors.New("not implemented")
}

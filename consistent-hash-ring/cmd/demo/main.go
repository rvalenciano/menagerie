// Command demo connects to Postgres, builds a consistent-hash ring from
// a node list (showing what's currently persisted in ring_nodes), prints
// the key->node assignment for a set of sample keys, then adds and
// removes a node and reports how many keys changed owner each time.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"consistent-hash-ring/internal/ring"
)

func main() {
	nodesFlag := flag.String("nodes", "node-1,node-2,node-3", "comma-separated list of node names")
	keysFlag := flag.String("keys", "", "comma-separated list of sample keys (default: generates key-0..key-999)")
	vnodesFlag := flag.Int("vnodes", 100, "virtual nodes per physical node")
	sampleFlag := flag.Int("sample", 1000, "number of generated sample keys when -keys is not given")
	flag.Parse()

	nodes := splitCSV(*nodesFlag)
	if len(nodes) == 0 {
		log.Fatal("no nodes given; pass -nodes=node-1,node-2,...")
	}

	keys := splitCSV(*keysFlag)
	if len(keys) == 0 {
		keys = generateKeys(*sampleFlag)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set; refusing to start without it")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer conn.Close(ctx)

	persisted, err := ring.LoadActiveNodeNames(ctx, conn)
	if err != nil {
		log.Fatalf("loading active node names: %v", err)
	}
	fmt.Printf("persisted active nodes in ring_nodes: %v\n\n", persisted)

	r := ring.NewRingWithDB(*vnodesFlag, conn)
	for _, n := range nodes {
		if err := r.AddNode(ctx, n); err != nil {
			log.Printf("AddNode(%q): %v (expected until seam 2 is implemented test-first)", n, err)
		}
	}

	fmt.Printf("nodes: %v (vnodes/node=%d)\n", nodes, *vnodesFlag)
	fmt.Printf("keys: %d\n\n", len(keys))

	before := assign(r, keys)
	printSample(before, keys, 10)

	newNode := "node-new"
	fmt.Printf("\n--- adding %q ---\n", newNode)
	if err := r.AddNode(ctx, newNode); err != nil {
		log.Printf("AddNode(%q): %v (expected until seam 2 is implemented test-first)", newNode, err)
	}
	after := assign(r, keys)
	reportChurn(before, after, len(nodes)+1)

	fmt.Printf("\n--- removing %q ---\n", newNode)
	if err := r.RemoveNode(ctx, newNode); err != nil {
		log.Printf("RemoveNode(%q): %v (expected until seam 2 is implemented test-first)", newNode, err)
	}
	final := assign(r, keys)
	reportChurn(after, final, len(nodes))
}

// splitCSV splits a comma-separated string into trimmed, non-empty
// fields.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// generateKeys returns n sample keys: key-0, key-1, ..., key-(n-1).
func generateKeys(n int) []string {
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	return keys
}

// assign maps every key to its owning node (or its error, formatted as
// "error: ...", if the ring can't answer yet).
func assign(r *ring.Ring, keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		node, err := r.Get(k)
		if err != nil {
			out[k] = "error: " + err.Error()
			continue
		}
		out[k] = node
	}
	return out
}

// printSample prints up to n key->owner assignments.
func printSample(assignments map[string]string, keys []string, n int) {
	if n > len(keys) {
		n = len(keys)
	}
	fmt.Println("sample assignment:")
	for _, k := range keys[:n] {
		fmt.Printf("  %s -> %s\n", k, assignments[k])
	}
}

// reportChurn compares two key->owner assignments and prints how many
// keys changed owner, and what fraction that is of the total. With a
// working ring implementation, adding/removing one node out of N should
// remap roughly 1/N of keys, not all of them.
func reportChurn(before, after map[string]string, n int) {
	changed := 0
	for k, oldOwner := range before {
		if after[k] != oldOwner {
			changed++
		}
	}
	total := len(before)
	fraction := 0.0
	if total > 0 {
		fraction = float64(changed) / float64(total)
	}
	fmt.Printf("keys changed owner: %d/%d (%.1f%%), expected roughly 1/%d = %.1f%%\n",
		changed, total, fraction*100, n, 100.0/float64(n))
}

// Command demo is a tiny in-memory key-value store backed by the WAL:
// every put/delete is appended to the log before being applied to the
// in-memory map, and on startup the map is rebuilt by replaying the
// log. It exists to give the WAL something concrete to prove itself
// against — the interesting demonstration isn't "does it run," it's
// "does it survive being killed mid-write":
//
//	./demo -log /tmp/demo.wal stress -n 1000000 &
//	sleep 0.2 && kill -9 $!                    # simulate a crash mid-append
//	./demo -log /tmp/demo.wal dump | wc -l     # some N <= 1000000, no error
//
// Because wal.Log's Append/Replay are still stubs, every subcommand
// below currently just surfaces their "not implemented" error — that
// becomes a real, meaningful chaos test once you implement them.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"wal/internal/wal"
)

// op is what actually gets appended to the log. JSON is a deliberate
// choice for the DEMO's own record format — encoding an op is not the
// WAL's job (that's frame.Encode's job, one layer lower), it's just
// "what bytes does this particular application choose to store."
type op struct {
	Kind  string `json:"kind"` // "put" or "delete"
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

func main() {
	logPath := flag.String("log", "demo.wal", "path to the WAL file")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("usage: demo -log <path> <put KEY VALUE|get KEY|delete KEY|dump|stress -n N>")
	}

	l, err := wal.Open(*logPath)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer l.Close()

	store := map[string]string{}
	recovered := 0
	if replayErr := l.Replay(func(record []byte) error {
		var o op
		if err := json.Unmarshal(record, &o); err != nil {
			return err
		}
		applyOp(store, o)
		recovered++
		return nil
	}); replayErr != nil {
		// Expected right now: Replay is a stub returning "not
		// implemented." Log it and keep going with whatever's in
		// `store` (nothing, currently) so the rest of the CLI stays
		// usable to observe.
		log.Printf("replay: %v (store has %d recovered entries)", replayErr, recovered)
	} else {
		log.Printf("replay: recovered %d entries", recovered)
	}

	switch args[0] {
	case "put":
		if len(args) != 3 {
			log.Fatal("usage: demo put KEY VALUE")
		}
		appendAndApply(l, store, op{Kind: "put", Key: args[1], Value: args[2]})
		fmt.Println("OK")

	case "delete":
		if len(args) != 2 {
			log.Fatal("usage: demo delete KEY")
		}
		appendAndApply(l, store, op{Kind: "delete", Key: args[1]})
		fmt.Println("OK")

	case "get":
		if len(args) != 2 {
			log.Fatal("usage: demo get KEY")
		}
		v, ok := store[args[1]]
		if !ok {
			fmt.Println("NOT_FOUND")
			os.Exit(1)
		}
		fmt.Println(v)

	case "dump":
		w := bufio.NewWriter(os.Stdout)
		defer w.Flush()
		for k, v := range store {
			fmt.Fprintf(w, "%s=%s\n", k, v)
		}

	case "stress":
		fs := flag.NewFlagSet("stress", flag.ExitOnError)
		n := fs.Int("n", 1000, "number of put operations to append")
		fs.Parse(args[1:])
		for i := 0; i < *n; i++ {
			key := "stress-" + strconv.Itoa(i)
			appendAndApply(l, store, op{Kind: "put", Key: key, Value: key})
		}
		fmt.Printf("appended %d records\n", *n)

	default:
		log.Fatalf("unknown command %q", args[0])
	}
}

func appendAndApply(l *wal.Log, store map[string]string, o op) {
	record, err := json.Marshal(o)
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := l.Append(record); err != nil {
		// Expected right now: Append is a stub. Once implemented, a
		// non-nil error here means the operation is NOT durable and
		// must NOT be applied to the in-memory store either — never
		// apply what you didn't durably log is the whole point of a
		// WAL, so this early-return is intentionally the permanent
		// shape of this function, not just a placeholder.
		log.Printf("append: %v (not applying to in-memory store)", err)
		return
	}
	applyOp(store, o)
}

func applyOp(store map[string]string, o op) {
	switch o.Kind {
	case "put":
		store[o.Key] = o.Value
	case "delete":
		delete(store, o.Key)
	}
}

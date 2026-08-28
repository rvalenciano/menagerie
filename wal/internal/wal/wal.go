// Package wal is a crash-safe, append-only log: once Append returns
// successfully, the record is durably on disk and Replay will produce
// it even after an immediate power loss. Its two responsibilities
// (durability and safe recovery) are deliberately split into two
// seams — see README.md §3.
package wal

import (
	"errors"
	"os"
)

// Syncer is the durability boundary Log writes through. *os.File
// satisfies it. Tests can inject a fake Syncer to assert Sync was
// actually called, without touching a real disk.
type Syncer interface {
	Write(p []byte) (int, error)
	Sync() error
	Close() error
}

// Log is an append-only, crash-safe log of records, backed by a
// single file.
type Log struct {
	f Syncer
	// TODO(TDD): whatever offset/position bookkeeping Append and
	// Replay end up needing (e.g. the current write offset, if Append
	// is to report it back to the caller).
}

// Open opens (creating if necessary) the log file at path for
// appending. This is plumbing — implemented fully, unlike Append and
// Replay below.
func Open(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Log{f: f}, nil
}

// Close closes the underlying file.
func (l *Log) Close() error {
	return l.f.Close()
}

// Append durably persists record and must not return until it's
// actually on disk — a crash the instant after Append returns must
// never lose that record. That's the entire point of a WAL.
//
// TODO(TDD): implement test-first (seam 2 in README.md §3).
//
// Expected shape: encode record with frame.Encode, write the result
// to l.f, then call l.f.Sync() — treat the append as failed if either
// the write or the Sync returns an error. Sync (an fsync under the
// hood) is the expensive part and the reason this project has a real
// performance question: how many durable appends/sec can you sustain,
// and does batching several appends behind one Sync call ("group
// commit," a technique every real database uses) help? That's a good
// v2 once the naive one-Sync-per-Append version is green.
func (l *Log) Append(record []byte) error {
	return errors.New("not implemented")
}

// Replay reads every complete record in the log, in the order they
// were appended, calling fn with each one. If the log's tail is torn
// (the process crashed mid-Append, leaving a partial frame), Replay
// must stop cleanly at the last complete record and return nil — not
// an error. A torn tail is an expected, recoverable condition for a
// crash-safe log, not corruption to panic over.
//
// TODO(TDD): implement test-first (seam 3 in README.md §3). This is
// the seam worth the most attention: write a test that appends N
// valid records, then appends a deliberately truncated (N+1)th
// frame's bytes directly (bypassing Append), and asserts Replay
// yields exactly the N complete records without erroring.
//
// Expected shape: read the file's contents (or stream them) and
// repeatedly call frame.Decode on what's left, calling fn(record) for
// each successful decode and advancing past `consumed` bytes. On
// frame.ErrCorrupt, stop the loop and return nil — that's the
// torn-tail case. Propagate any other error (e.g. from fn) or a
// clean, empty EOF as you see fit.
func (l *Log) Replay(fn func(record []byte) error) error {
	return errors.New("not implemented")
}

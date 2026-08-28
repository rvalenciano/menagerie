// Package frame is the pure, no-I/O half of the WAL: how a single
// record gets serialized into a self-describing chunk of bytes that a
// reader can pull off a stream, and how a reader tells a complete,
// valid frame apart from a torn one left by a crash mid-write. It has
// zero file/disk involvement, which is what makes it testable with
// plain byte slices — no temp files, no fsync, no process to kill.
package frame

import "errors"

// ErrCorrupt means buf didn't contain one complete, checksum-valid
// frame. This covers two different real situations that look
// identical from a decoder's point of view: genuine bit-rot, and (far
// more commonly) a process that crashed partway through writing a
// frame, leaving a truncated tail. Callers replaying a log treat
// ErrCorrupt as "stop here, cleanly" rather than a fatal error — see
// wal.Log.Replay.
var ErrCorrupt = errors.New("frame: corrupt or incomplete")

// Encode serializes record into a self-describing frame: a header
// (at minimum, the record's length and a checksum of its bytes)
// followed by the raw record bytes.
//
// TODO(TDD): implement test-first (seam 1 in README.md §3).
//
// Expected shape: a fixed-size header — e.g. a 4-byte big-endian
// uint32 length, then a 4-byte big-endian uint32 CRC32 checksum of
// record — followed by record itself. encoding/binary and hash/crc32
// are the standard-library tools for this; you don't need a real
// serialization framework for something this small.
func Encode(record []byte) []byte {
	return nil
}

// Decode reads exactly one frame from the front of buf, returning the
// original record and how many bytes of buf it consumed. It returns
// ErrCorrupt (not a lower-level error) if buf doesn't hold one
// complete, checksum-valid frame — including the case where buf is
// simply too short because it ends mid-frame.
//
// TODO(TDD): implement test-first (seam 1 in README.md §3).
//
// Expected shape: check buf is at least long enough to hold the
// header (ErrCorrupt if not), read the length out of the header,
// check buf is at least header+length bytes long (ErrCorrupt if not —
// this is the torn-write case), recompute the checksum over those
// length bytes and compare it to the header's checksum (ErrCorrupt on
// mismatch), then return the record slice and the total bytes
// consumed (header + record).
func Decode(buf []byte) (record []byte, consumed int, err error) {
	return nil, 0, errors.New("not implemented")
}

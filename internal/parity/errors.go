package parity

import "errors"

// Sentinel errors. Wrap with %w; callers check via errors.Is.
var (
	// ErrEnvelopeNotFound is returned by RecordedTransport.Run when no
	// recorded envelope matches the supplied prompt. This means either
	// the corpus is incomplete or the workflow under test produced a
	// new prompt the prototype never emitted.
	ErrEnvelopeNotFound = errors.New("parity: no recorded envelope matches prompt")

	// ErrCorpusInvalid is returned by NewRecordedTransport when the
	// on-disk corpus has malformed envelope files or duplicate prompt
	// hashes (which indicate a capture-script bug, not a port issue).
	ErrCorpusInvalid = errors.New("parity: corpus invalid")

	// ErrFixtureMissing is returned by LoadFixture when the source
	// fixture directory does not exist or is not a directory.
	ErrFixtureMissing = errors.New("parity: fixture directory missing")
)

// Package parity provides primitives for parity-validation tests of
// LLM-touching workflows against captured prototype behavior.
//
// The three primitives:
//
//   - LoadFixture copies a fixture vault tree into a per-test temp dir
//     so tests get a hermetic, mutable vault. The fixture stays clean;
//     the test can write back without polluting checked-in data.
//
//   - RecordedTransport implements transport.Transport by replaying
//     previously-captured prototype LLM responses keyed by prompt
//     hash. The captured envelopes include the skill SHA at capture
//     time; tests compare it against the current skill file SHA to
//     surface skill drift (config) distinctly from port regressions
//     (code). Decision 4 in PARITY.md.
//
//   - Diff walks two vault trees and produces a structural difference
//     report under a configurable Equivalence relation. The default
//     relation matches PARITY.md's contract for LLM-touching
//     workflows: structural mutations (which files exist, which
//     frontmatter keys are set, which wikilinks are present) must
//     match exactly; LLM-authored prose body content is not compared.
//
// T-PROVE in v12 P1-W2 will consume these primitives on the first
// chosen workflow (summarize-transcripts per PARITY.md decision 3).
// This package ships the primitives only — capture scripting and
// per-workflow assertions are downstream.
package parity

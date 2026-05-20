// Package inferencerouter defines the inference eval router seam (see
// ARCHITECTURE.md §6).
//
// The seam is harness-agnostic: it is not designed around any specific
// external routing product, present or anticipated. AI command wrappers
// obtain an InferenceDecision from this package before invoking any LLM
// transport.
//
// Constraints on this package, enforced by CI grep gates:
//   - zero exec.Command calls
//   - zero outbound net/http
//   - zero provider SDK imports
//   - zero string literals for model/provider IDs
//
// At this scaffold wave (T02) the package exists only as a placeholder
// so the layout is committed before implementation lands. The interface,
// types, and LocalRouter stub land in T06 (P3-W1).
package inferencerouter

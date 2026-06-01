package parity

// Equivalence configures what counts as "the same" when Diff compares
// two vault trees. PARITY.md's contract for LLM-touching workflows is
// post-state / contract equivalence under a defined relation:
// structural mutations match exactly; LLM-authored prose does not need
// to.
//
// The defaults match that contract: file presence, frontmatter keys,
// wikilink set. Body prose is NOT compared by default. Workflows that
// need a tighter check (pure-data outputs that should be byte-for-byte
// identical) can opt in to CompareBody.
type Equivalence struct {
	// FilesPresent asserts the set of relative file paths under each
	// root match exactly. Default: true.
	FilesPresent bool

	// FrontmatterKeys asserts every markdown file with frontmatter has
	// the same set of keys with the same values on both sides.
	// Default: true.
	FrontmatterKeys bool

	// WikilinkSet asserts every markdown file emits the same set of
	// [[wikilinks]] on both sides. Order is normalized out. Default:
	// true.
	WikilinkSet bool

	// CompareBody asserts byte-exact body equality after frontmatter
	// is stripped. Use only for workflows whose output IS deterministic
	// (pure-data reports, not LLM-authored prose). Default: false.
	CompareBody bool

	// IgnorePaths is a set of relative paths to exclude from every
	// axis above. Useful for skipping `_meta/processing-logs/` and
	// other timestamped artifacts the comparison doesn't gate on.
	IgnorePaths []string
}

// DefaultEquivalence returns the relation matching PARITY.md's
// contract for LLM-touching workflows.
func DefaultEquivalence() Equivalence {
	return Equivalence{
		FilesPresent:    true,
		FrontmatterKeys: true,
		WikilinkSet:     true,
		CompareBody:     false,
	}
}

// StrictEquivalence is byte-for-byte body comparison too. Use only
// for pure-data workflows whose output is deterministic.
func StrictEquivalence() Equivalence {
	e := DefaultEquivalence()
	e.CompareBody = true
	return e
}

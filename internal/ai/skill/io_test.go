package skill

import "os"

// Tiny os indirections for tests; defined as vars so individual tests
// could swap them. At this stage they're direct passthroughs.
var (
	osGetwd = os.Getwd
	osStat  = os.Stat
)

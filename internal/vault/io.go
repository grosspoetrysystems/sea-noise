package vault

import (
	"io/fs"
	"os"
	"path/filepath"
)

// os* indirections so tests can swap in fakes if needed. At this scaffold
// stage they are direct passthroughs to the stdlib; no swapping yet.

var (
	osStat     = func(name string) (fs.FileInfo, error) { return os.Stat(name) }
	osReadFile = func(name string) ([]byte, error) { return os.ReadFile(name) }
	osWalkDir  = func(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }
)

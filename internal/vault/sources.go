package vault

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"sea-noise/internal/markdown"
)

var (
	// ErrSourceConflict means two explicit sources would produce the same
	// logical note ID from different files.
	ErrSourceConflict = errors.New("vault: source conflict")
	// ErrSourcePath is returned for a source mapping that is not an absolute,
	// existing file/directory or whose logical root escapes the vault.
	ErrSourcePath = errors.New("vault: invalid source path")
)

// Source maps one logical vault-relative root to one resolved physical target.
// PhysicalRoot may be a directory or a single markdown file; callers must
// register it explicitly before the walker follows that boundary.
type Source struct {
	LogicalRoot  string
	PhysicalRoot string
}

// WalkSourcesContext walks the vault root plus explicitly registered sources.
// Directory symlinks and symlink files are never followed during any walk.
// PhysicalPath on each returned Note is the actual file read, while ID and
// Path remain stable logical vault-relative identifiers.
func WalkSourcesContext(ctx context.Context, root string, excludeDirs []string, sources []Source) ([]Note, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rootAbs, _, err := absoluteExisting(root, true)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrSourcePath) {
			return nil, ErrVaultNotFound
		}
		return nil, err
	}

	all := make([]Source, 0, len(sources)+1)
	all = append(all, Source{LogicalRoot: "", PhysicalRoot: rootAbs})
	all = append(all, sources...)
	exclude := make(map[string]struct{}, len(excludeDirs))
	for _, d := range excludeDirs {
		exclude[d] = struct{}{}
	}

	sourceRoots := make(map[string]struct{}, len(sources))
	seenLogical := make(map[string]string)
	seenPhysical := make(map[string]struct{})
	var notes []Note
	for sourceIndex, source := range all {
		if sourceIndex > 0 {
			logicalRoot := filepath.ToSlash(strings.Trim(source.LogicalRoot, "/"))
			if logicalRoot == "" {
				return nil, fmt.Errorf("%w: explicit source requires a logical root", ErrSourcePath)
			}
			if _, ok := sourceRoots[logicalRoot]; ok {
				return nil, fmt.Errorf("%w: duplicate logical root %q", ErrSourceConflict, logicalRoot)
			}
			sourceRoots[logicalRoot] = struct{}{}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		physical, info, err := validateSource(source, rootAbs)
		if err != nil {
			return nil, err
		}
		err = osWalkDir(physical, func(filePath string, entry fs.DirEntry, walkErr error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(physical, filePath)
			if err != nil {
				return err
			}
			if source.LogicalRoot == "" && (rel == "vernacular" || strings.HasPrefix(rel, "vernacular"+string(filepath.Separator))) {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			// WalkDir does not descend through directory symlinks, but it
			// still reports symlink files. Skip both forms so a normal vault
			// walk cannot accidentally cross the bridge links.
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				if filePath != physical {
					if _, ok := exclude[entry.Name()]; ok {
						return fs.SkipDir
					}
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				return nil
			}
			logical := sourceLogicalPath(source.LogicalRoot, rel)
			if logical == "" || logical == "." {
				return nil
			}
			canonical, err := filepath.EvalSymlinks(filePath)
			if err != nil {
				return nil
			}
			if _, ok := seenPhysical[canonical]; ok {
				return nil
			}
			if previous, ok := seenLogical[logical]; ok {
				return fmt.Errorf("%w: logical ID %q maps to %q and %q", ErrSourceConflict, logical, previous, canonical)
			}
			content, err := ReadFileRetryContext(ctx, filePath)
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				return nil
			}
			if strings.TrimSpace(string(content)) == "" {
				return nil
			}
			fm := markdown.ParseFrontmatter(string(content))
			title := fm.Title
			if title == "" {
				title = markdown.TitleFromFilenameStem(strings.TrimSuffix(filepath.Base(logical), filepath.Ext(logical)))
			}
			seenPhysical[canonical] = struct{}{}
			seenLogical[logical] = canonical
			notes = append(notes, Note{
				ID:           logical,
				Path:         logical,
				PhysicalPath: filePath,
				Title:        title,
				Tags:         fm.Tags,
				Body:         fm.Body,
			})
			return nil
		})
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && info.IsDir() {
				return nil, ErrVaultNotFound
			}
			return nil, err
		}
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	return notes, nil
}

// ResolvePath maps a stable logical ID to a registered physical file. It
// rejects symlink components so direct reads obey the same explicit-source
// boundary as WalkSourcesContext.
func ResolvePath(root, id string, sources []Source) (string, bool) {
	rootAbs, _, err := absoluteExisting(root, true)
	if err != nil {
		return "", false
	}
	id = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(id))))
	if id == "." || id == "" || filepath.IsAbs(filepath.FromSlash(id)) || strings.HasPrefix(id, "../") || strings.Contains(id, "/../") {
		return "", false
	}
	all := make([]Source, 0, len(sources)+1)
	all = append(all, Source{LogicalRoot: "", PhysicalRoot: rootAbs})
	all = append(all, sources...)
	sort.SliceStable(all, func(i, j int) bool {
		return len(all[i].LogicalRoot) > len(all[j].LogicalRoot)
	})
	for _, source := range all {
		physical, info, err := validateSource(source, rootAbs)
		if err != nil {
			continue
		}
		logical := filepath.ToSlash(strings.Trim(source.LogicalRoot, "/"))
		var rel string
		switch {
		case logical == "" && id != "":
			rel = filepath.FromSlash(id)
		case id == logical:
			if info.IsDir() {
				return "", false
			}
			return physical, safePhysicalPath(physical, physical)
		case logical != "" && strings.HasPrefix(id, logical+"/") && info.IsDir():
			rel = filepath.FromSlash(strings.TrimPrefix(id, logical+"/"))
		default:
			continue
		}
		candidate := filepath.Join(physical, rel)
		if !pathWithin(physical, candidate) || !safePhysicalPath(physical, candidate) {
			return "", false
		}
		return candidate, true
	}
	return "", false
}

func validateSource(source Source, vaultRoot string) (string, fs.FileInfo, error) {
	rawLogical := filepath.FromSlash(source.LogicalRoot)
	if filepath.IsAbs(rawLogical) || strings.HasPrefix(source.LogicalRoot, "/") {
		return "", nil, fmt.Errorf("%w: logical root %q", ErrSourcePath, source.LogicalRoot)
	}
	logical := filepath.ToSlash(strings.Trim(source.LogicalRoot, "/"))
	if logical != "" {
		clean := path.Clean(logical)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") || strings.Contains(clean, "\\") {
			return "", nil, fmt.Errorf("%w: logical root %q", ErrSourcePath, source.LogicalRoot)
		}
		logical = clean
	}
	physical, info, err := absoluteExisting(source.PhysicalRoot, false)
	if err != nil {
		return "", nil, fmt.Errorf("%w: physical root %q: %v", ErrSourcePath, source.PhysicalRoot, err)
	}
	if source.LogicalRoot != "" && logical == "" {
		return "", nil, fmt.Errorf("%w: empty logical root", ErrSourcePath)
	}
	if source.LogicalRoot == "" {
		if physical != vaultRoot {
			return "", nil, fmt.Errorf("%w: empty logical root must map to vault root", ErrSourcePath)
		}
	} else if pathWithin(vaultRoot, physical) || pathWithin(physical, vaultRoot) {
		return "", nil, fmt.Errorf("%w: source target overlaps vault", ErrSourcePath)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%w: physical root is not a file or directory", ErrSourcePath)
	}
	return physical, info, nil
}

func sourceLogicalPath(prefix, rel string) string {
	rel = filepath.ToSlash(rel)
	if prefix == "" {
		return rel
	}
	if rel == "." || rel == "" {
		return filepath.ToSlash(prefix)
	}
	return filepath.ToSlash(filepath.Join(prefix, filepath.FromSlash(rel)))
}

func pathWithin(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
func safePhysicalPath(base, candidate string) bool {
	base, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil || !pathWithin(base, candidate) {
		return false
	}
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	current := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func absoluteExisting(name string, wantDir bool) (string, fs.FileInfo, error) {
	if name == "" {
		return "", nil, ErrSourcePath
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", nil, err
	}
	info, err := osStat(abs)
	if err != nil {
		return "", nil, err
	}
	if wantDir && !info.IsDir() {
		return "", nil, ErrSourcePath
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, err
	}
	if resolvedInfo, err := osStat(resolved); err == nil {
		info = resolvedInfo
	} else {
		return "", nil, err
	}
	if wantDir && !info.IsDir() {
		return "", nil, ErrSourcePath
	}
	return resolved, info, nil
}

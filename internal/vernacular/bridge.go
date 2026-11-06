// Package vernacular owns Sea Noise's opt-in project wing under a vault root.
//
// The bridge stores only its own membership record and generated symlinks. The
// files reached through those links remain the original Vernacular files.
package vernacular

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sea-noise/internal/vault"
)

const (
	wingDirName      = "vernacular"
	membershipName   = ".sea-noise-membership.json"
	membershipOwner  = "sea-noise-vernacular"
	membershipSchema = 1
	resolverTimeout  = 30 * time.Second
)

var (
	ErrInvalidKey          = errors.New("vernacular: invalid project key")
	ErrInvalidPath         = errors.New("vernacular: invalid path")
	ErrMembership          = errors.New("vernacular: invalid membership")
	ErrProjectNotFound     = errors.New("vernacular: project not found")
	ErrDestinationConflict = errors.New("vernacular: destination conflict")
	ErrOwnership           = errors.New("vernacular: generated link ownership mismatch")
	ErrStaleProject        = errors.New("vernacular: registered project is stale")
	ErrTargetConflict      = errors.New("vernacular: project targets conflict")
)

// Project is a selected project in the vault wing. Directory fields are the
// resolved physical Vernacular targets; Overview is the stable logical note
// ID Sea Noise uses for retrieval and discovery.
type Project struct {
	Key       string
	Checkout  string
	Canon     string
	Apparatus string
	Working   string
	Overview  string
}

// List returns bridge-owned projects in stable key order. It does not hide a
// stale registration; Sources reports stale targets before retrieval uses it.
func List(vaultRoot string) ([]Project, error) {
	root, err := cleanVaultRoot(vaultRoot)
	if err != nil {
		return nil, err
	}
	state, exists, err := readMembership(root)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []Project{}, nil
	}
	projects := make([]Project, 0, len(state.Projects))
	for _, p := range state.Projects {
		projects = append(projects, p.project())
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Key < projects[j].Key })
	return projects, nil
}

// Sources returns explicit source mappings for all registered projects. The
// vault root itself is implicit to callers; these mappings are the only paths
// allowed to cross the external project boundary. Overview is ordered before
// directory sources so a file that also lives under apparatus/working gets
// its stable entry ID rather than a duplicate physical identity.
func Sources(ctx context.Context, vaultRoot string) ([]vault.Source, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := cleanVaultRoot(vaultRoot)
	if err != nil {
		return nil, err
	}
	state, exists, err := readMembership(root)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []vault.Source{}, nil
	}
	if err := validateMembership(root, state); err != nil {
		return nil, err
	}
	projects := append([]membershipProject(nil), state.Projects...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].Key < projects[j].Key })

	out := make([]vault.Source, 0, len(projects)*3)
	for _, p := range projects {
		if err := resolveRegisteredTargets(ctx, p); err != nil {
			return nil, err
		}
		if err := verifyProjectTargets(root, p); err != nil {
			return nil, err
		}
		if err := verifyOwnedLinks(root, p); err != nil {
			return nil, err
		}
		out = append(out,
			vault.Source{LogicalRoot: p.OverviewID, PhysicalRoot: p.OverviewTarget},
			vault.Source{LogicalRoot: filepath.ToSlash(filepath.Join(wingDirName, p.Key, "apparatus")), PhysicalRoot: p.Apparatus},
			vault.Source{LogicalRoot: filepath.ToSlash(filepath.Join(wingDirName, p.Key, "working")), PhysicalRoot: p.Working},
		)
	}
	return out, nil
}

type membership struct {
	Owner    string              `json:"owner"`
	Version  int                 `json:"version"`
	Projects []membershipProject `json:"projects"`
}

type membershipProject struct {
	Key             string `json:"key"`
	Checkout        string `json:"checkout"`
	Canon           string `json:"canon"`
	Apparatus       string `json:"apparatus"`
	Working         string `json:"working"`
	OverviewID      string `json:"overview_id"`
	OverviewTarget  string `json:"overview_target"`
	ApparatusLink   string `json:"apparatus_link"`
	WorkingLink     string `json:"working_link"`
	OverviewLink    string `json:"overview_link"`
	EntryReadmeHash string `json:"entry_readme_hash,omitempty"`
}

func (p membershipProject) project() Project {
	return Project{
		Key:       p.Key,
		Checkout:  p.Checkout,
		Canon:     p.Canon,
		Apparatus: p.Apparatus,
		Working:   p.Working,
		Overview:  p.OverviewID,
	}
}

// Attach attaches one selected checkout to the vault wing. It is idempotent
// only when the existing bridge-owned selection and generated links are
// identical.
func Attach(ctx context.Context, vaultRoot, key, checkout string) (Project, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateKey(key); err != nil {
		return Project{}, err
	}
	root, err := cleanVaultRoot(vaultRoot)
	if err != nil {
		return Project{}, err
	}
	checkout, err = cleanExistingDir(checkout, true)
	if err != nil {
		return Project{}, fmt.Errorf("%w: checkout: %v", ErrInvalidPath, err)
	}
	if pathInside(root, checkout) || pathInside(checkout, root) {
		return Project{}, fmt.Errorf("%w: checkout overlaps vault", ErrTargetConflict)
	}

	canon, err := vernPath(ctx, checkout, "canon")
	if err != nil {
		return Project{}, err
	}
	apparatus, err := vernPath(ctx, checkout, "apparatus")
	if err != nil {
		return Project{}, err
	}
	working, err := vernPath(ctx, checkout, "working")
	if err != nil {
		return Project{}, err
	}
	if err := validateTargets(root, canon, apparatus, working); err != nil {
		return Project{}, err
	}

	state, _, err := readMembership(root)
	if err != nil {
		return Project{}, err
	}
	if err := validateMembership(root, state); err != nil {
		return Project{}, err
	}
	for _, existing := range state.Projects {
		if existing.Key != key {
			continue
		}
		if existing.Checkout != checkout || existing.Canon != canon || existing.Apparatus != apparatus || existing.Working != working {
			return Project{}, fmt.Errorf("%w: key %q is already registered", ErrDestinationConflict, key)
		}
		if err := verifyOwnedLinks(root, existing); err != nil {
			return Project{}, err
		}
		return existing.project(), nil
	}
	for _, existing := range state.Projects {
		if existing.Checkout == checkout {
			return Project{}, fmt.Errorf("%w: checkout already registered as %q", ErrTargetConflict, existing.Key)
		}
		if pathsOverlap(existing.Canon, canon) || pathsOverlap(existing.Apparatus, apparatus) || pathsOverlap(existing.Working, working) ||
			pathsOverlap(existing.Canon, apparatus) || pathsOverlap(existing.Canon, working) || pathsOverlap(existing.Apparatus, working) {
			return Project{}, fmt.Errorf("%w: key %q overlaps %q", ErrTargetConflict, key, existing.Key)
		}
	}

	wing, err := ensureWing(root)
	if err != nil {
		return Project{}, err
	}
	entry := filepath.Join(wing, key)
	if _, err := os.Lstat(entry); err == nil {
		return Project{}, fmt.Errorf("%w: project entry %s exists", ErrDestinationConflict, entry)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Project{}, fmt.Errorf("%w: inspect project entry: %v", ErrDestinationConflict, err)
	}

	overviewTarget, createdOverview, err := chooseOverview(key, apparatus, working)
	if err != nil {
		return Project{}, err
	}
	if err := verifyTarget(root, overviewTarget, false); err != nil {
		if createdOverview {
			_ = os.Remove(overviewTarget)
		}
		return Project{}, fmt.Errorf("%w: overview target: %v", ErrTargetConflict, err)
	}
	newTargets := []string{canon, apparatus, working, overviewTarget}
	for _, existing := range state.Projects {
		existingTargets := []string{existing.Canon, existing.Apparatus, existing.Working, existing.OverviewTarget}
		for _, target := range newTargets {
			for _, existingTarget := range existingTargets {
				if pathsOverlap(target, existingTarget) {
					if createdOverview {
						_ = os.Remove(overviewTarget)
					}
					return Project{}, fmt.Errorf("%w: key %q overlaps %q", ErrTargetConflict, key, existing.Key)
				}
			}
		}
	}
	if err := ensureWingReadme(wing); err != nil {
		if createdOverview {
			_ = os.Remove(overviewTarget)
		}
		return Project{}, err
	}
	overviewID := filepath.ToSlash(filepath.Join(wingDirName, key, "overview.md"))
	readmeContent := projectReadmeContent(key, checkout, canon, apparatus, working, overviewID)
	owned := membershipProject{
		Key:             key,
		Checkout:        checkout,
		Canon:           canon,
		Apparatus:       apparatus,
		Working:         working,
		OverviewID:      overviewID,
		OverviewTarget:  overviewTarget,
		ApparatusLink:   apparatus,
		WorkingLink:     working,
		OverviewLink:    overviewTarget,
		EntryReadmeHash: contentHash(readmeContent),
	}
	if err := os.Mkdir(entry, 0o755); err != nil {
		if createdOverview {
			_ = os.Remove(overviewTarget)
		}
		return Project{}, fmt.Errorf("%w: create project entry: %v", ErrDestinationConflict, err)
	}
	created := make([]string, 0, 4)
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
		_ = os.Remove(entry)
		if createdOverview {
			_ = os.Remove(overviewTarget)
		}
	}
	readmePath := filepath.Join(entry, "README.md")
	if err := os.WriteFile(readmePath, readmeContent, 0o644); err != nil {
		rollback()
		return Project{}, fmt.Errorf("%w: create project README: %v", ErrDestinationConflict, err)
	}
	created = append(created, readmePath)
	for _, spec := range []struct{ name, target string }{
		{name: "apparatus", target: apparatus},
		{name: "working", target: working},
		{name: "overview.md", target: overviewTarget},
	} {
		path := filepath.Join(entry, spec.name)
		if err := os.Symlink(spec.target, path); err != nil {
			rollback()
			return Project{}, fmt.Errorf("%w: create %s: %v", ErrDestinationConflict, path, err)
		}
		created = append(created, path)
	}
	state.Projects = append(state.Projects, owned)
	if err := writeMembership(root, state); err != nil {
		rollback()
		return Project{}, err
	}
	return owned.project(), nil
}

// Detach removes only links and metadata owned by the bridge. Original target
// files and user-authored files in the project entry are preserved.
func Detach(vaultRoot, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	root, err := cleanVaultRoot(vaultRoot)
	if err != nil {
		return err
	}
	state, exists, err := readMembership(root)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: %q", ErrProjectNotFound, key)
	}
	if err := validateMembership(root, state); err != nil {
		return err
	}
	idx := -1
	for i, p := range state.Projects {
		if p.Key == key {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("%w: %q", ErrProjectNotFound, key)
	}
	p := state.Projects[idx]
	if err := verifyOwnedLinks(root, p); err != nil {
		return err
	}
	entry := filepath.Join(root, wingDirName, key)
	if err := removeGeneratedReadme(entry, p); err != nil {
		return fmt.Errorf("%w: remove generated README: %v", ErrOwnership, err)
	}
	for _, path := range []string{p.ApparatusLinkPath(root), p.WorkingLinkPath(root), p.OverviewLinkPath(root)} {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("%w: remove owned link %s: %v", ErrOwnership, path, err)
		}
	}
	if entries, readErr := os.ReadDir(entry); readErr == nil && len(entries) == 0 {
		_ = os.Remove(entry)
	}
	state.Projects = append(state.Projects[:idx], state.Projects[idx+1:]...)
	if len(state.Projects) == 0 {
		if err := removeMembership(root); err != nil {
			return err
		}
		return nil
	}
	return writeMembership(root, state)
}

func (p membershipProject) ApparatusLinkPath(root string) string {
	return filepath.Join(root, wingDirName, p.Key, "apparatus")
}
func (p membershipProject) WorkingLinkPath(root string) string {
	return filepath.Join(root, wingDirName, p.Key, "working")
}
func (p membershipProject) OverviewLinkPath(root string) string {
	return filepath.Join(root, wingDirName, p.Key, "overview.md")
}

func chooseOverview(key, apparatus, working string) (string, bool, error) {
	for _, root := range []string{apparatus, working} {
		if _, err := verifyCanonicalPath(root, true); err != nil {
			return "", false, fmt.Errorf("%w: inspect overview root %s: %v", ErrInvalidPath, root, err)
		}
		for _, name := range []string{"overview.md", "project-overview.md", "context.md", "README.md"} {
			path := filepath.Join(root, name)
			if _, err := verifyCanonicalPath(path, false); err == nil {
				return filepath.Clean(path), false, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return "", false, fmt.Errorf("%w: inspect overview %s: %v", ErrDestinationConflict, path, err)
			}
		}
	}
	path := filepath.Join(apparatus, "overview.md")
	if _, err := verifyCanonicalPath(path, false); err == nil {
		return "", false, fmt.Errorf("%w: overview destination %s exists", ErrDestinationConflict, path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", false, fmt.Errorf("%w: inspect overview destination: %v", ErrDestinationConflict, err)
	}
	content := []byte("---\ntitle: " + key + "\n---\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", false, fmt.Errorf("%w: create private overview: %v", ErrDestinationConflict, err)
	}
	if _, err := verifyCanonicalPath(path, false); err != nil {
		_ = os.Remove(path)
		return "", false, fmt.Errorf("%w: verify created overview: %v", ErrInvalidPath, err)
	}
	return filepath.Clean(path), true, nil
}

func verifyCanonicalPath(path string, wantDir bool) (fs.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, ErrInvalidPath
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidPath
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(resolved) != path {
		return nil, ErrInvalidPath
	}
	if wantDir && !info.IsDir() {
		return nil, fs.ErrInvalid
	}
	if !wantDir && !info.Mode().IsRegular() {
		return nil, fs.ErrInvalid
	}
	return info, nil
}

func verifyOwnedLinks(root string, p membershipProject) error {
	for _, dir := range []string{root, filepath.Join(root, wingDirName), filepath.Join(root, wingDirName, p.Key)} {
		if _, err := verifyCanonicalPath(dir, true); err != nil {
			return fmt.Errorf("%w: project entry ancestor %s: %v", ErrOwnership, dir, err)
		}
	}
	for _, spec := range []struct{ path, target string }{
		{p.ApparatusLinkPath(root), p.ApparatusLink},
		{p.WorkingLinkPath(root), p.WorkingLink},
		{p.OverviewLinkPath(root), p.OverviewLink},
	} {
		info, err := os.Lstat(spec.path)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrOwnership, spec.path, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%w: %s is not a symlink", ErrOwnership, spec.path)
		}
		actual, err := os.Readlink(spec.path)
		if err != nil {
			return fmt.Errorf("%w: read %s: %v", ErrOwnership, spec.path, err)
		}
		if actual != spec.target {
			return fmt.Errorf("%w: %s target changed", ErrOwnership, spec.path)
		}
	}
	return nil
}

func resolveRegisteredTargets(ctx context.Context, p membershipProject) error {
	checkout, err := cleanExistingDir(p.Checkout, true)
	if err != nil || checkout != p.Checkout {
		return fmt.Errorf("%w: checkout for %q changed; relink the project", ErrStaleProject, p.Key)
	}
	for _, item := range []struct {
		name, expected string
	}{
		{"canon", p.Canon},
		{"apparatus", p.Apparatus},
		{"working", p.Working},
	} {
		actual, err := vernPath(ctx, p.Checkout, item.name)
		if err != nil {
			return fmt.Errorf("%w: resolve %s for %q: %v; relink the project", ErrStaleProject, item.name, p.Key, err)
		}
		if actual != item.expected {
			return fmt.Errorf("%w: %s for %q changed from %s to %s; relink the project", ErrStaleProject, item.name, p.Key, item.expected, actual)
		}
	}
	return nil
}

func verifyProjectTargets(root string, p membershipProject) error {
	if err := validateKey(p.Key); err != nil {
		return fmt.Errorf("%w: membership key %q", ErrMembership, p.Key)
	}
	if p.OverviewID != filepath.ToSlash(filepath.Join(wingDirName, p.Key, "overview.md")) {
		return fmt.Errorf("%w: overview ID for %q", ErrMembership, p.Key)
	}
	if p.ApparatusLink != p.Apparatus || p.WorkingLink != p.Working || p.OverviewLink != p.OverviewTarget {
		return fmt.Errorf("%w: link target metadata for %q", ErrMembership, p.Key)
	}
	overviewAllowed := false
	for _, dir := range []string{p.Apparatus, p.Working} {
		for _, name := range []string{"overview.md", "project-overview.md", "context.md", "README.md"} {
			if p.OverviewTarget == filepath.Join(dir, name) {
				overviewAllowed = true
			}
		}
	}
	if !overviewAllowed {
		return fmt.Errorf("%w: overview target for %q is outside private apparatus/working", ErrMembership, p.Key)
	}
	for _, dir := range []string{p.Checkout, p.Canon, p.Apparatus, p.Working} {
		if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
			return fmt.Errorf("%w: non-canonical target in %q", ErrMembership, p.Key)
		}
	}
	for _, item := range []struct {
		name, path string
		dir        bool
	}{
		{"checkout", p.Checkout, true},
		{"canon", p.Canon, true},
		{"apparatus", p.Apparatus, true},
		{"working", p.Working, true},
		{"overview", p.OverviewTarget, false},
	} {
		if err := verifyTarget(root, item.path, item.dir); err != nil {
			return fmt.Errorf("%w: %s for %q: %v", ErrStaleProject, item.name, p.Key, err)
		}
	}
	return nil
}

func validateMembership(root string, state membership) error {
	if state.Owner != membershipOwner || state.Version != membershipSchema {
		return ErrMembership
	}
	seen := map[string]struct{}{}
	for _, p := range state.Projects {
		if _, ok := seen[p.Key]; ok {
			return fmt.Errorf("%w: duplicate key %q", ErrMembership, p.Key)
		}
		seen[p.Key] = struct{}{}
		if err := verifyProjectTargets(root, p); err != nil {
			// Listing is allowed to expose stale registrations; structural
			// validation must still reject malformed identity and paths.
			if errors.Is(err, ErrStaleProject) {
				continue
			}
			return err
		}
	}
	for i := range state.Projects {
		for j := i + 1; j < len(state.Projects); j++ {
			a, b := state.Projects[i], state.Projects[j]
			if a.Checkout == b.Checkout {
				return fmt.Errorf("%w: checkout %q is registered as %q and %q", ErrTargetConflict, a.Checkout, a.Key, b.Key)
			}
			targetsA := []string{a.Canon, a.Apparatus, a.Working, a.OverviewTarget}
			targetsB := []string{b.Canon, b.Apparatus, b.Working, b.OverviewTarget}
			for _, targetA := range targetsA {
				for _, targetB := range targetsB {
					if pathsOverlap(targetA, targetB) {
						return fmt.Errorf("%w: %q and %q", ErrTargetConflict, a.Key, b.Key)
					}
				}
			}
		}
	}
	return nil
}

func validateTargets(root, canon, apparatus, working string) error {
	for name, path := range map[string]string{"canon": canon, "apparatus": apparatus, "working": working} {
		if err := verifyTarget(root, path, true); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrTargetConflict, name, err)
		}
	}
	paths := []string{canon, apparatus, working}
	for i := range paths {
		for j := i + 1; j < len(paths); j++ {
			if pathsOverlap(paths[i], paths[j]) {
				return fmt.Errorf("%w: %s and %s", ErrTargetConflict, paths[i], paths[j])
			}
		}
	}
	return nil
}

func verifyTarget(vaultRoot, target string, wantDir bool) error {
	if !filepath.IsAbs(target) || pathInside(vaultRoot, target) || pathInside(target, vaultRoot) {
		return ErrInvalidPath
	}
	if _, err := verifyCanonicalPath(target, wantDir); err != nil {
		return err
	}
	return nil
}

func vernPath(ctx context.Context, checkout, space string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, resolverTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "vern", "path", space)
	cmd.Dir = checkout
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("vernacular: vern path %s: %w", space, err)
	}
	text := string(out)
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
		if strings.HasSuffix(text, "\r") {
			text = strings.TrimSuffix(text, "\r")
		}
	}
	if text == "" || strings.TrimSpace(text) != text || strings.ContainsAny(text, "\r\n") || !filepath.IsAbs(text) {
		return "", fmt.Errorf("%w: vern path %s must print one absolute directory", ErrInvalidPath, space)
	}
	resolved, err := cleanExistingDir(text, false)
	if err != nil {
		return "", fmt.Errorf("%w: vern path %s: %v", ErrInvalidPath, space, err)
	}
	return resolved, nil
}

func cleanExistingDir(path string, requireAbs bool) (string, error) {
	if path == "" || (requireAbs && !filepath.IsAbs(path)) {
		return "", ErrInvalidPath
	}
	if !filepath.IsAbs(path) {
		path, _ = filepath.Abs(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fs.ErrInvalid
	}
	return filepath.EvalSymlinks(path)
}

func cleanVaultRoot(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", ErrInvalidPath
	}
	return filepath.EvalSymlinks(abs)
}

func ensureWing(root string) (string, error) {
	path := filepath.Join(root, wingDirName)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(path, 0o755); err != nil {
			return "", fmt.Errorf("%w: create wing: %v", ErrDestinationConflict, err)
		}
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("%w: inspect wing: %v", ErrDestinationConflict, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%w: wing is not a directory", ErrDestinationConflict)
	}
	return path, nil
}

const wingReadme = `# Vernacular

This is the persistent project wing of the vault, not an inbox. It exposes
live links into selected project checkouts: apparatus is private project
material and working is tentative project material. Selecting a project for
retrieval does not enroll it in automatic write or inbox workflows.

Use sea-noise vernacular list to inspect selected projects. Open a project's
entry README for checkout navigation, and edit its original overview rather
than creating a duplicate summary here.
`

func ensureWingReadme(wing string) error {
	path := filepath.Join(wing, "README.md")
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: inspect wing README: %v", ErrDestinationConflict, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("%w: create wing README: %v", ErrDestinationConflict, err)
	}
	if _, err := file.WriteString(wingReadme); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("%w: write wing README: %v", ErrDestinationConflict, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("%w: close wing README: %v", ErrDestinationConflict, err)
	}
	return nil
}

func projectReadmeContent(key, checkout, canon, apparatus, working, overviewID string) []byte {
	return []byte(fmt.Sprintf("# %s\n\nThis entry is navigation only; edit the original overview.\n\n- Checkout: `%s`\n- Canon: `%s`\n- Apparatus: `%s`\n- Working: `%s`\n- Overview: [[%s]]\n", key, checkout, canon, apparatus, working, overviewID))
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum[:])
}

func removeGeneratedReadme(entry string, p membershipProject) error {
	expectedHash := contentHash(projectReadmeContent(p.Key, p.Checkout, p.Canon, p.Apparatus, p.Working, p.OverviewID))
	if p.EntryReadmeHash != expectedHash {
		return nil
	}
	path := filepath.Join(entry, "README.md")
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil || contentHash(content) != expectedHash {
		return nil
	}
	return os.Remove(path)
}

func readMembership(root string) (membership, bool, error) {
	wing := filepath.Join(root, wingDirName)
	wingInfo, err := os.Lstat(wing)
	if errors.Is(err, fs.ErrNotExist) {
		return membership{Owner: membershipOwner, Version: membershipSchema}, false, nil
	}
	if err != nil {
		return membership{}, false, fmt.Errorf("%w: inspect wing: %v", ErrMembership, err)
	}
	if wingInfo.Mode()&os.ModeSymlink != 0 || !wingInfo.IsDir() {
		return membership{}, false, fmt.Errorf("%w: wing is not a directory", ErrMembership)
	}
	path := filepath.Join(wing, membershipName)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return membership{Owner: membershipOwner, Version: membershipSchema}, false, nil
	}
	if err != nil {
		return membership{}, false, fmt.Errorf("%w: inspect membership: %v", ErrMembership, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return membership{}, false, fmt.Errorf("%w: membership file is not regular", ErrMembership)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return membership{}, false, fmt.Errorf("%w: read membership: %v", ErrMembership, err)
	}
	var state membership
	if err := json.Unmarshal(b, &state); err != nil {
		return membership{}, false, fmt.Errorf("%w: decode membership: %v", ErrMembership, err)
	}
	return state, true, nil
}

func writeMembership(root string, state membership) error {
	state.Owner = membershipOwner
	state.Version = membershipSchema
	sort.Slice(state.Projects, func(i, j int) bool { return state.Projects[i].Key < state.Projects[j].Key })
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: encode membership: %v", ErrMembership, err)
	}
	data = append(data, '\n')
	wing, err := ensureWing(root)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(wing, membershipName+".tmp-")
	if err != nil {
		return fmt.Errorf("%w: create membership temp: %v", ErrMembership, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: chmod membership temp: %v", ErrMembership, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: write membership: %v", ErrMembership, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: close membership: %v", ErrMembership, err)
	}
	if err := os.Rename(tmpName, filepath.Join(wing, membershipName)); err != nil {
		return fmt.Errorf("%w: replace membership: %v", ErrMembership, err)
	}
	return nil
}

func removeMembership(root string) error {
	path := filepath.Join(root, wingDirName, membershipName)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%w: inspect membership: %v", ErrOwnership, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: membership file replaced", ErrOwnership)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%w: remove membership: %v", ErrMembership, err)
	}
	return nil
}

func validateMembershipKeyOnly(key string) bool {
	if key == "" || key == "." || key == ".." || filepath.Base(key) != key || strings.ContainsAny(key, `/\\`) {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validateKey(key string) error {
	if !validateMembershipKeyOnly(key) {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return nil
}

func pathInside(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func pathsOverlap(a, b string) bool {
	return pathInside(a, b) || pathInside(b, a)
}

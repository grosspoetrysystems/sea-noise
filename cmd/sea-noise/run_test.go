package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunAcceptsFlagsAfterPipeline(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test source")
	}

	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	if err := os.MkdirAll(filepath.Join(vault, "inbox", "_meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	inboxNote := filepath.Join(vault, "inbox", "stub.md")
	before := []byte("---\nsource: https://example.com\n---\nshort stub\n")
	if err := os.WriteFile(inboxNote, before, 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "sea-noise")

	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Dir(sourceFile)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sea-noise: %v\n%s", err, output)
	}
	env := []string{"HOME=" + root, "PATH=" + os.Getenv("PATH"), "MIND_PALACE_VAULT=" + vault}
	invalid := exec.Command(binary, "run", "inbox", "--force", "--dry-run")
	invalid.Env = env
	if output, err := invalid.CombinedOutput(); err == nil {
		t.Fatalf("inbox --force unexpectedly succeeded:\n%s", output)
	} else if !strings.Contains(string(output), "--force is only valid for daily or weekly pipelines") {
		t.Fatalf("inbox --force error = %s", output)
	}

	run := exec.Command(binary, "run", "inbox", "--dry-run")
	run.Env = env
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("dry-run after pipeline failed: %v\n%s", err, output)
	}

	entries, err := os.ReadDir(filepath.Join(vault, "_meta", "processing-logs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("processing log count = %d, want 1", len(entries))
	}
	log, err := os.ReadFile(filepath.Join(vault, "_meta", "processing-logs", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "[DRY-RUN] process-inbox") {
		t.Fatalf("processing log does not record a dry run:\n%s", log)
	}

	after, err := os.ReadFile(inboxNote)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("dry-run changed inbox note:\n%s", after)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta", "crawl-failures.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created crawl-failures.json: %v", err)
	}
}

func TestSplitRunArgsPreservesFlagsAfterPipeline(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	force := fs.Bool("force", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	pipeline, flagArgs, err := splitRunArgs(fs, []string{"inbox", "--force", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.Parse(flagArgs); err != nil {
		t.Fatal(err)
	}
	if pipeline != "inbox" || !*force || !*dryRun {
		t.Fatalf("pipeline=%q force=%v dry-run=%v", pipeline, *force, *dryRun)
	}
}

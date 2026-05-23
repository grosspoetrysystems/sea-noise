package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/enrichtopics"
)

// runEnrichTopics implements `sea-noise enrich-topics <topic>`.
//
// Manual topic mode only. Auto mode (driven by _meta/topic-frequencies.md)
// and per-note enrichment are deferred follow-ups per the prototype's
// shape — see internal/enrichtopics/enrichtopics.go for the boundary.
//
// Crawler: Cloudflare /browser-rendering/crawl is used when both
// CF_ACCOUNT_ID and CF_API_TOKEN are set. Otherwise enrichment proceeds
// vault-only (no web context), matching the prototype's silent fallback.
func runEnrichTopics(args []string) error {
	fs := flag.NewFlagSet("enrich-topics", flag.ContinueOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files (defaults to .agents/shared/skills/inference relative to binary or CWD)")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} is substituted")
	timeoutSec := fs.Int("timeout", 600, "per-inference-call timeout in seconds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage: sea-noise enrich-topics [flags] <topic>

Manual topic mode: scan the vault for notes whose tags or filename slug
contain <topic>, optionally crawl their source URLs via Cloudflare
/browser-rendering/crawl, and write a research brief to
notes/references/briefs/.

Flags:
  --cli-command CMD     inference CLI (required, e.g. "claude", "codex")
  --cli-args     ARGS   argv template; {PROMPT} substituted at call time
  --skills-dir   DIR    inference skill directory (auto-located by default)
  --timeout      SECS   per-call timeout (default 600)

Crawler config (optional):
  CF_ACCOUNT_ID   Cloudflare account ID
  CF_API_TOKEN    Cloudflare API token
Both must be set for crawling; otherwise enrichment is vault-only.`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return errors.New("topic argument required")
	}
	topic := fs.Arg(0)

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	skillsPath := *skillsDir
	if skillsPath == "" {
		skillsPath = guessSkillsDir()
	}
	skills, err := skill.LoadFromDir(skillsPath)
	if err != nil {
		return fmt.Errorf("load skills from %s: %w", skillsPath, err)
	}
	if _, ok := skills["generate-research-brief"]; !ok {
		return errors.New("generate-research-brief skill not found; check --skills-dir")
	}
	if *cliCommand == "" {
		return errors.New("--cli-command required (e.g., claude, codex)")
	}

	tx := &transport.CLITransport{
		Command: *cliCommand,
		Args:    splitCSV(*cliArgsList),
		Timeout: time.Duration(*timeoutSec) * time.Second,
		EnvOverrides: map[string]string{
			"CLAUDECODE": "",
		},
	}
	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), tx)
	crawler := enrichtopics.NewCrawlerFromEnv()

	res, err := enrichtopics.EnrichTopic(context.Background(), exec, crawler, enrichtopics.Input{
		VaultRoot: vaultRoot,
		Topic:     topic,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Enriched topic %q: %d notes, %d crawled records\n", topic, res.Notes, res.Crawled)
	fmt.Printf("  brief: %s\n", res.BriefPath)
	fmt.Printf("  log:   %s\n", res.LogPath)
	return nil
}

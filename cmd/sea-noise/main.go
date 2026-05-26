// Command sea-noise is the CLI entry point for the Sea Noise runtime.
//
// At this scaffold stage it parses --help/--version and prints usage.
// Subcommands are added in later waves: deterministic pipeline commands
// (T05), AI command wrappers (T07), and so on.
package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.0.0-scaffold"

func main() {
	flag.Usage = printUsage
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if flag.NArg() == 0 {
		printUsage()
		return
	}

	sub := flag.Arg(0)
	rest := flag.Args()[1:]

	switch sub {
	case "audit-links":
		if err := runAuditLinks(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise audit-links: %v\n", err)
			os.Exit(1)
		}
	case "summarize-transcripts":
		if err := runSummarizeTranscripts(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise summarize-transcripts: %v\n", err)
			os.Exit(1)
		}
	case "profile-entities":
		if err := runProfileEntities(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise profile-entities: %v\n", err)
			os.Exit(1)
		}
	case "index":
		if err := runIndex(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise index: %v\n", err)
			os.Exit(1)
		}
	case "search":
		if err := runSearch(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise search: %v\n", err)
			os.Exit(1)
		}
	case "generate-signal":
		if err := runGenerateSignal(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise generate-signal: %v\n", err)
			os.Exit(1)
		}
	case "enrich-topics":
		if err := runEnrichTopics(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise enrich-topics: %v\n", err)
			os.Exit(1)
		}
	case "youtube":
		if err := runYoutube(rest); err != nil {
			fmt.Fprintf(os.Stderr, "sea-noise youtube: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "sea-noise: unknown subcommand %q\n\n", sub)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `sea-noise — local retrieval and context-assembly runtime over a markdown vault

Usage:
  sea-noise [flags] <subcommand> [args...]

Flags:
  --help       show this help
  --version    print version and exit

Subcommands:
  audit-links              scan vault for broken [[wikilinks]], write report to _meta/broken-links.md
  summarize-transcripts    enrich notes containing ## Transcript with a ## Research Context section via the configured inference CLI
  profile-entities         generate an entity profile from vault mentions; manual mode only (auto-mode is a follow-up port)
  index                    rebuild the SQLite retrieval index for the configured vault (--db PATH)
  search                   run FTS5 keyword search against the index (--db PATH QUERY...)
  generate-signal          assemble the rolling signal queue from unconsumed notes, write to signal/queue.md
  enrich-topics            write a research brief for a topic cluster; uses Cloudflare /crawl when CF_ACCOUNT_ID + CF_API_TOKEN are set, vault-only otherwise
  youtube                  hydrate notes whose source: frontmatter points at YouTube with a ## Transcript section

Config:
  Vault path is resolved from MIND_PALACE_VAULT env or vault.path in
  sea-noise.toml. See sea-noise.toml.example for the format.`)
}

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
  audit-links    scan vault for broken [[wikilinks]], write report to _meta/broken-links.md

Config:
  Vault path is resolved from MIND_PALACE_VAULT env or vault.path in
  mind-palace.toml. See mind-palace.toml.example for the format.`)
}

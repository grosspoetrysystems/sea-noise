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

	// Subcommands land in later waves. Until then any positional argument
	// is an unknown subcommand.
	fmt.Fprintf(os.Stderr, "sea-noise: unknown subcommand %q\n\n", flag.Arg(0))
	printUsage()
	os.Exit(2)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `sea-noise — local retrieval and context-assembly runtime over a markdown vault

Usage:
  sea-noise [flags] <subcommand> [args...]

Flags:
  --help       show this help
  --version    print version and exit

Subcommands:
  (none yet — added in subsequent waves)

Config:
  Vault path is resolved from MIND_PALACE_VAULT env or vault.path in
  mind-palace.toml. See mind-palace.toml.example for the format.`)
}

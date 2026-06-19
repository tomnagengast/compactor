package main

import (
	"fmt"
	"os"
)

var version = "dev"

const usage = `compactor

Progressive disclosure for agent compaction.

Usage:
  compactor --help
  compactor --version

Status:
  Planning scaffold. Compaction commands are not implemented yet.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Print(usage)
		return
	}

	if args[0] == "--version" || args[0] == "-v" || args[0] == "version" {
		fmt.Printf("compactor %s\n", version)
		return
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", args[0], usage)
	os.Exit(2)
}

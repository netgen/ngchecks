// Package cli implements the ngcheck command line interface.
package cli

import (
	"fmt"
	"io"

	"github.com/netgen/ngcheck/internal/betterleaks"
)

// Exit codes shared by all checks.
const (
	ExitOK       = 0
	ExitFindings = 1
	ExitError    = 2
)

const usage = `ngcheck - Netgen project checks

Usage:
  ngcheck init [path]                Set up the git hook and CI check of a project
  ngcheck secrets [options] [path]   Scan a git repository for leaked secrets
  ngcheck version                    Show the ngcheck and Betterleaks versions
  ngcheck help                       Show this help

Run "ngcheck <command> --help" for the options of a command.
Exit codes: 0 passed, 1 problems found, 2 error.
`

// Run executes ngcheck with the given arguments (without the program name).
func Run(args []string, stdout, stderr io.Writer, getenv func(string) string, version string) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitError
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "secrets":
		return runSecrets(args[1:], stdout, stderr, getenv)
	case "version", "--version":
		return runVersion(stdout, stderr, getenv, version)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "ngcheck: unknown command %q\n\n%s", args[0], usage)
		return ExitError
	}
}

func runVersion(stdout, stderr io.Writer, getenv func(string) string, version string) int {
	fmt.Fprintf(stdout, "ngcheck %s\n", version)

	binary, err := betterleaks.Locate(getenv)
	if err == nil {
		var betterleaksVersion string
		if betterleaksVersion, err = betterleaks.Version(binary); err == nil {
			fmt.Fprintf(stdout, "betterleaks %s (%s)\n", betterleaksVersion, binary)
			return ExitOK
		}
	}

	fmt.Fprintf(stderr, "ngcheck: error: %v\n", err)

	return ExitError
}

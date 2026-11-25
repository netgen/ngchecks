// Command ngcheck runs Netgen project checks, such as scanning for leaked secrets.
package main

import (
	"os"

	"github.com/netgen/ngcheck/internal/cli"
)

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv, version))
}

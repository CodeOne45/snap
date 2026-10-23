// Command snap is the entry point wiring the CLI adapter to the core.
package main

import (
	"os"

	"github.com/CodeOne45/snap/internal/adapters/cli"
)

func main() {
	err := cli.NewRootCmd().Execute()
	os.Exit(cli.ExitCodeFor(err))
}

// Command benchforge is the local benchmark CLI.
package main

import (
	"os"

	"github.com/coderRaj07/benchforge/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
	// What: runs the benchforge CLI and exits with its status code.
	// Used by: the benchforge process, package main only.
	// Logic: os.Args[1:] is the command and flags. Stdout and stderr are the process streams. cli.Main returns 0, 1, 2, or 3.
}

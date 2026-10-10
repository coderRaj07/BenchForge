// Command benchforge is the local benchmark CLI.
package main

import (
	"os"

	"github.com/coderRaj07/benchforge/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}

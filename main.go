package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/ravisuhag/astro/cli"
)

//go:embed docs/cli/*.md
var docsFS embed.FS

func main() {
	cmd := cli.New(docsFS)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

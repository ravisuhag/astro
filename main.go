package main

import (
	"fmt"
	"os"

	"github.com/ravisuhag/astro/cli"
)

func main() {
	cmd := cli.New()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

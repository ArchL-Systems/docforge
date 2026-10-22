package main

import (
	"fmt"
	"os"

	"github.com/gabbzin/docforge/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "docforge:", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	"github.com/tomnagengast/compactor/internal/cli"
	"github.com/tomnagengast/compactor/internal/version"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version.String()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

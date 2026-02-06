package main

import (
	"fmt"
	"os"

	"github.com/GainForest/hypercerts-cli/cmd"
)

func main() {
	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

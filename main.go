package main

import (
	"os"

	"github.com/patrickserrano/brewmaster/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(2)
	}
}

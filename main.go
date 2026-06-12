package main

import (
	"errors"
	"os"

	"github.com/patrickserrano/brewmaster/cmd"
)

func main() {
	err := cmd.NewRootCmd().Execute()
	switch {
	case err == nil:
	case errors.Is(err, cmd.ErrAdoptableFound):
		os.Exit(1)
	default:
		os.Exit(2)
	}
}

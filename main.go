package main

import (
	"errors"
	"os"

	"github.com/patrickserrano/brewmaster/cmd"
)

func main() {
	os.Exit(exitCode(cmd.NewRootCmd().Execute()))
}

// exitCode maps command errors to the documented exit codes: 0 clean,
// 1 drift-like (adoptable apps found, or positional args matched no
// adoptable app — nothing broke, work didn't apply), 2 real errors.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, cmd.ErrAdoptableFound), errors.Is(err, cmd.ErrUnmatchedArgs):
		return 1
	default:
		return 2
	}
}

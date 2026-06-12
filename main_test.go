package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/patrickserrano/brewmaster/cmd"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is clean", nil, 0},
		{"adoptable drift exits 1", cmd.ErrAdoptableFound, 1},
		{"unmatched adopt args exit 1", cmd.ErrUnmatchedArgs, 1},
		{"wrapped unmatched args exit 1",
			fmt.Errorf("2 argument(s) %w", cmd.ErrUnmatchedArgs), 1},
		{"real errors exit 2", errors.New("brew missing"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootShowsHelp(t *testing.T) {
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "brewmaster") {
		t.Errorf("help output missing program name: %q", out.String())
	}
}

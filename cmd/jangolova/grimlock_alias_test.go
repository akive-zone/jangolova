package main

import (
	"strings"
	"testing"
)

func TestGrimlockCommandRejectsMultipleProtocolModes(t *testing.T) {
	err := grimlockCommand([]string{"--mcp", "--acp"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("grimlockCommand error = %v, want mutually-exclusive protocol error", err)
	}
}

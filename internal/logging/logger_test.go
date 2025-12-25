package logging

import (
	"bytes"
	"testing"
)

func TestNewLoggerFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)

	logger.Debug("hidden")
	if buf.Len() != 0 {
		t.Fatalf("expected debug to be filtered")
	}

	logger.Info("shown")
	if buf.Len() == 0 {
		t.Fatalf("expected info to be written")
	}
}

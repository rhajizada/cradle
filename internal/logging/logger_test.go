package logging_test

import (
	"bytes"
	"testing"

	"github.com/rhajizada/cradle/internal/logging"
)

func TestNewLoggerFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf)

	logger.Debug("hidden")
	if buf.Len() != 0 {
		t.Fatalf("expected debug to be filtered")
	}

	logger.Info("shown")
	if buf.Len() == 0 {
		t.Fatalf("expected info to be written")
	}
}

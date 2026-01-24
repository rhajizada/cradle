package cli_test

import (
	"testing"

	"github.com/rhajizada/cradle/internal/cli"
)

func TestExecuteVersion(t *testing.T) {
	if err := cli.ExecuteArgs("test", []string{"-V"}); err != nil {
		t.Fatalf("execute error: %v", err)
	}
}

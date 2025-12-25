package cli

import (
	"os"
	"testing"
)

func TestExecuteVersion(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	os.Args = []string{"cradle", "-V"}
	Execute("test")
}

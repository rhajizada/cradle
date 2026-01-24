package cli

import (
	"os"
	"path/filepath"
)

func DefaultConfigPath() string {
	if xdg, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdg != "" {
		return filepath.Join(xdg, "cradle", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".cradle.yaml"
	}
	return filepath.Join(home, ".config", "cradle", "config.yaml")
}

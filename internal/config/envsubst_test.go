package config_test

import (
	"testing"

	"github.com/rhajizada/cradle/internal/config"
)

func TestExpandEnvBasic(t *testing.T) {
	t.Setenv("FOO", "bar")
	t.Setenv("EMPTY", "")

	got, err := config.ExpandEnv("x:$FOO:y:${FOO}")
	if err != nil {
		t.Fatalf("ExpandEnv error: %v", err)
	}
	if got != "x:bar:y:bar" {
		t.Fatalf("unexpected value: %q", got)
	}

	got, err = config.ExpandEnv("${EMPTY:-fallback}")
	if err != nil {
		t.Fatalf("ExpandEnv error: %v", err)
	}
	if got != "fallback" {
		t.Fatalf("unexpected value: %q", got)
	}
}

func TestExpandEnvDefaults(t *testing.T) {
	t.Setenv("SET", "ok")
	got, err := config.ExpandEnv("${SET-default}-${UNSET-default}-${UNSET:-alt}-${SET:-alt}")
	if err != nil {
		t.Fatalf("ExpandEnv error: %v", err)
	}
	if got != "ok-default-alt-ok" {
		t.Fatalf("unexpected value: %q", got)
	}
}

func TestExpandEnvEscapes(t *testing.T) {
	got, err := config.ExpandEnv(`$$:\$:\$${FOO}`)
	if err != nil {
		t.Fatalf("ExpandEnv error: %v", err)
	}
	if got != "$:$:$" {
		t.Fatalf("unexpected value: %q", got)
	}
}

func TestExpandEnvBadSyntax(t *testing.T) {
	_, err := config.ExpandEnv("${FOO")
	if err == nil {
		t.Fatalf("expected error for bad expansion syntax")
	}
}

func TestExpandEnvUnset(t *testing.T) {
	got, err := config.ExpandEnv("x$UNSETy")
	if err != nil {
		t.Fatalf("ExpandEnv error: %v", err)
	}
	if got != "x" {
		t.Fatalf("unexpected value: %q", got)
	}
}

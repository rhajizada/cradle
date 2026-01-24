package service_test

import (
	"reflect"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"
)

func TestNormalizeTrimmedSlice(t *testing.T) {
	got := service.NormalizeTrimmedSlice([]string{" a ", "", "b", "  "})
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestMapToEnv(t *testing.T) {
	out := service.MapToEnv(map[string]string{"A": "1", "B": "2"})
	if len(out) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(out))
	}
}

func TestBoolDefault(t *testing.T) {
	if service.BoolDefault(nil, true) != true {
		t.Fatalf("expected default true")
	}
	v := false
	if service.BoolDefault(&v, true) != false {
		t.Fatalf("expected explicit false")
	}
}

func TestToDockerMounts(t *testing.T) {
	mounts := []config.MountSpec{
		{Type: "bind", Source: "/src", Target: "/dst", ReadOnly: true},
		{Type: "volume", Source: "data", Target: "/data"},
		{Type: "tmpfs", Target: "/tmp"},
	}
	out := service.ToDockerMounts(mounts)
	if len(out) != 3 {
		t.Fatalf("expected 3 mounts, got %d", len(out))
	}
	if out[0].Target != "/dst" || !out[0].ReadOnly {
		t.Fatalf("unexpected bind mount: %#v", out[0])
	}
	if out[1].Type == "" || out[2].Type == "" {
		t.Fatalf("expected mount types to be set")
	}
}

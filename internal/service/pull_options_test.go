package service_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/registry"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"
)

func TestPullOptionsFromSpec(t *testing.T) {
	spec := &config.PullSpec{
		Ref:      "ghcr.io/org/app:latest",
		Platform: "linux/amd64",
		Auth: &config.RegistryAuthSpec{
			Username:      "demo",
			Password:      "secret",
			ServerAddress: "ghcr.io",
		},
	}
	opts, err := service.PullOptionsFromSpec(spec)
	if err != nil {
		t.Fatalf("PullOptionsFromSpec error: %v", err)
	}
	if len(opts.Platforms) != 1 || opts.Platforms[0].OS != "linux" || opts.Platforms[0].Architecture != "amd64" {
		t.Fatalf("unexpected platform options: %+v", opts.Platforms)
	}
	if opts.RegistryAuth == "" {
		t.Fatalf("expected registry auth")
	}

	decoded, err := base64.StdEncoding.DecodeString(opts.RegistryAuth)
	if err != nil {
		t.Fatalf("decode auth: %v", err)
	}
	var auth registry.AuthConfig
	if unmarshalErr := json.Unmarshal(decoded, &auth); unmarshalErr != nil {
		t.Fatalf("unmarshal auth: %v", unmarshalErr)
	}
	if auth.Username != "demo" || auth.Password != "secret" || auth.ServerAddress != "ghcr.io" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
}

func TestPullOptionsFromSpecNoAuth(t *testing.T) {
	spec := &config.PullSpec{Ref: "ubuntu:24.04"}
	opts, err := service.PullOptionsFromSpec(spec)
	if err != nil {
		t.Fatalf("PullOptionsFromSpec error: %v", err)
	}
	if opts.RegistryAuth != "" {
		t.Fatalf("expected empty registry auth")
	}
}

func TestPullOptionsFromSpecNil(t *testing.T) {
	if _, err := service.PullOptionsFromSpec(nil); err == nil {
		t.Fatalf("expected error for nil spec")
	}
}

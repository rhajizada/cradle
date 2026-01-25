package service_test

import (
	"testing"

	"github.com/moby/moby/client"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"
)

func TestBuildOptionsFromSpecDefaults(t *testing.T) {
	spec := &config.BuildSpec{}
	opts, err := service.BuildOptionsFromSpec(spec, "demo:latest")
	if err != nil {
		t.Fatalf("BuildOptionsFromSpec error: %v", err)
	}
	if len(opts.Tags) != 1 || opts.Tags[0] != "demo:latest" {
		t.Fatalf("unexpected tags: %+v", opts.Tags)
	}
	if opts.Dockerfile != "Dockerfile" {
		t.Fatalf("unexpected dockerfile: %q", opts.Dockerfile)
	}
	if !opts.Remove || !opts.ForceRemove {
		t.Fatalf("expected remove defaults true")
	}
}

func TestBuildOptionsFromSpecOverrides(t *testing.T) {
	remove := false
	forceRemove := false
	spec := &config.BuildSpec{
		Tags:           []string{"extra:tag"},
		Dockerfile:     "Dockerfile.dev",
		SuppressOutput: true,
		RemoteContext:  "https://example.com/repo.git",
		Remove:         &remove,
		ForceRemove:    &forceRemove,
		Network:        "host",
		ExtraHosts:     []string{"host.docker.internal:host-gateway"},
		BuildID:        "build-123",
		Platforms:      []string{"linux/amd64"},
		Args: map[string]string{
			"ONE": "1",
		},
		Labels: map[string]string{
			"org.example.role": "dev",
		},
		SecurityOpt: []string{"seccomp=unconfined"},
		Ulimits: []config.UlimitSpec{{
			Name: "nofile",
			Soft: 1024,
			Hard: 2048,
		}},
		AuthConfigs: map[string]config.BuildAuthConfig{
			"ghcr.io": {
				Username:      "demo",
				Password:      "secret",
				Auth:          "token",
				ServerAddress: "ghcr.io",
				IdentityToken: "id",
				RegistryToken: "reg",
			},
		},
		Outputs: []config.BuildOutputSpec{{
			Type: "local",
			Attrs: map[string]string{
				"dest": "./out",
			},
		}},
	}

	opts, err := service.BuildOptionsFromSpec(spec, "demo:latest")
	if err != nil {
		t.Fatalf("BuildOptionsFromSpec error: %v", err)
	}
	assertBuildOverridesBasics(t, opts)
	assertBuildOverridesArgsLabels(t, opts)
	assertBuildOverridesOutputs(t, opts)
	assertBuildOverridesAuthUlimits(t, opts)
}

func TestBuildOptionsFromSpecResources(t *testing.T) {
	spec := &config.BuildSpec{
		Isolation:    "hyperv",
		CPUSetCPUs:   "0-2",
		CPUSetMems:   "0",
		CPUShares:    256,
		CPUQuota:     50000,
		CPUPeriod:    100000,
		Memory:       128 * 1024 * 1024,
		MemorySwap:   256 * 1024 * 1024,
		CgroupParent: "/my/cgroup",
		ShmSize:      64 * 1024 * 1024,
		PullParent:   true,
		NoCache:      true,
		CacheFrom:    []string{"cache:latest"},
		Squash:       true,
	}

	opts, err := service.BuildOptionsFromSpec(spec, "demo:latest")
	if err != nil {
		t.Fatalf("BuildOptionsFromSpec error: %v", err)
	}
	if opts.Isolation != "hyperv" {
		t.Fatalf("unexpected isolation: %q", opts.Isolation)
	}
	if opts.CPUSetCPUs != "0-2" || opts.CPUSetMems != "0" {
		t.Fatalf("unexpected cpuset settings")
	}
	if opts.CPUShares != 256 || opts.CPUQuota != 50000 || opts.CPUPeriod != 100000 {
		t.Fatalf("unexpected cpu settings")
	}
	if opts.Memory != 128*1024*1024 || opts.MemorySwap != 256*1024*1024 {
		t.Fatalf("unexpected memory settings")
	}
	if opts.CgroupParent != "/my/cgroup" || opts.ShmSize != 64*1024*1024 {
		t.Fatalf("unexpected cgroup/shm settings")
	}
	if !opts.PullParent || !opts.NoCache || !opts.Squash {
		t.Fatalf("unexpected cache flags")
	}
	if len(opts.CacheFrom) != 1 || opts.CacheFrom[0] != "cache:latest" {
		t.Fatalf("unexpected cache_from: %+v", opts.CacheFrom)
	}
}

func assertBuildOverridesBasics(t *testing.T, opts client.ImageBuildOptions) {
	t.Helper()
	if len(opts.Tags) != 2 || opts.Tags[0] != "demo:latest" || opts.Tags[1] != "extra:tag" {
		t.Fatalf("unexpected tags: %+v", opts.Tags)
	}
	if opts.Dockerfile != "Dockerfile.dev" {
		t.Fatalf("unexpected dockerfile: %q", opts.Dockerfile)
	}
	if !opts.SuppressOutput || opts.RemoteContext == "" {
		t.Fatalf("unexpected output/context settings")
	}
	if opts.Remove || opts.ForceRemove {
		t.Fatalf("expected remove overrides to be false")
	}
	if opts.NetworkMode != "host" {
		t.Fatalf("unexpected network mode: %q", opts.NetworkMode)
	}
	if len(opts.ExtraHosts) != 1 || opts.ExtraHosts[0] != "host.docker.internal:host-gateway" {
		t.Fatalf("unexpected extra hosts: %+v", opts.ExtraHosts)
	}
	if opts.BuildID != "build-123" {
		t.Fatalf("unexpected build id: %q", opts.BuildID)
	}
	if len(opts.Platforms) != 1 {
		t.Fatalf("unexpected platforms: %+v", opts.Platforms)
	}
}

func assertBuildOverridesArgsLabels(t *testing.T, opts client.ImageBuildOptions) {
	t.Helper()
	if opts.BuildArgs["ONE"] == nil || *opts.BuildArgs["ONE"] != "1" {
		t.Fatalf("unexpected build args: %+v", opts.BuildArgs)
	}
	if opts.Labels["org.example.role"] != "dev" {
		t.Fatalf("unexpected labels: %+v", opts.Labels)
	}
	if len(opts.SecurityOpt) != 1 || opts.SecurityOpt[0] != "seccomp=unconfined" {
		t.Fatalf("unexpected security opt: %+v", opts.SecurityOpt)
	}
}

func assertBuildOverridesOutputs(t *testing.T, opts client.ImageBuildOptions) {
	t.Helper()
	if len(opts.Outputs) != 1 {
		t.Fatalf("unexpected outputs: %+v", opts.Outputs)
	}
	if opts.Outputs[0].Type != "local" || opts.Outputs[0].Attrs["dest"] != "./out" {
		t.Fatalf("unexpected outputs: %+v", opts.Outputs)
	}
}

func assertBuildOverridesAuthUlimits(t *testing.T, opts client.ImageBuildOptions) {
	t.Helper()
	if len(opts.Ulimits) != 1 {
		t.Fatalf("unexpected ulimits: %+v", opts.Ulimits)
	}
	if opts.Ulimits[0].Name != "nofile" || opts.Ulimits[0].Soft != 1024 || opts.Ulimits[0].Hard != 2048 {
		t.Fatalf("unexpected ulimit values: %+v", opts.Ulimits[0])
	}
	auth := opts.AuthConfigs["ghcr.io"]
	if auth.Username != "demo" || auth.Password != "secret" || auth.Auth != "token" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
	if auth.ServerAddress != "ghcr.io" || auth.IdentityToken != "id" || auth.RegistryToken != "reg" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
}

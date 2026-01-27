package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
)

func TestLoadFileResolvesPaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      build:
        cwd: ./images/demo
    run:
      volumes:
        - type: bind
          source: ./src
          target: /workspace
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	a := cfg.Aliases["demo"]
	if a.Image.Build == nil {
		t.Fatalf("expected build spec")
	}
	wantCwd := filepath.Join(dir, "images", "demo")
	if a.Image.Build.Cwd != wantCwd {
		t.Fatalf("cwd not resolved: got %q want %q", a.Image.Build.Cwd, wantCwd)
	}

	if len(a.Run.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(a.Run.Volumes))
	}
	wantSrc := filepath.Join(dir, "src")
	if a.Run.Volumes[0].Source != wantSrc {
		t.Fatalf("volume source not resolved: got %q want %q", a.Run.Volumes[0].Source, wantSrc)
	}
}

func TestLoadFileUnknownField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      pull:
        ref: ubuntu:24.04
    run:
      bogus: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := config.LoadFile(cfgPath); err == nil {
		t.Fatalf("expected error for unknown field")
	}
}

func TestValidateUIDGID(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}},
				Run:   config.RunSpec{UID: 1000},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error when gid missing")
	}
}

func TestValidateMountType(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}},
				Run: config.RunSpec{
					Volumes: []config.MountSpec{{Type: "bad", Target: "/x"}},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid mount type")
	}
}

func TestValidateImageSpecErrors(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing image spec")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{
					Pull:  &config.PullSpec{Ref: "ubuntu:24.04"},
					Build: &config.BuildSpec{Cwd: "/tmp"},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for pull+build")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: ""}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for empty pull ref")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: ""}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing build cwd")
	}
}

func TestValidateImagePolicyDefaults(t *testing.T) {
	cfg := &config.Config{Aliases: map[string]config.Alias{
		"pull":  {Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}}},
		"build": {Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: "/tmp"}}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if cfg.Aliases["pull"].Image.Pull.Policy != config.ImagePolicyAlways {
		t.Fatalf("expected pull policy default to always")
	}
	if cfg.Aliases["build"].Image.Build.Policy != config.ImagePolicyAlways {
		t.Fatalf("expected build policy default to always")
	}
}

func TestValidateImagePolicyInvalid(t *testing.T) {
	cfg := &config.Config{Aliases: map[string]config.Alias{
		"pull": {Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04", Policy: "bad"}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid pull policy")
	}

	cfg = &config.Config{Aliases: map[string]config.Alias{
		"build": {Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: "/tmp", Policy: "bad"}}},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid build policy")
	}
}

func TestValidateImagePolicyValues(t *testing.T) {
	cfg := &config.Config{Aliases: map[string]config.Alias{
		"pull": {
			Image: config.ImageSpec{
				Pull: &config.PullSpec{
					Ref:    "ubuntu:24.04",
					Policy: config.ImagePolicyIfMissing,
				},
			},
		},
		"build": {
			Image: config.ImageSpec{
				Build: &config.BuildSpec{
					Cwd:    "/tmp",
					Policy: config.ImagePolicyNever,
				},
			},
		},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if cfg.Aliases["pull"].Image.Pull.Policy != config.ImagePolicyIfMissing {
		t.Fatalf("expected pull policy if_missing")
	}
	if cfg.Aliases["build"].Image.Build.Policy != config.ImagePolicyNever {
		t.Fatalf("expected build policy never")
	}
}

func TestLoadFileBuildOptionsExtended(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      build:
        cwd: ./images/demo
        dockerfile: Dockerfile.dev
        args:
          ONE: "1"
        target: runtime
        labels:
          org.example.role: dev
        pull: true
        no_cache: true
        cache_from:
          - ghcr.io/org/app:cache
        tags:
          - demo:latest
        suppress_output: true
        remote_context: https://example.com/repo.git
        remove: false
        force_remove: false
        isolation: hyperv
        cpuset_cpus: "0-1"
        cpuset_mems: "0"
        cpu_shares: 128
        cpu_quota: 50000
        cpu_period: 100000
        memory: 104857600
        memory_swap: 209715200
        cgroup_parent: /my/cgroup
        shm_size: 67108864
        ulimits:
          - name: nofile
            soft: 1024
            hard: 2048
        auth_configs:
          ghcr.io:
            username: demo
            password: secret
            auth: token
            server_address: ghcr.io
            identity_token: id
            registry_token: reg
        squash: true
        security_opt:
          - seccomp=unconfined
        build_id: build-123
        outputs:
          - type: local
            attrs:
              dest: ./out
        network: host
        extra_hosts:
          - "host.docker.internal:host-gateway"
        platforms:
          - linux/amd64
    run:
      cmd: ["/bin/true"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	build := cfg.Aliases["demo"].Image.Build
	assertBuildBasics(t, build, dir)
	assertBuildCacheAndTags(t, build)
	assertBuildResources(t, build)
	assertBuildAuthAndOutputs(t, build)
	assertBuildNetworkAndPlatforms(t, build)
}

func TestLoadFileBuildRemoteContextNoCwd(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      build:
        remote_context: https://example.com/repo.git
    run:
      cmd: ["/bin/true"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	build := cfg.Aliases["demo"].Image.Build
	if build == nil {
		t.Fatalf("expected build spec")
	}
	if build.Cwd != "" {
		t.Fatalf("expected empty cwd with remote context, got %q", build.Cwd)
	}
	if build.RemoteContext != "https://example.com/repo.git" {
		t.Fatalf("unexpected remote_context: %q", build.RemoteContext)
	}
}

func TestLoadFilePullPlatformAuth(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      pull:
        ref: ghcr.io/org/app:latest
        platform: linux/amd64
        auth:
          username: demo
          password: secret
          server_address: ghcr.io
    run:
      cmd: ["/bin/true"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	pull := cfg.Aliases["demo"].Image.Pull
	if pull == nil {
		t.Fatalf("expected pull spec")
	}
	if pull.Platform != "linux/amd64" {
		t.Fatalf("unexpected platform: %q", pull.Platform)
	}
	if pull.Auth == nil || pull.Auth.Username != "demo" || pull.Auth.Password != "secret" {
		t.Fatalf("unexpected auth: %+v", pull.Auth)
	}
}

func TestLoadFileRunComposeFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      pull:
        ref: ubuntu:24.04
    run:
      work_dir: /workspace
      domain_name: example.local
      network_mode: host
      volumes:
        - type: bind
          source: ./src
          target: /workspace
      read_only: true
      stop_grace_period: 30s
      healthcheck:
        test: ["CMD", "true"]
        interval: 30s
      logging:
        driver: json-file
        options:
          max-size: 10m
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	run := cfg.Aliases["demo"].Run
	if run.WorkDir != "/workspace" {
		t.Fatalf("unexpected work_dir: %q", run.WorkDir)
	}
	if run.DomainName != "example.local" {
		t.Fatalf("unexpected domain_name: %q", run.DomainName)
	}
	if run.NetworkMode != "host" {
		t.Fatalf("unexpected network_mode: %q", run.NetworkMode)
	}
	if len(run.Volumes) != 1 {
		t.Fatalf("expected volumes")
	}
	if run.Volumes[0].Source != filepath.Join(dir, "src") {
		t.Fatalf("unexpected volume source: %q", run.Volumes[0].Source)
	}
	if !run.ReadOnly {
		t.Fatalf("expected read_only true")
	}
	if run.StopGracePeriod != "30s" {
		t.Fatalf("unexpected stop_grace_period: %q", run.StopGracePeriod)
	}
	if run.HealthCheck == nil || len(run.HealthCheck.Test) == 0 {
		t.Fatalf("expected healthcheck")
	}
	if run.Logging == nil || run.Logging.Driver != "json-file" {
		t.Fatalf("unexpected logging config: %+v", run.Logging)
	}
}

func assertBuildBasics(t *testing.T, build *config.BuildSpec, dir string) {
	t.Helper()
	if build == nil {
		t.Fatalf("expected build spec")
	}
	if build.Cwd != filepath.Join(dir, "images", "demo") {
		t.Fatalf("unexpected cwd: %q", build.Cwd)
	}
	if build.Dockerfile != "Dockerfile.dev" {
		t.Fatalf("unexpected dockerfile: %q", build.Dockerfile)
	}
	if build.Args["ONE"] != "1" {
		t.Fatalf("unexpected build args: %+v", build.Args)
	}
	if build.Target != "runtime" {
		t.Fatalf("unexpected target: %q", build.Target)
	}
	if build.Labels["org.example.role"] != "dev" {
		t.Fatalf("unexpected labels: %+v", build.Labels)
	}
}

func assertBuildCacheAndTags(t *testing.T, build *config.BuildSpec) {
	t.Helper()
	if !build.PullParent || !build.NoCache {
		t.Fatalf("unexpected pull/no_cache: %v %v", build.PullParent, build.NoCache)
	}
	if len(build.CacheFrom) != 1 || build.CacheFrom[0] != "ghcr.io/org/app:cache" {
		t.Fatalf("unexpected cache_from: %+v", build.CacheFrom)
	}
	if len(build.Tags) != 1 || build.Tags[0] != "demo:latest" {
		t.Fatalf("unexpected tags: %+v", build.Tags)
	}
	if !build.SuppressOutput || build.RemoteContext == "" {
		t.Fatalf("unexpected suppress_output/remote_context: %v %q", build.SuppressOutput, build.RemoteContext)
	}
	if build.Remove == nil || build.ForceRemove == nil || *build.Remove || *build.ForceRemove {
		t.Fatalf("unexpected remove flags: %v %v", build.Remove, build.ForceRemove)
	}
}

func assertBuildResources(t *testing.T, build *config.BuildSpec) {
	t.Helper()
	if build.Isolation != "hyperv" {
		t.Fatalf("unexpected isolation: %q", build.Isolation)
	}
	if build.CPUSetCPUs != "0-1" || build.CPUSetMems != "0" {
		t.Fatalf("unexpected cpuset settings")
	}
	if build.CPUShares != 128 || build.CPUQuota != 50000 || build.CPUPeriod != 100000 {
		t.Fatalf("unexpected cpu settings")
	}
	if build.Memory != 104857600 || build.MemorySwap != 209715200 {
		t.Fatalf("unexpected memory settings")
	}
	if build.CgroupParent != "/my/cgroup" || build.ShmSize != 67108864 {
		t.Fatalf("unexpected cgroup/shm settings")
	}
	if len(build.Ulimits) != 1 {
		t.Fatalf("unexpected ulimits: %+v", build.Ulimits)
	}
	if build.Ulimits[0].Name != "nofile" || build.Ulimits[0].Soft != 1024 || build.Ulimits[0].Hard != 2048 {
		t.Fatalf("unexpected ulimit values: %+v", build.Ulimits[0])
	}
}

func assertBuildAuthAndOutputs(t *testing.T, build *config.BuildSpec) {
	t.Helper()
	auth := build.AuthConfigs["ghcr.io"]
	if auth.Username != "demo" || auth.Password != "secret" || auth.Auth != "token" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
	if auth.ServerAddress != "ghcr.io" || auth.IdentityToken != "id" || auth.RegistryToken != "reg" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
	if !build.Squash {
		t.Fatalf("expected squash true")
	}
	if len(build.SecurityOpt) != 1 || build.SecurityOpt[0] != "seccomp=unconfined" {
		t.Fatalf("unexpected security_opt: %+v", build.SecurityOpt)
	}
	if build.BuildID != "build-123" {
		t.Fatalf("unexpected build_id: %q", build.BuildID)
	}
	if len(build.Outputs) != 1 {
		t.Fatalf("unexpected outputs: %+v", build.Outputs)
	}
	if build.Outputs[0].Type != "local" || build.Outputs[0].Attrs["dest"] != "./out" {
		t.Fatalf("unexpected outputs: %+v", build.Outputs)
	}
}

func assertBuildNetworkAndPlatforms(t *testing.T, build *config.BuildSpec) {
	t.Helper()
	if build.Network != "host" {
		t.Fatalf("unexpected network: %q", build.Network)
	}
	if len(build.ExtraHosts) != 1 || build.ExtraHosts[0] != "host.docker.internal:host-gateway" {
		t.Fatalf("unexpected extra_hosts: %+v", build.ExtraHosts)
	}
	if len(build.Platforms) != 1 || build.Platforms[0] != "linux/amd64" {
		t.Fatalf("unexpected platforms: %+v", build.Platforms)
	}
}

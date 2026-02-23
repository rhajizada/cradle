package service_test

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"

	"github.com/moby/moby/api/types/container"
	mobynet "github.com/moby/moby/api/types/network"
)

func TestBuildContainerCreateOptionsComposeFields(t *testing.T) {
	pidsLimit := int64(128)
	run := config.RunSpec{
		User:       "1000:1000",
		WorkDir:    "/work",
		Hostname:   "app",
		DomainName: "example.local",
		Env: map[string]string{
			"APP_ENV": "test",
		},
		Labels: map[string]string{
			"app": "demo",
		},
		Ports:  []string{"8080:80"},
		Expose: []string{"9000"},
		ExtraHosts: []string{
			"example.local:127.0.0.1",
		},
		DNS:         []string{"1.1.1.1"},
		DNSSearch:   []string{"example.local"},
		DNSOptions:  []string{"ndots:1"},
		NetworkMode: "bridge",
		Networks: map[string]config.NetworkSpec{
			"default": {Aliases: []string{"demo"}},
		},
		Volumes: []config.MountSpec{{
			Type:     "bind",
			Source:   "/src",
			Target:   "/dst",
			ReadOnly: true,
		}},
		Resources: &config.ResourcesSpec{
			CPUs:              2,
			CPUShares:         128,
			CPUQuota:          50000,
			CPUPeriod:         100000,
			CPUSetCPUs:        "0-1",
			Memory:            "64m",
			MemoryReservation: "32m",
			MemorySwap:        "128m",
			PidsLimit:         &pidsLimit,
		},
		Ulimits: []config.UlimitSpec{{
			Name: "nofile",
			Soft: 1024,
			Hard: 2048,
		}},
		Devices: []string{"/dev/null:/dev/null:rwm"},
		GPUs: []config.GPURequestSpec{{
			Driver:       "nvidia",
			Count:        config.DeviceCountAll,
			DeviceIDs:    []string{"0", "1"},
			Capabilities: []string{"compute", "utility"},
			Options: map[string]string{
				"profile": "dev",
			},
		}},
		Tmpfs:       []string{"/run:rw,noexec"},
		ReadOnly:    true,
		CapAdd:      []string{"NET_ADMIN"},
		CapDrop:     []string{"SYS_ADMIN"},
		GroupAdd:    []string{"audio"},
		SecurityOpt: []string{"seccomp=unconfined"},
		Sysctls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
		StopSignal:      "SIGTERM",
		StopGracePeriod: "30s",
		HealthCheck: &config.HealthCheckSpec{
			Test:     []string{"CMD", "true"},
			Interval: "30s",
			Timeout:  "5s",
		},
		Logging: &config.LogConfigSpec{
			Driver: "json-file",
			Options: map[string]string{
				"max-size": "10m",
			},
		},
	}

	opts, err := service.BuildContainerCreateOptions(
		"demo",
		run,
		"image:tag",
		"fingerprint",
		true,
		true,
		false,
	)
	if err != nil {
		t.Fatalf("BuildContainerCreateOptions error: %v", err)
	}

	assertContainerConfig(t, opts.Config)
	assertHostConfig(t, opts.HostConfig)
	assertNetworkingConfig(t, opts.NetworkingConfig)
}

func assertContainerConfig(t *testing.T, cfg *container.Config) {
	t.Helper()
	if cfg == nil {
		t.Fatalf("expected container config")
	}
	if cfg.User != "1000:1000" {
		t.Fatalf("unexpected user: %q", cfg.User)
	}
	if cfg.WorkingDir != "/work" {
		t.Fatalf("unexpected work dir: %q", cfg.WorkingDir)
	}
	if cfg.Domainname != "example.local" {
		t.Fatalf("unexpected domain name: %q", cfg.Domainname)
	}
	if cfg.Labels["app"] != "demo" {
		t.Fatalf("unexpected labels: %+v", cfg.Labels)
	}
	if cfg.StopSignal != "SIGTERM" {
		t.Fatalf("unexpected stop signal: %q", cfg.StopSignal)
	}
	if cfg.StopTimeout == nil || *cfg.StopTimeout != 30 {
		t.Fatalf("unexpected stop timeout: %v", cfg.StopTimeout)
	}
	if cfg.Healthcheck == nil {
		t.Fatalf("expected healthcheck")
	}
	if cfg.Healthcheck.Interval != 30*time.Second {
		t.Fatalf("unexpected healthcheck interval: %v", cfg.Healthcheck.Interval)
	}
	port80, _ := mobynet.ParsePort("80")
	port9000, _ := mobynet.ParsePort("9000")
	if _, ok := cfg.ExposedPorts[port80]; !ok {
		t.Fatalf("expected port 80 exposed")
	}
	if _, ok := cfg.ExposedPorts[port9000]; !ok {
		t.Fatalf("expected port 9000 exposed")
	}
}

func assertHostConfig(t *testing.T, hostCfg *container.HostConfig) {
	t.Helper()
	if hostCfg == nil {
		t.Fatalf("expected host config")
	}
	assertHostConfigBasics(t, hostCfg)
	assertHostConfigResources(t, hostCfg)
	assertHostConfigNetworking(t, hostCfg)
}

func assertHostConfigBasics(t *testing.T, hostCfg *container.HostConfig) {
	t.Helper()
	if !hostCfg.ReadonlyRootfs {
		t.Fatalf("expected readonly rootfs")
	}
	if len(hostCfg.CapAdd) != 1 || hostCfg.CapAdd[0] != "NET_ADMIN" {
		t.Fatalf("unexpected cap_add: %+v", hostCfg.CapAdd)
	}
	if len(hostCfg.CapDrop) != 1 || hostCfg.CapDrop[0] != "SYS_ADMIN" {
		t.Fatalf("unexpected cap_drop: %+v", hostCfg.CapDrop)
	}
	if len(hostCfg.GroupAdd) != 1 || hostCfg.GroupAdd[0] != "audio" {
		t.Fatalf("unexpected group_add: %+v", hostCfg.GroupAdd)
	}
	if hostCfg.LogConfig.Type != "json-file" {
		t.Fatalf("unexpected log config: %+v", hostCfg.LogConfig)
	}
}

func assertHostConfigResources(t *testing.T, hostCfg *container.HostConfig) {
	t.Helper()
	if hostCfg.Resources.NanoCPUs != 2_000_000_000 {
		t.Fatalf("unexpected NanoCPUs: %d", hostCfg.Resources.NanoCPUs)
	}
	if len(hostCfg.Resources.Ulimits) != 1 {
		t.Fatalf("unexpected ulimits: %+v", hostCfg.Resources.Ulimits)
	}
	if len(hostCfg.Resources.Devices) != 1 {
		t.Fatalf("unexpected devices: %+v", hostCfg.Resources.Devices)
	}
	if len(hostCfg.Resources.DeviceRequests) != 1 {
		t.Fatalf("unexpected device requests: %+v", hostCfg.Resources.DeviceRequests)
	}
	request := hostCfg.Resources.DeviceRequests[0]
	if request.Driver != "nvidia" {
		t.Fatalf("unexpected gpu driver: %q", request.Driver)
	}
	if request.Count != -1 {
		t.Fatalf("unexpected gpu count: %d", request.Count)
	}
	if len(request.DeviceIDs) != 2 || request.DeviceIDs[0] != "0" || request.DeviceIDs[1] != "1" {
		t.Fatalf("unexpected gpu device ids: %+v", request.DeviceIDs)
	}
	if len(request.Capabilities) != 1 || len(request.Capabilities[0]) != 3 {
		t.Fatalf("unexpected gpu capabilities: %+v", request.Capabilities)
	}
	if request.Capabilities[0][2] != "gpu" {
		t.Fatalf("expected gpu capability, got: %+v", request.Capabilities)
	}
	if _, ok := hostCfg.Tmpfs["/run"]; !ok {
		t.Fatalf("expected tmpfs entry")
	}
}

func assertHostConfigNetworking(t *testing.T, hostCfg *container.HostConfig) {
	t.Helper()
	if len(hostCfg.DNS) != 1 || hostCfg.DNS[0] != netip.MustParseAddr("1.1.1.1") {
		t.Fatalf("unexpected dns: %+v", hostCfg.DNS)
	}
	if len(hostCfg.PortBindings) == 0 {
		t.Fatalf("expected port bindings")
	}
}

func assertNetworkingConfig(t *testing.T, netCfg *mobynet.NetworkingConfig) {
	t.Helper()
	if netCfg == nil || netCfg.EndpointsConfig == nil {
		t.Fatalf("expected networking config")
	}
	endpoint := netCfg.EndpointsConfig["default"]
	if endpoint == nil || len(endpoint.Aliases) != 1 || endpoint.Aliases[0] != "demo" {
		t.Fatalf("unexpected endpoint config: %+v", endpoint)
	}
}

func TestBuildContainerCreateOptionsInvalidInputs(t *testing.T) {
	badHealthcheck := &config.HealthCheckSpec{
		Test:     []string{"CMD", "true"},
		Interval: "invalid",
	}
	cases := []struct {
		name string
		run  config.RunSpec
		want string
	}{
		{
			name: "invalid stop_grace_period",
			run: config.RunSpec{
				StopGracePeriod: "notaduration",
			},
			want: "stop_grace_period",
		},
		{
			name: "invalid healthcheck interval",
			run: config.RunSpec{
				HealthCheck: badHealthcheck,
			},
			want: "healthcheck.interval",
		},
		{
			name: "invalid tmpfs",
			run: config.RunSpec{
				Tmpfs: []string{":ro"},
			},
			want: "tmpfs",
		},
		{
			name: "invalid device",
			run: config.RunSpec{
				Devices: []string{"a:"},
			},
			want: "device",
		},
		{
			name: "invalid dns",
			run: config.RunSpec{
				DNS: []string{"nope"},
			},
			want: "dns",
		},
		{
			name: "invalid expose",
			run: config.RunSpec{
				Expose: []string{"bad"},
			},
			want: "expose",
		},
		{
			name: "invalid ports",
			run: config.RunSpec{
				Ports: []string{"bad:format:too:many"},
			},
			want: "port",
		},
		{
			name: "invalid memory",
			run: config.RunSpec{
				Resources: &config.ResourcesSpec{
					Memory: "bad",
				},
			},
			want: "run.resources.memory",
		},
		{
			name: "invalid memory reservation",
			run: config.RunSpec{
				Resources: &config.ResourcesSpec{
					MemoryReservation: "bad",
				},
			},
			want: "run.resources.memory_reservation",
		},
		{
			name: "invalid memory swap",
			run: config.RunSpec{
				Resources: &config.ResourcesSpec{
					MemorySwap: "bad",
				},
			},
			want: "run.resources.memory_swap",
		},
	}

	for _, tc := range cases {
		_, err := service.BuildContainerCreateOptions(
			"demo",
			tc.run,
			"image:tag",
			"fingerprint",
			false,
			false,
			false,
		)
		if err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// BaseDir is the directory containing the config file; useful for resolving relative paths.
	BaseDir string `json:"-" yaml:"-"`

	Version int              `json:"version" yaml:"version"`
	Aliases map[string]Alias `json:"aliases" yaml:"aliases"`
}

type Alias struct {
	Image ImageSpec `json:"image" yaml:"image"`
	Run   RunSpec   `json:"run"   yaml:"run"`
}

type ImageSpec struct {
	// Exactly one of Pull or Build should be set.
	Pull  *PullSpec  `json:"pull,omitempty"  yaml:"pull,omitempty"`
	Build *BuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
}

type ImagePolicy string

const (
	ImagePolicyAlways    ImagePolicy = "always"
	ImagePolicyIfMissing ImagePolicy = "if_missing"
	ImagePolicyNever     ImagePolicy = "never"
)

type PullSpec struct {
	Ref    string      `json:"ref"              yaml:"ref"`
	Policy ImagePolicy `json:"policy,omitempty" yaml:"policy,omitempty"`
	// e.g. ubuntu:24.04
	// Optional: later you can add platform, auth, etc.
}

type BuildSpec struct {
	Cwd        string            `json:"cwd"                  yaml:"cwd"`                  // context root (your “cwd”)
	Dockerfile string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"` // default: Dockerfile
	Args       map[string]string `json:"args,omitempty"       yaml:"args,omitempty"`
	Target     string            `json:"target,omitempty"     yaml:"target,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"     yaml:"labels,omitempty"`
	Policy     ImagePolicy       `json:"policy,omitempty"     yaml:"policy,omitempty"`

	PullParent bool     `json:"pull,omitempty"       yaml:"pull,omitempty"` // maps to PullParent
	NoCache    bool     `json:"no_cache,omitempty"   yaml:"no_cache,omitempty"`
	CacheFrom  []string `json:"cache_from,omitempty" yaml:"cache_from,omitempty"`

	Tags           []string                   `json:"tags,omitempty"            yaml:"tags,omitempty"`
	SuppressOutput bool                       `json:"suppress_output,omitempty" yaml:"suppress_output,omitempty"`
	RemoteContext  string                     `json:"remote_context,omitempty"  yaml:"remote_context,omitempty"`
	Remove         *bool                      `json:"remove,omitempty"          yaml:"remove,omitempty"`
	ForceRemove    *bool                      `json:"force_remove,omitempty"    yaml:"force_remove,omitempty"`
	Isolation      string                     `json:"isolation,omitempty"       yaml:"isolation,omitempty"`
	CPUSetCPUs     string                     `json:"cpuset_cpus,omitempty"     yaml:"cpuset_cpus,omitempty"`
	CPUSetMems     string                     `json:"cpuset_mems,omitempty"     yaml:"cpuset_mems,omitempty"`
	CPUShares      int64                      `json:"cpu_shares,omitempty"      yaml:"cpu_shares,omitempty"`
	CPUQuota       int64                      `json:"cpu_quota,omitempty"       yaml:"cpu_quota,omitempty"`
	CPUPeriod      int64                      `json:"cpu_period,omitempty"      yaml:"cpu_period,omitempty"`
	Memory         int64                      `json:"memory,omitempty"          yaml:"memory,omitempty"`
	MemorySwap     int64                      `json:"memory_swap,omitempty"     yaml:"memory_swap,omitempty"`
	CgroupParent   string                     `json:"cgroup_parent,omitempty"   yaml:"cgroup_parent,omitempty"`
	ShmSize        int64                      `json:"shm_size,omitempty"        yaml:"shm_size,omitempty"`
	Ulimits        []UlimitSpec               `json:"ulimits,omitempty"         yaml:"ulimits,omitempty"`
	AuthConfigs    map[string]BuildAuthConfig `json:"auth_configs,omitempty"    yaml:"auth_configs,omitempty"`
	Squash         bool                       `json:"squash,omitempty"          yaml:"squash,omitempty"`
	SecurityOpt    []string                   `json:"security_opt,omitempty"    yaml:"security_opt,omitempty"`
	BuildID        string                     `json:"build_id,omitempty"        yaml:"build_id,omitempty"`
	Outputs        []BuildOutputSpec          `json:"outputs,omitempty"         yaml:"outputs,omitempty"`

	Network    string   `json:"network,omitempty"     yaml:"network,omitempty"`     // e.g. "host"
	ExtraHosts []string `json:"extra_hosts,omitempty" yaml:"extra_hosts,omitempty"` // ["host.docker.internal:host-gateway"]

	Platforms []string `json:"platforms,omitempty" yaml:"platforms,omitempty"` // e.g. ["linux/amd64"]
}

type UlimitSpec struct {
	Name string `json:"name"           yaml:"name"`
	Soft int64  `json:"soft,omitempty" yaml:"soft,omitempty"`
	Hard int64  `json:"hard,omitempty" yaml:"hard,omitempty"`
}

type BuildAuthConfig struct {
	Username      string `json:"username,omitempty"       yaml:"username,omitempty"`
	Password      string `json:"password,omitempty"       yaml:"password,omitempty"`
	Auth          string `json:"auth,omitempty"           yaml:"auth,omitempty"`
	ServerAddress string `json:"server_address,omitempty" yaml:"server_address,omitempty"`
	IdentityToken string `json:"identity_token,omitempty" yaml:"identity_token,omitempty"`
	RegistryToken string `json:"registry_token,omitempty" yaml:"registry_token,omitempty"`
}

type BuildOutputSpec struct {
	Type  string            `json:"type,omitempty"  yaml:"type,omitempty"`
	Attrs map[string]string `json:"attrs,omitempty" yaml:"attrs,omitempty"`
}

type RunSpec struct {
	// Identity (user or numeric uid/gid)
	UID  int    `json:"uid,omitempty"  yaml:"uid,omitempty"`
	GID  int    `json:"gid,omitempty"  yaml:"gid,omitempty"`
	User string `json:"user,omitempty" yaml:"user,omitempty"`

	// UX defaults for interactive shells
	TTY        *bool `json:"tty,omitempty"         yaml:"tty,omitempty"`         // default false if nil
	StdinOpen  *bool `json:"stdin_open,omitempty"  yaml:"stdin_open,omitempty"`  // default false if nil
	AutoRemove *bool `json:"auto_remove,omitempty" yaml:"auto_remove,omitempty"` // default false if nil
	Attach     *bool `json:"attach,omitempty"      yaml:"attach,omitempty"`      // default false if nil

	Name       string `json:"name,omitempty"        yaml:"name,omitempty"`        // optional; else generated
	Hostname   string `json:"hostname,omitempty"    yaml:"hostname,omitempty"`    // optional
	DomainName string `json:"domain_name,omitempty" yaml:"domain_name,omitempty"` // optional

	WorkDir    string            `json:"work_dir,omitempty"   yaml:"work_dir,omitempty"`
	Env        map[string]string `json:"env,omitempty"        yaml:"env,omitempty"` // rendered into KEY=VAL
	Entrypoint []string          `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"        yaml:"cmd,omitempty"`

	NetworkMode string                 `json:"network_mode,omitempty" yaml:"network_mode,omitempty"` // bridge|host|none|<network>
	Networks    map[string]NetworkSpec `json:"networks,omitempty"     yaml:"networks,omitempty"`
	Ports       []string               `json:"ports,omitempty"        yaml:"ports,omitempty"` // ["8080:80", "127.0.0.1:2222:22"]
	Expose      []string               `json:"expose,omitempty"       yaml:"expose,omitempty"`
	ExtraHosts  []string               `json:"extra_hosts,omitempty"  yaml:"extra_hosts,omitempty"`
	DNS         []string               `json:"dns,omitempty"          yaml:"dns,omitempty"`
	DNSSearch   []string               `json:"dns_search,omitempty"   yaml:"dns_search,omitempty"`
	DNSOptions  []string               `json:"dns_opt,omitempty"      yaml:"dns_opt,omitempty"`
	IPC         string                 `json:"ipc,omitempty"          yaml:"ipc,omitempty"`
	PID         string                 `json:"pid,omitempty"          yaml:"pid,omitempty"`
	UTS         string                 `json:"uts,omitempty"          yaml:"uts,omitempty"`
	Runtime     string                 `json:"runtime,omitempty"      yaml:"runtime,omitempty"`

	Volumes []MountSpec `json:"volumes,omitempty" yaml:"volumes,omitempty"`

	Resources       *ResourcesSpec    `json:"resources,omitempty"         yaml:"resources,omitempty"`
	Privileged      bool              `json:"privileged,omitempty"        yaml:"privileged,omitempty"`
	ReadOnly        bool              `json:"read_only,omitempty"         yaml:"read_only,omitempty"`
	CapAdd          []string          `json:"cap_add,omitempty"           yaml:"cap_add,omitempty"`
	CapDrop         []string          `json:"cap_drop,omitempty"          yaml:"cap_drop,omitempty"`
	SecurityOpt     []string          `json:"security_opt,omitempty"      yaml:"security_opt,omitempty"`
	Sysctls         map[string]string `json:"sysctls,omitempty"           yaml:"sysctls,omitempty"`
	Ulimits         []UlimitSpec      `json:"ulimits,omitempty"           yaml:"ulimits,omitempty"`
	Tmpfs           []string          `json:"tmpfs,omitempty"             yaml:"tmpfs,omitempty"`
	Devices         []string          `json:"devices,omitempty"           yaml:"devices,omitempty"`
	GroupAdd        []string          `json:"group_add,omitempty"         yaml:"group_add,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"            yaml:"labels,omitempty"`
	StopSignal      string            `json:"stop_signal,omitempty"       yaml:"stop_signal,omitempty"`
	StopGracePeriod string            `json:"stop_grace_period,omitempty" yaml:"stop_grace_period,omitempty"`
	HealthCheck     *HealthCheckSpec  `json:"healthcheck,omitempty"       yaml:"healthcheck,omitempty"`
	Logging         *LogConfigSpec    `json:"logging,omitempty"           yaml:"logging,omitempty"`
	Restart         string            `json:"restart,omitempty"           yaml:"restart,omitempty"` // "no", "on-failure", "always", "unless-stopped"

	Platform string `json:"platform,omitempty" yaml:"platform,omitempty"` // optional override, e.g. linux/amd64
}

type NetworkSpec struct {
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

type MountSpec struct {
	// type: bind|volume|tmpfs (start with bind+volume)
	Type     string `json:"type"                yaml:"type"`
	Source   string `json:"source,omitempty"    yaml:"source,omitempty"`
	Target   string `json:"target"              yaml:"target"`
	ReadOnly bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

type ResourcesSpec struct {
	CPUs              float64 `json:"cpus,omitempty"               yaml:"cpus,omitempty"` // maps to NanoCPUs (cpus * 1e9)
	CPUShares         int64   `json:"cpu_shares,omitempty"         yaml:"cpu_shares,omitempty"`
	CPUQuota          int64   `json:"cpu_quota,omitempty"          yaml:"cpu_quota,omitempty"`
	CPUPeriod         int64   `json:"cpu_period,omitempty"         yaml:"cpu_period,omitempty"`
	CPUSetCPUs        string  `json:"cpuset_cpus,omitempty"        yaml:"cpuset_cpus,omitempty"`
	CPUSetMems        string  `json:"cpuset_mems,omitempty"        yaml:"cpuset_mems,omitempty"`
	Memory            string  `json:"memory,omitempty"             yaml:"memory,omitempty"` // e.g. "512m", "2g" (parse later)
	MemoryReservation string  `json:"memory_reservation,omitempty" yaml:"memory_reservation,omitempty"`
	MemorySwap        string  `json:"memory_swap,omitempty"        yaml:"memory_swap,omitempty"`
	PidsLimit         *int64  `json:"pids_limit,omitempty"         yaml:"pids_limit,omitempty"`
	OomKillDisable    *bool   `json:"oom_kill_disable,omitempty"   yaml:"oom_kill_disable,omitempty"`
	CgroupParent      string  `json:"cgroup_parent,omitempty"      yaml:"cgroup_parent,omitempty"`
	ShmSize           string  `json:"shm_size,omitempty"           yaml:"shm_size,omitempty"`
}

type HealthCheckSpec struct {
	Test          []string `json:"test,omitempty"           yaml:"test,omitempty"`
	Interval      string   `json:"interval,omitempty"       yaml:"interval,omitempty"`
	Timeout       string   `json:"timeout,omitempty"        yaml:"timeout,omitempty"`
	Retries       *int     `json:"retries,omitempty"        yaml:"retries,omitempty"`
	StartPeriod   string   `json:"start_period,omitempty"   yaml:"start_period,omitempty"`
	StartInterval string   `json:"start_interval,omitempty" yaml:"start_interval,omitempty"`
	Disable       bool     `json:"disable,omitempty"        yaml:"disable,omitempty"`
}

type LogConfigSpec struct {
	Driver  string            `json:"driver,omitempty"  yaml:"driver,omitempty"`
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

func LoadFile(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded, err := ExpandEnv(string(raw))
	if err != nil {
		return nil, fmt.Errorf("env expansion failed: %w", err)
	}

	cfg := &Config{
		BaseDir: filepath.Dir(absPath),
	}

	dec := yaml.NewDecoder(bytes.NewReader([]byte(expanded)))
	dec.KnownFields(true) // strict: unknown keys become errors

	if decodeErr := dec.Decode(cfg); decodeErr != nil {
		return nil, fmt.Errorf("parse yaml: %w", decodeErr)
	}

	if cfg.Aliases == nil {
		cfg.Aliases = map[string]Alias{}
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, validateErr
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	for name, alias := range c.Aliases {
		updated, err := c.validateAlias(name, alias)
		if err != nil {
			return err
		}
		c.Aliases[name] = updated
	}

	return nil
}

func (c *Config) validateAlias(name string, alias Alias) (Alias, error) {
	if err := validateImage(name, &alias, c.BaseDir); err != nil {
		return alias, err
	}

	if err := validateRun(name, &alias, c.BaseDir); err != nil {
		return alias, err
	}

	return alias, nil
}

func validateImage(name string, alias *Alias, baseDir string) error {
	if alias.Image.Pull == nil && alias.Image.Build == nil {
		return fmt.Errorf("aliases.%s.image: must specify either pull or build", name)
	}
	if alias.Image.Pull != nil && alias.Image.Build != nil {
		return fmt.Errorf("aliases.%s.image: cannot specify both pull and build", name)
	}

	if alias.Image.Pull != nil && alias.Image.Pull.Ref == "" {
		return fmt.Errorf("aliases.%s.image.pull.ref: required", name)
	}
	if alias.Image.Pull != nil {
		policy, err := normalizeImagePolicy(alias.Image.Pull.Policy, ImagePolicyAlways)
		if err != nil {
			return fmt.Errorf("aliases.%s.image.pull.policy: %w", name, err)
		}
		alias.Image.Pull.Policy = policy
	}

	if alias.Image.Build == nil {
		return nil
	}
	policy, err := normalizeImagePolicy(alias.Image.Build.Policy, ImagePolicyAlways)
	if err != nil {
		return fmt.Errorf("aliases.%s.image.build.policy: %w", name, err)
	}
	alias.Image.Build.Policy = policy

	if alias.Image.Build.Cwd == "" && alias.Image.Build.RemoteContext == "" {
		return fmt.Errorf("aliases.%s.image.build.cwd: required when remote_context is empty", name)
	}

	if alias.Image.Build.Cwd != "" {
		alias.Image.Build.Cwd = resolvePath(baseDir, alias.Image.Build.Cwd)
	}
	if alias.Image.Build.Dockerfile == "" {
		alias.Image.Build.Dockerfile = "Dockerfile"
	}

	return nil
}

func normalizeImagePolicy(value ImagePolicy, defaultPolicy ImagePolicy) (ImagePolicy, error) {
	if value == "" {
		return defaultPolicy, nil
	}
	switch value {
	case ImagePolicyAlways, ImagePolicyIfMissing, ImagePolicyNever:
		return value, nil
	default:
		return "", fmt.Errorf("invalid policy %q", value)
	}
}

func validateRun(name string, alias *Alias, baseDir string) error {
	if err := validateRunIDs(name, alias.Run); err != nil {
		return err
	}

	volumes, err := validateMounts(name, alias.Run.Volumes, baseDir)
	if err != nil {
		return err
	}

	alias.Run.Volumes = volumes
	return nil
}

func validateRunIDs(name string, run RunSpec) error {
	if run.UID == 0 && run.GID == 0 {
		return nil
	}
	if run.UID <= 0 {
		return fmt.Errorf("aliases.%s.run.uid: must be > 0 when set", name)
	}
	if run.GID <= 0 {
		return fmt.Errorf("aliases.%s.run.gid: must be > 0 when set", name)
	}
	return nil
}

func validateMounts(name string, volumes []MountSpec, baseDir string) ([]MountSpec, error) {
	validated := make([]MountSpec, len(volumes))
	for i, v := range volumes {
		vm, err := validateMount(name, i, v, baseDir)
		if err != nil {
			return nil, err
		}
		validated[i] = vm
	}
	return validated, nil
}

func validateMount(name string, idx int, volume MountSpec, baseDir string) (MountSpec, error) {
	if volume.Type == "" || volume.Target == "" {
		return volume, fmt.Errorf("aliases.%s.run.volumes[%d]: type and target are required", name, idx)
	}
	switch volume.Type {
	case "bind", "volume", "tmpfs":
	default:
		return volume, fmt.Errorf("aliases.%s.run.volumes[%d].type: must be bind|volume|tmpfs", name, idx)
	}

	if volume.Type != "tmpfs" && volume.Source == "" {
		return volume, fmt.Errorf("aliases.%s.run.volumes[%d].source: required for %s", name, idx, volume.Type)
	}

	if volume.Type == "bind" && volume.Source != "" && !filepath.IsAbs(volume.Source) {
		volume.Source = resolvePath(baseDir, volume.Source)
	}

	return volume, nil
}

var ErrBadExpansion = errors.New("bad ${...} expansion syntax")

func resolvePath(baseDir, p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

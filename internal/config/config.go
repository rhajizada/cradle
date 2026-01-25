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

type PullSpec struct {
	Ref string `json:"ref" yaml:"ref"` // e.g. ubuntu:24.04
	// Optional: later you can add platform, auth, etc.
}

type BuildSpec struct {
	Cwd        string            `json:"cwd"                  yaml:"cwd"`                  // context root (your “cwd”)
	Dockerfile string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"` // default: Dockerfile
	Args       map[string]string `json:"args,omitempty"       yaml:"args,omitempty"`
	Target     string            `json:"target,omitempty"     yaml:"target,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"     yaml:"labels,omitempty"`

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
	// Identity (your choice: expose uid/gid/username only)
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	UID      int    `json:"uid,omitempty"      yaml:"uid,omitempty"`
	GID      int    `json:"gid,omitempty"      yaml:"gid,omitempty"`

	// UX defaults for interactive shells
	TTY        *bool `json:"tty,omitempty"         yaml:"tty,omitempty"`         // default false if nil
	StdinOpen  *bool `json:"stdin_open,omitempty"  yaml:"stdin_open,omitempty"`  // default false if nil
	AutoRemove *bool `json:"auto_remove,omitempty" yaml:"auto_remove,omitempty"` // default false if nil
	Attach     *bool `json:"attach,omitempty"      yaml:"attach,omitempty"`      // default false if nil

	Name     string `json:"name,omitempty"     yaml:"name,omitempty"`     // optional; else generated
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"` // optional

	Workdir    string            `json:"workdir,omitempty"    yaml:"workdir,omitempty"`
	Env        map[string]string `json:"env,omitempty"        yaml:"env,omitempty"` // rendered into KEY=VAL
	Entrypoint []string          `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"        yaml:"cmd,omitempty"`

	Network    string   `json:"network,omitempty"     yaml:"network,omitempty"` // bridge|host|none|<network>
	Ports      []string `json:"ports,omitempty"       yaml:"ports,omitempty"`   // ["8080:80", "127.0.0.1:2222:22"]
	ExtraHosts []string `json:"extra_hosts,omitempty" yaml:"extra_hosts,omitempty"`

	Mounts []MountSpec `json:"mounts,omitempty" yaml:"mounts,omitempty"`

	Resources  *ResourcesSpec `json:"resources,omitempty"  yaml:"resources,omitempty"`
	Privileged bool           `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Restart    string         `json:"restart,omitempty"    yaml:"restart,omitempty"` // "no", "on-failure", "always", "unless-stopped"

	Platform string `json:"platform,omitempty" yaml:"platform,omitempty"` // optional override, e.g. linux/amd64
}

type MountSpec struct {
	// type: bind|volume|tmpfs (start with bind+volume)
	Type     string `json:"type"               yaml:"type"`
	Source   string `json:"source,omitempty"   yaml:"source,omitempty"`
	Target   string `json:"target"             yaml:"target"`
	ReadOnly bool   `json:"readonly,omitempty" yaml:"readonly,omitempty"`
}

type ResourcesSpec struct {
	CPUs    float64 `json:"cpus,omitempty"     yaml:"cpus,omitempty"`   // maps to NanoCPUs (cpus * 1e9)
	Memory  string  `json:"memory,omitempty"   yaml:"memory,omitempty"` // e.g. "512m", "2g" (parse later)
	ShmSize string  `json:"shm_size,omitempty" yaml:"shm_size,omitempty"`
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

	if alias.Image.Build == nil {
		return nil
	}

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

func validateRun(name string, alias *Alias, baseDir string) error {
	if err := validateRunIDs(name, alias.Run); err != nil {
		return err
	}

	mounts, err := validateMounts(name, alias.Run.Mounts, baseDir)
	if err != nil {
		return err
	}

	alias.Run.Mounts = mounts
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

func validateMounts(name string, mounts []MountSpec, baseDir string) ([]MountSpec, error) {
	validated := make([]MountSpec, len(mounts))
	for i, m := range mounts {
		vm, err := validateMount(name, i, m, baseDir)
		if err != nil {
			return nil, err
		}
		validated[i] = vm
	}
	return validated, nil
}

func validateMount(name string, idx int, mount MountSpec, baseDir string) (MountSpec, error) {
	if mount.Type == "" || mount.Target == "" {
		return mount, fmt.Errorf("aliases.%s.run.mounts[%d]: type and target are required", name, idx)
	}
	switch mount.Type {
	case "bind", "volume", "tmpfs":
	default:
		return mount, fmt.Errorf("aliases.%s.run.mounts[%d].type: must be bind|volume|tmpfs", name, idx)
	}

	if mount.Type != "tmpfs" && mount.Source == "" {
		return mount, fmt.Errorf("aliases.%s.run.mounts[%d].source: required for %s", name, idx, mount.Type)
	}

	if mount.Type == "bind" && mount.Source != "" && !filepath.IsAbs(mount.Source) {
		mount.Source = resolvePath(baseDir, mount.Source)
	}

	return mount, nil
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

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
	Run   RunSpec   `json:"run" yaml:"run"`
}

type ImageSpec struct {
	// Exactly one of Pull or Build should be set.
	Pull  *PullSpec  `json:"pull,omitempty" yaml:"pull,omitempty"`
	Build *BuildSpec `json:"build,omitempty" yaml:"build,omitempty"`
}

type PullSpec struct {
	Ref string `json:"ref" yaml:"ref"` // e.g. ubuntu:24.04
	// Optional: later you can add platform, auth, etc.
}

type BuildSpec struct {
	Cwd        string            `json:"cwd" yaml:"cwd"`                                   // context root (your “cwd”)
	Dockerfile string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"` // default: Dockerfile
	Args       map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
	Target     string            `json:"target,omitempty" yaml:"target,omitempty"`
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	PullParent bool     `json:"pull,omitempty" yaml:"pull,omitempty"` // maps to PullParent
	NoCache    bool     `json:"no_cache,omitempty" yaml:"no_cache,omitempty"`
	CacheFrom  []string `json:"cache_from,omitempty" yaml:"cache_from,omitempty"`

	// Keep minimal; add more only when you actually need them.
	Network    string   `json:"network,omitempty" yaml:"network,omitempty"`         // e.g. "host"
	ExtraHosts []string `json:"extra_hosts,omitempty" yaml:"extra_hosts,omitempty"` // ["host.docker.internal:host-gateway"]

	Platforms []string `json:"platforms,omitempty" yaml:"platforms,omitempty"` // e.g. ["linux/amd64"]
}

type RunSpec struct {
	// Identity (your choice: expose uid/gid/username only)
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	UID      int    `json:"uid,omitempty" yaml:"uid,omitempty"`
	GID      int    `json:"gid,omitempty" yaml:"gid,omitempty"`

	// UX defaults for interactive shells
	TTY        *bool `json:"tty,omitempty" yaml:"tty,omitempty"`                 // default false if nil
	StdinOpen  *bool `json:"stdin_open,omitempty" yaml:"stdin_open,omitempty"`   // default false if nil
	AutoRemove *bool `json:"auto_remove,omitempty" yaml:"auto_remove,omitempty"` // default false if nil
	Attach     *bool `json:"attach,omitempty" yaml:"attach,omitempty"`           // default false if nil

	Name     string `json:"name,omitempty" yaml:"name,omitempty"`         // optional; else generated
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"` // optional

	Workdir    string            `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Env        map[string]string `json:"env,omitempty" yaml:"env,omitempty"` // rendered into KEY=VAL
	Entrypoint []string          `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Cmd        []string          `json:"cmd,omitempty" yaml:"cmd,omitempty"`

	Network    string   `json:"network,omitempty" yaml:"network,omitempty"` // bridge|host|none|<network>
	Ports      []string `json:"ports,omitempty" yaml:"ports,omitempty"`     // ["8080:80", "127.0.0.1:2222:22"]
	ExtraHosts []string `json:"extra_hosts,omitempty" yaml:"extra_hosts,omitempty"`

	Mounts []MountSpec `json:"mounts,omitempty" yaml:"mounts,omitempty"`

	Resources  ResourcesSpec `json:"resources,omitempty" yaml:"resources,omitempty"`
	Privileged bool          `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Restart    string        `json:"restart,omitempty" yaml:"restart,omitempty"` // "no", "on-failure", "always", "unless-stopped"

	Platform string `json:"platform,omitempty" yaml:"platform,omitempty"` // optional override, e.g. linux/amd64
}

type MountSpec struct {
	// type: bind|volume|tmpfs (start with bind+volume)
	Type     string `json:"type" yaml:"type"`
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`
	Target   string `json:"target" yaml:"target"`
	ReadOnly bool   `json:"readonly,omitempty" yaml:"readonly,omitempty"`
}

type ResourcesSpec struct {
	CPUs    float64 `json:"cpus,omitempty" yaml:"cpus,omitempty"`     // maps to NanoCPUs (cpus * 1e9)
	Memory  string  `json:"memory,omitempty" yaml:"memory,omitempty"` // e.g. "512m", "2g" (parse later)
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

	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if cfg.Aliases == nil {
		cfg.Aliases = map[string]Alias{}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	for name, a := range c.Aliases {
		if a.Image.Pull == nil && a.Image.Build == nil {
			return fmt.Errorf("aliases.%s.image: must specify either pull or build", name)
		}
		if a.Image.Pull != nil && a.Image.Build != nil {
			return fmt.Errorf("aliases.%s.image: cannot specify both pull and build", name)
		}

		if a.Image.Pull != nil {
			if a.Image.Pull.Ref == "" {
				return fmt.Errorf("aliases.%s.image.pull.ref: required", name)
			}
		}

		if a.Image.Build != nil {
			if a.Image.Build.Cwd == "" {
				return fmt.Errorf("aliases.%s.image.build.cwd: required", name)
			}
			a.Image.Build.Cwd = resolvePath(c.BaseDir, a.Image.Build.Cwd)
			// default dockerfile
			if a.Image.Build.Dockerfile == "" {
				a.Image.Build.Dockerfile = "Dockerfile"
			}
		}

		// Run validation
		if a.Run.UID != 0 || a.Run.GID != 0 {
			if a.Run.UID <= 0 {
				return fmt.Errorf("aliases.%s.run.uid: must be > 0 when set", name)
			}
			if a.Run.GID <= 0 {
				return fmt.Errorf("aliases.%s.run.gid: must be > 0 when set", name)
			}
		}

		// mounts basic validation
		for i, m := range a.Run.Mounts {
			if m.Type == "" || m.Target == "" {
				return fmt.Errorf("aliases.%s.run.mounts[%d]: type and target are required", name, i)
			}
			switch m.Type {
			case "bind", "volume", "tmpfs":
			default:
				return fmt.Errorf("aliases.%s.run.mounts[%d].type: must be bind|volume|tmpfs", name, i)
			}
			if m.Type != "tmpfs" && m.Source == "" {
				return fmt.Errorf("aliases.%s.run.mounts[%d].source: required for %s", name, i, m.Type)
			}
			if m.Type == "bind" && m.Source != "" && !filepath.IsAbs(m.Source) {
				m.Source = resolvePath(c.BaseDir, m.Source)
				a.Run.Mounts[i] = m
			}
		}

		c.Aliases[name] = a
	}

	return nil
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

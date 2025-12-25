package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/containerd/errdefs"
	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	mobynet "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"golang.org/x/term"
)

type RunResult struct {
	ID         string
	AutoRemove bool
	Attach     bool
	TTY        bool
}

type AttachOptions struct {
	ID         string
	AutoRemove bool
	TTY        bool
	Stdin      *os.File
	Stdout     io.Writer
}

const containerFingerprintLabel = "io.cradle.fingerprint"

func (s *Service) Run(ctx context.Context, alias string, out io.Writer) (*RunResult, error) {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return nil, fmt.Errorf("unknown alias %q", alias)
	}

	imageRef, err := s.ensureImage(ctx, alias, out)
	if err != nil {
		return nil, err
	}

	createName := a.Run.Name
	if createName == "" {
		createName = fmt.Sprintf("cradle-%s", alias)
	}

	tty := boolDefault(a.Run.TTY, false)
	stdinOpen := boolDefault(a.Run.StdinOpen, false)
	autoRemove := boolDefault(a.Run.AutoRemove, false)
	attach := boolDefault(a.Run.Attach, false)

	imageInfo, err := s.cli.ImageInspect(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	fingerprint, err := runFingerprint(alias, createName, imageRef, imageInfo.ID, a.Run, tty, stdinOpen, autoRemove)
	if err != nil {
		return nil, err
	}

	if result, ok, err := s.tryReuseContainer(ctx, createName, fingerprint, autoRemove, attach, tty); err != nil {
		return nil, err
	} else if ok {
		return result, nil
	}

	env := mapToEnv(a.Run.Env)

	userSpec := ""
	if a.Run.UID > 0 && a.Run.GID > 0 {
		userSpec = fmt.Sprintf("%d:%d", a.Run.UID, a.Run.GID)
	}

	resources := container.Resources{}
	if a.Run.Resources.CPUs > 0 {
		resources.NanoCPUs = int64(a.Run.Resources.CPUs * 1e9)
	}
	if a.Run.Resources.Memory != "" {
		mem, err := units.RAMInBytes(a.Run.Resources.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid run.resources.memory: %w", err)
		}
		resources.Memory = mem
	}

	hostCfg := &container.HostConfig{
		AutoRemove:  autoRemove,
		Privileged:  a.Run.Privileged,
		NetworkMode: container.NetworkMode(a.Run.Network),
		ExtraHosts:  a.Run.ExtraHosts,
		Mounts:      toDockerMounts(a.Run.Mounts),
		Resources:   resources,
	}
	if a.Run.Restart != "" {
		hostCfg.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(a.Run.Restart)}
	}
	if a.Run.Resources.ShmSize != "" {
		shm, err := units.RAMInBytes(a.Run.Resources.ShmSize)
		if err != nil {
			return nil, fmt.Errorf("invalid run.resources.shm_size: %w", err)
		}
		hostCfg.ShmSize = shm
	}

	exposed, bindings, err := parsePorts(a.Run.Ports)
	if err != nil {
		return nil, err
	}

	cfgCtr := &container.Config{
		Image:        imageRef,
		User:         userSpec,
		Env:          env,
		WorkingDir:   a.Run.Workdir,
		Entrypoint:   a.Run.Entrypoint,
		Cmd:          a.Run.Cmd,
		Hostname:     a.Run.Hostname,
		Tty:          tty,
		OpenStdin:    stdinOpen,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: exposed,
		Labels: map[string]string{
			containerFingerprintLabel: fingerprint,
		},
	}

	hostCfg.PortBindings = bindings

	createOpts := client.ContainerCreateOptions{
		Name:       createName,
		Config:     cfgCtr,
		HostConfig: hostCfg,
	}
	if a.Run.Platform != "" {
		platform, err := parsePlatform(a.Run.Platform)
		if err != nil {
			return nil, err
		}
		createOpts.Platform = platform
	}

	created, err := s.cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return nil, err
	}
	id := created.ID

	if _, err := s.cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	return &RunResult{
		ID:         id,
		AutoRemove: autoRemove,
		Attach:     attach,
		TTY:        tty,
	}, nil
}

func (s *Service) AttachAndWait(ctx context.Context, opts AttachOptions) error {
	attached, err := s.cli.ContainerAttach(ctx, opts.ID, client.ContainerAttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return err
	}
	defer attached.Close()

	if opts.TTY && term.IsTerminal(int(opts.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(opts.Stdin.Fd()))
		if err == nil {
			defer func() {
				_ = term.Restore(int(opts.Stdin.Fd()), oldState)
			}()
		}
	}

	if opts.TTY {
		resize := func() {
			w, h, err := term.GetSize(int(opts.Stdin.Fd()))
			if err != nil {
				return
			}
			_, _ = s.cli.ContainerResize(context.Background(), opts.ID, client.ContainerResizeOptions{
				Width:  uint(w),
				Height: uint(h),
			})
		}
		resize()
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		defer signal.Stop(winch)
		go func() {
			for range winch {
				resize()
			}
		}()
	}

	go func() { _, _ = io.Copy(attached.Conn, opts.Stdin) }()
	_, _ = io.Copy(opts.Stdout, attached.Reader)

	wait := s.cli.ContainerWait(ctx, opts.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case err := <-wait.Error:
		if err != nil {
			return err
		}
	case <-wait.Result:
	}

	if !opts.AutoRemove {
		return nil
	}
	_, _ = s.cli.ContainerRemove(context.Background(), opts.ID, client.ContainerRemoveOptions{Force: true})
	return nil
}

func (s *Service) tryReuseContainer(ctx context.Context, name, fingerprint string, autoRemove, attach, tty bool) (*RunResult, bool, error) {
	ctr, err := s.cli.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	labels := map[string]string{}
	if ctr.Container.Config != nil && ctr.Container.Config.Labels != nil {
		labels = ctr.Container.Config.Labels
	}

	if labels[containerFingerprintLabel] != fingerprint {
		if ctr.Container.State != nil && ctr.Container.State.Running {
			_, _ = s.cli.ContainerStop(ctx, ctr.Container.ID, client.ContainerStopOptions{})
		}
		_, _ = s.cli.ContainerRemove(ctx, ctr.Container.ID, client.ContainerRemoveOptions{Force: true})
		return nil, false, nil
	}

	if ctr.Container.State == nil || !ctr.Container.State.Running {
		if _, err := s.cli.ContainerStart(ctx, ctr.Container.ID, client.ContainerStartOptions{}); err != nil {
			return nil, false, err
		}
	}

	return &RunResult{
		ID:         ctr.Container.ID,
		AutoRemove: autoRemove,
		Attach:     attach,
		TTY:        tty,
	}, true, nil
}

type envKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type runFingerprintSpec struct {
	Alias    string `json:"alias"`
	Name     string `json:"name"`
	ImageRef string `json:"image_ref"`
	ImageID  string `json:"image_id"`
	Run      struct {
		Username   string               `json:"username"`
		UID        int                  `json:"uid"`
		GID        int                  `json:"gid"`
		TTY        bool                 `json:"tty"`
		StdinOpen  bool                 `json:"stdin_open"`
		AutoRemove bool                 `json:"auto_remove"`
		Hostname   string               `json:"hostname"`
		Workdir    string               `json:"workdir"`
		Env        []envKV              `json:"env"`
		Entrypoint []string             `json:"entrypoint"`
		Cmd        []string             `json:"cmd"`
		Network    string               `json:"network"`
		Ports      []string             `json:"ports"`
		ExtraHosts []string             `json:"extra_hosts"`
		Mounts     []config.MountSpec   `json:"mounts"`
		Resources  config.ResourcesSpec `json:"resources"`
		Privileged bool                 `json:"privileged"`
		Restart    string               `json:"restart"`
		Platform   string               `json:"platform"`
	} `json:"run"`
}

func runFingerprint(alias, name, imageRef, imageID string, run config.RunSpec, tty, stdinOpen, autoRemove bool) (string, error) {
	spec := runFingerprintSpec{
		Alias:    alias,
		Name:     name,
		ImageRef: imageRef,
		ImageID:  imageID,
	}

	envKeys := make([]string, 0, len(run.Env))
	for k := range run.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	envList := make([]envKV, 0, len(envKeys))
	for _, k := range envKeys {
		envList = append(envList, envKV{Key: k, Value: run.Env[k]})
	}

	ports := normalizeTrimmedSlice(run.Ports)
	extraHosts := normalizeTrimmedSlice(run.ExtraHosts)

	spec.Run.Username = run.Username
	spec.Run.UID = run.UID
	spec.Run.GID = run.GID
	spec.Run.TTY = tty
	spec.Run.StdinOpen = stdinOpen
	spec.Run.AutoRemove = autoRemove
	spec.Run.Hostname = run.Hostname
	spec.Run.Workdir = run.Workdir
	spec.Run.Env = envList
	spec.Run.Entrypoint = run.Entrypoint
	spec.Run.Cmd = run.Cmd
	spec.Run.Network = run.Network
	spec.Run.Ports = ports
	spec.Run.ExtraHosts = extraHosts
	spec.Run.Mounts = run.Mounts
	spec.Run.Resources = run.Resources
	spec.Run.Privileged = run.Privileged
	spec.Run.Restart = run.Restart
	spec.Run.Platform = run.Platform

	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeTrimmedSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func mapToEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func boolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func toDockerMounts(ms []config.MountSpec) []mount.Mount {
	out := make([]mount.Mount, 0, len(ms))
	for _, m := range ms {
		switch m.Type {
		case "bind":
			out = append(out, mount.Mount{
				Type:     mount.TypeBind,
				Source:   m.Source,
				Target:   m.Target,
				ReadOnly: m.ReadOnly,
			})
		case "volume":
			out = append(out, mount.Mount{
				Type:     mount.TypeVolume,
				Source:   m.Source,
				Target:   m.Target,
				ReadOnly: m.ReadOnly,
			})
		case "tmpfs":
			out = append(out, mount.Mount{
				Type:   mount.TypeTmpfs,
				Target: m.Target,
			})
		}
	}
	return out
}

func parsePorts(specs []string) (mobynet.PortSet, mobynet.PortMap, error) {
	if len(specs) == 0 {
		return nil, nil, nil
	}

	exposed := mobynet.PortSet{}
	bindings := mobynet.PortMap{}

	for _, s := range specs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		var hostIPStr, hostPortStr, containerPortStr string
		if strings.HasPrefix(s, "[") {
			end := strings.Index(s, "]")
			if end == -1 {
				return nil, nil, fmt.Errorf("invalid port mapping %q", s)
			}
			hostIPStr = s[1:end]
			if end+2 >= len(s) || s[end+1] != ':' {
				return nil, nil, fmt.Errorf("invalid port mapping %q", s)
			}
			rest := s[end+2:]
			parts := strings.Split(rest, ":")
			if len(parts) != 2 {
				return nil, nil, fmt.Errorf("invalid port mapping %q", s)
			}
			hostPortStr = parts[0]
			containerPortStr = parts[1]
		} else {
			parts := strings.Split(s, ":")
			switch len(parts) {
			case 1:
				containerPortStr = parts[0]
			case 2:
				hostPortStr = parts[0]
				containerPortStr = parts[1]
			case 3:
				hostIPStr = parts[0]
				hostPortStr = parts[1]
				containerPortStr = parts[2]
			default:
				return nil, nil, fmt.Errorf("invalid port mapping %q", s)
			}
		}

		port, err := mobynet.ParsePort(containerPortStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid container port %q in %q: %w", containerPortStr, s, err)
		}

		exposed[port] = struct{}{}

		if hostPortStr != "" {
			var hostIP netip.Addr
			if hostIPStr != "" {
				ip, err := netip.ParseAddr(hostIPStr)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid host ip %q in %q: %w", hostIPStr, s, err)
				}
				hostIP = ip
			}

			bindings[port] = append(bindings[port], mobynet.PortBinding{
				HostIP:   hostIP,
				HostPort: hostPortStr,
			})
		}
	}

	return exposed, bindings, nil
}

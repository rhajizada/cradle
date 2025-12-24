package service

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rhajizada/cradle/internal/config"

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

	tty := boolDefault(a.Run.TTY, true)
	stdinOpen := boolDefault(a.Run.StdinOpen, true)
	autoRemove := boolDefault(a.Run.AutoRemove, true)
	attach := boolDefault(a.Run.Attach, true)

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
		_, _ = s.cli.ContainerRemove(context.Background(), opts.ID, client.ContainerRemoveOptions{Force: true})
	}
	return nil
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

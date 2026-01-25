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
const (
	nanoCPUsPerCPU    = 1_000_000_000
	singlePortPart    = 1
	hostPortParts     = 2
	hostWithIPParts   = 3
	ipv6PortPartCount = 2
)

type runFlags struct {
	tty        bool
	stdinOpen  bool
	autoRemove bool
	attach     bool
}

func (s *Service) Run(ctx context.Context, alias string, out io.Writer) (*RunResult, error) {
	a, found := s.cfg.Aliases[alias]
	if !found {
		return nil, fmt.Errorf("unknown alias %q", alias)
	}

	imageRef, err := s.EnsureImage(ctx, alias, out)
	if err != nil {
		return nil, err
	}

	createName := defaultContainerName(alias, a.Run.Name)
	flags := runFlags{
		tty:        BoolDefault(a.Run.TTY, false),
		stdinOpen:  BoolDefault(a.Run.StdinOpen, false),
		autoRemove: BoolDefault(a.Run.AutoRemove, false),
		attach:     BoolDefault(a.Run.Attach, false),
	}

	imageInfo, err := s.cli.ImageInspect(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	fingerprint, err := RunFingerprint(
		alias,
		createName,
		imageRef,
		imageInfo.ID,
		a.Run,
		flags.tty,
		flags.stdinOpen,
		flags.autoRemove,
	)
	if err != nil {
		return nil, err
	}

	if result, reused, reuseErr := s.tryReuseContainer(
		ctx,
		createName,
		fingerprint,
		flags.autoRemove,
		flags.attach,
		flags.tty,
	); reuseErr != nil {
		return nil, reuseErr
	} else if reused {
		return result, nil
	}

	id, createErr := s.createContainer(ctx, createName, a.Run, imageRef, fingerprint, flags)
	if createErr != nil {
		return nil, createErr
	}

	return &RunResult{
		ID:         id,
		AutoRemove: flags.autoRemove,
		Attach:     flags.attach,
		TTY:        flags.tty,
	}, nil
}

func (s *Service) createContainer(
	ctx context.Context,
	name string,
	run config.RunSpec,
	imageRef string,
	fingerprint string,
	flags runFlags,
) (string, error) {
	env := MapToEnv(run.Env)
	userSpec := userSpec(run)

	resources, err := buildResources(run.Resources)
	if err != nil {
		return "", err
	}

	hostCfg, err := buildHostConfig(run, resources, flags.autoRemove)
	if err != nil {
		return "", err
	}

	exposed, bindings, err := ParsePorts(run.Ports)
	if err != nil {
		return "", err
	}
	hostCfg.PortBindings = bindings

	cfgCtr := &container.Config{
		Image:        imageRef,
		User:         userSpec,
		Env:          env,
		WorkingDir:   run.Workdir,
		Entrypoint:   run.Entrypoint,
		Cmd:          run.Cmd,
		Hostname:     run.Hostname,
		Tty:          flags.tty,
		OpenStdin:    flags.stdinOpen,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: exposed,
		Labels: map[string]string{
			containerFingerprintLabel: fingerprint,
		},
	}

	createOpts := client.ContainerCreateOptions{
		Name:       name,
		Config:     cfgCtr,
		HostConfig: hostCfg,
	}
	if run.Platform != "" {
		platform, parsePlatformErr := ParsePlatform(run.Platform)
		if parsePlatformErr != nil {
			return "", parsePlatformErr
		}
		createOpts.Platform = platform
	}

	created, err := s.cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return "", err
	}

	if _, startErr := s.cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); startErr != nil {
		return "", startErr
	}

	return created.ID, nil
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
		oldState, setRawErr := term.MakeRaw(int(opts.Stdin.Fd()))
		if setRawErr == nil {
			defer func() {
				_ = term.Restore(int(opts.Stdin.Fd()), oldState)
			}()
		}
	}

	if opts.TTY {
		resize := func() {
			w, h, sizeErr := term.GetSize(int(opts.Stdin.Fd()))
			if sizeErr != nil || w < 0 || h < 0 {
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
	case waitErr := <-wait.Error:
		if waitErr != nil {
			return waitErr
		}
	case <-wait.Result:
	}

	if !opts.AutoRemove {
		return nil
	}
	_, _ = s.cli.ContainerRemove(context.Background(), opts.ID, client.ContainerRemoveOptions{Force: true})
	return nil
}

func (s *Service) tryReuseContainer(
	ctx context.Context,
	name, fingerprint string,
	autoRemove, attach, tty bool,
) (*RunResult, bool, error) {
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
		if _, startErr := s.cli.ContainerStart(ctx, ctr.Container.ID, client.ContainerStartOptions{}); startErr != nil {
			return nil, false, startErr
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
		Username   string                `json:"username"`
		UID        int                   `json:"uid"`
		GID        int                   `json:"gid"`
		TTY        bool                  `json:"tty"`
		StdinOpen  bool                  `json:"stdin_open"`
		AutoRemove bool                  `json:"auto_remove"`
		Hostname   string                `json:"hostname"`
		Workdir    string                `json:"workdir"`
		Env        []envKV               `json:"env"`
		Entrypoint []string              `json:"entrypoint"`
		Cmd        []string              `json:"cmd"`
		Network    string                `json:"network"`
		Ports      []string              `json:"ports"`
		ExtraHosts []string              `json:"extra_hosts"`
		Mounts     []config.MountSpec    `json:"mounts"`
		Resources  *config.ResourcesSpec `json:"resources,omitempty"`
		Privileged bool                  `json:"privileged"`
		Restart    string                `json:"restart"`
		Platform   string                `json:"platform"`
	} `json:"run"`
}

func RunFingerprint(
	alias, name, imageRef, imageID string,
	run config.RunSpec,
	tty, stdinOpen, autoRemove bool,
) (string, error) {
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

	ports := NormalizeTrimmedSlice(run.Ports)
	extraHosts := NormalizeTrimmedSlice(run.ExtraHosts)

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

func NormalizeTrimmedSlice(in []string) []string {
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

func MapToEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func BoolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func defaultContainerName(alias, configured string) string {
	if configured != "" {
		return configured
	}
	return fmt.Sprintf("cradle-%s", alias)
}

func userSpec(run config.RunSpec) string {
	if run.UID > 0 && run.GID > 0 {
		return fmt.Sprintf("%d:%d", run.UID, run.GID)
	}
	return ""
}

func buildResources(spec *config.ResourcesSpec) (container.Resources, error) {
	resources := container.Resources{}
	if spec == nil {
		return resources, nil
	}
	if spec.CPUs > 0 {
		resources.NanoCPUs = int64(spec.CPUs * nanoCPUsPerCPU)
	}
	if spec.Memory != "" {
		mem, err := units.RAMInBytes(spec.Memory)
		if err != nil {
			return resources, fmt.Errorf("invalid run.resources.memory: %w", err)
		}
		resources.Memory = mem
	}
	return resources, nil
}

func buildHostConfig(
	run config.RunSpec,
	resources container.Resources,
	autoRemove bool,
) (*container.HostConfig, error) {
	hostCfg := &container.HostConfig{
		AutoRemove:  autoRemove,
		Privileged:  run.Privileged,
		NetworkMode: container.NetworkMode(run.Network),
		ExtraHosts:  run.ExtraHosts,
		Mounts:      ToDockerMounts(run.Mounts),
		Resources:   resources,
	}

	if run.Restart != "" {
		hostCfg.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(run.Restart)}
	}

	if run.Resources != nil && run.Resources.ShmSize != "" {
		shm, err := units.RAMInBytes(run.Resources.ShmSize)
		if err != nil {
			return nil, fmt.Errorf("invalid run.resources.shm_size: %w", err)
		}
		hostCfg.ShmSize = shm
	}

	return hostCfg, nil
}

func ToDockerMounts(ms []config.MountSpec) []mount.Mount {
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

func ParsePorts(specs []string) (mobynet.PortSet, mobynet.PortMap, error) {
	if len(specs) == 0 {
		return nil, nil, nil
	}

	exposed := mobynet.PortSet{}
	bindings := mobynet.PortMap{}

	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}

		port, binding, err := parsePortMapping(spec)
		if err != nil {
			return nil, nil, err
		}

		exposed[port] = struct{}{}
		if binding != nil {
			bindings[port] = append(bindings[port], *binding)
		}
	}

	return exposed, bindings, nil
}

func parsePortMapping(spec string) (mobynet.Port, *mobynet.PortBinding, error) {
	hostIPStr, hostPortStr, containerPortStr, err := splitPortSpec(spec)
	if err != nil {
		return mobynet.Port{}, nil, err
	}

	port, parseErr := mobynet.ParsePort(containerPortStr)
	if parseErr != nil {
		return mobynet.Port{}, nil, fmt.Errorf("invalid container port %q in %q: %w", containerPortStr, spec, parseErr)
	}

	binding, hasBinding, bindingErr := buildPortBinding(hostIPStr, hostPortStr, spec)
	if bindingErr != nil {
		return mobynet.Port{}, nil, bindingErr
	}

	if !hasBinding {
		return port, nil, nil
	}

	return port, &binding, nil
}

func splitPortSpec(spec string) (string, string, string, error) {
	if strings.HasPrefix(spec, "[") {
		return splitIPv6Port(spec)
	}
	return splitIPv4Port(spec)
}

func splitIPv6Port(spec string) (string, string, string, error) {
	end := strings.Index(spec, "]")
	if end == -1 {
		return "", "", "", fmt.Errorf("invalid port mapping %q", spec)
	}
	hostIPStr := spec[1:end]
	if end+hostPortParts >= len(spec) || spec[end+1] != ':' {
		return "", "", "", fmt.Errorf("invalid port mapping %q", spec)
	}
	rest := spec[end+hostPortParts:]
	parts := strings.Split(rest, ":")
	if len(parts) != ipv6PortPartCount {
		return "", "", "", fmt.Errorf("invalid port mapping %q", spec)
	}
	return hostIPStr, parts[0], parts[1], nil
}

func splitIPv4Port(spec string) (string, string, string, error) {
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case singlePortPart:
		return "", "", parts[0], nil
	case hostPortParts:
		return "", parts[0], parts[1], nil
	case hostWithIPParts:
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid port mapping %q", spec)
	}
}

func buildPortBinding(hostIPStr, hostPortStr, spec string) (mobynet.PortBinding, bool, error) {
	if hostPortStr == "" {
		return mobynet.PortBinding{}, false, nil
	}

	binding := mobynet.PortBinding{HostPort: hostPortStr}
	if hostIPStr == "" {
		return binding, true, nil
	}

	parsedIP, err := netip.ParseAddr(hostIPStr)
	if err != nil {
		return mobynet.PortBinding{}, false, fmt.Errorf("invalid host ip %q in %q: %w", hostIPStr, spec, err)
	}
	binding.HostIP = parsedIP
	return binding, true, nil
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

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

	tmpfsSpecParts       = 2
	deviceSpecParts      = 3
	deviceSpecHostOnly   = 1
	deviceSpecHostTarget = 2
	deviceSpecHostPerm   = 3
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
	createOpts, err := BuildContainerCreateOptions(
		name,
		run,
		imageRef,
		fingerprint,
		flags.tty,
		flags.stdinOpen,
		flags.autoRemove,
	)
	if err != nil {
		return "", err
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

func BuildContainerCreateOptions(
	name string,
	run config.RunSpec,
	imageRef string,
	fingerprint string,
	tty bool,
	stdinOpen bool,
	autoRemove bool,
) (client.ContainerCreateOptions, error) {
	env := MapToEnv(run.Env)
	userSpec := userSpec(run)

	resources, err := buildResources(run.Resources, run.Ulimits, run.Devices)
	if err != nil {
		return client.ContainerCreateOptions{}, err
	}

	hostCfg, err := buildHostConfig(run, resources, autoRemove)
	if err != nil {
		return client.ContainerCreateOptions{}, err
	}

	exposed, bindings, err := ParsePorts(run.Ports)
	if err != nil {
		return client.ContainerCreateOptions{}, err
	}
	hostCfg.PortBindings = bindings

	if exposed, err = addExposedPorts(exposed, run.Expose); err != nil {
		return client.ContainerCreateOptions{}, err
	}

	cfgCtr := &container.Config{
		Image:        imageRef,
		User:         userSpec,
		Env:          env,
		WorkingDir:   run.WorkDir,
		Entrypoint:   run.Entrypoint,
		Cmd:          run.Cmd,
		Hostname:     run.Hostname,
		Domainname:   run.DomainName,
		Tty:          tty,
		OpenStdin:    stdinOpen,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: exposed,
		Labels:       mergeLabels(run.Labels, fingerprint),
		StopSignal:   run.StopSignal,
	}
	stopTimeout, hasStopTimeout, stopTimeoutErr := parseStopTimeout(run.StopGracePeriod)
	if stopTimeoutErr != nil {
		return client.ContainerCreateOptions{}, stopTimeoutErr
	}
	if hasStopTimeout {
		cfgCtr.StopTimeout = &stopTimeout
	}

	healthcheck, hasHealthcheck, healthErr := buildHealthcheck(run.HealthCheck)
	if healthErr != nil {
		return client.ContainerCreateOptions{}, healthErr
	}
	if hasHealthcheck {
		cfgCtr.Healthcheck = healthcheck
	}

	createOpts := client.ContainerCreateOptions{
		Name:             name,
		Config:           cfgCtr,
		HostConfig:       hostCfg,
		NetworkingConfig: buildNetworkingConfig(run.Networks),
	}
	if run.Platform != "" {
		platform, parsePlatformErr := ParsePlatform(run.Platform)
		if parsePlatformErr != nil {
			return client.ContainerCreateOptions{}, parsePlatformErr
		}
		createOpts.Platform = platform
	}

	return createOpts, nil
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

type networkFingerprint struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type runFingerprintSpec struct {
	Alias    string            `json:"alias"`
	Name     string            `json:"name"`
	ImageRef string            `json:"image_ref"`
	ImageID  string            `json:"image_id"`
	Run      runFingerprintRun `json:"run"`
}

type runFingerprintRun struct {
	UID             int                     `json:"uid"`
	GID             int                     `json:"gid"`
	User            string                  `json:"user"`
	TTY             bool                    `json:"tty"`
	StdinOpen       bool                    `json:"stdin_open"`
	AutoRemove      bool                    `json:"auto_remove"`
	Hostname        string                  `json:"hostname"`
	DomainName      string                  `json:"domain_name"`
	WorkDir         string                  `json:"work_dir"`
	Env             []envKV                 `json:"env"`
	Entrypoint      []string                `json:"entrypoint"`
	Cmd             []string                `json:"cmd"`
	NetworkMode     string                  `json:"network_mode"`
	Networks        []networkFingerprint    `json:"networks"`
	Ports           []string                `json:"ports"`
	Expose          []string                `json:"expose"`
	ExtraHosts      []string                `json:"extra_hosts"`
	DNS             []string                `json:"dns"`
	DNSSearch       []string                `json:"dns_search"`
	DNSOptions      []string                `json:"dns_opt"`
	IPC             string                  `json:"ipc"`
	PID             string                  `json:"pid"`
	UTS             string                  `json:"uts"`
	Runtime         string                  `json:"runtime"`
	Volumes         []config.MountSpec      `json:"volumes"`
	Resources       *config.ResourcesSpec   `json:"resources,omitempty"`
	Privileged      bool                    `json:"privileged"`
	ReadOnly        bool                    `json:"read_only"`
	CapAdd          []string                `json:"cap_add"`
	CapDrop         []string                `json:"cap_drop"`
	SecurityOpt     []string                `json:"security_opt"`
	Sysctls         []envKV                 `json:"sysctls"`
	Ulimits         []config.UlimitSpec     `json:"ulimits"`
	Tmpfs           []string                `json:"tmpfs"`
	Devices         []string                `json:"devices"`
	GroupAdd        []string                `json:"group_add"`
	Labels          []envKV                 `json:"labels"`
	StopSignal      string                  `json:"stop_signal"`
	StopGracePeriod string                  `json:"stop_grace_period"`
	Healthcheck     *config.HealthCheckSpec `json:"healthcheck,omitempty"`
	Logging         *config.LogConfigSpec   `json:"logging,omitempty"`
	Restart         string                  `json:"restart"`
	Platform        string                  `json:"platform"`
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
		Run:      buildRunFingerprintRun(run, tty, stdinOpen, autoRemove),
	}

	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func buildRunFingerprintRun(run config.RunSpec, tty, stdinOpen, autoRemove bool) runFingerprintRun {
	return runFingerprintRun{
		UID:             run.UID,
		GID:             run.GID,
		User:            run.User,
		TTY:             tty,
		StdinOpen:       stdinOpen,
		AutoRemove:      autoRemove,
		Hostname:        run.Hostname,
		DomainName:      run.DomainName,
		WorkDir:         run.WorkDir,
		Env:             mapToSortedKVs(run.Env),
		Entrypoint:      run.Entrypoint,
		Cmd:             run.Cmd,
		NetworkMode:     run.NetworkMode,
		Networks:        normalizeNetworks(run.Networks),
		Ports:           NormalizeTrimmedSlice(run.Ports),
		Expose:          NormalizeTrimmedSlice(run.Expose),
		ExtraHosts:      NormalizeTrimmedSlice(run.ExtraHosts),
		DNS:             NormalizeTrimmedSlice(run.DNS),
		DNSSearch:       NormalizeTrimmedSlice(run.DNSSearch),
		DNSOptions:      NormalizeTrimmedSlice(run.DNSOptions),
		IPC:             run.IPC,
		PID:             run.PID,
		UTS:             run.UTS,
		Runtime:         run.Runtime,
		Volumes:         run.Volumes,
		Resources:       run.Resources,
		Privileged:      run.Privileged,
		ReadOnly:        run.ReadOnly,
		CapAdd:          NormalizeTrimmedSlice(run.CapAdd),
		CapDrop:         NormalizeTrimmedSlice(run.CapDrop),
		SecurityOpt:     NormalizeTrimmedSlice(run.SecurityOpt),
		Sysctls:         mapToSortedKVs(run.Sysctls),
		Ulimits:         run.Ulimits,
		Tmpfs:           NormalizeTrimmedSlice(run.Tmpfs),
		Devices:         NormalizeTrimmedSlice(run.Devices),
		GroupAdd:        NormalizeTrimmedSlice(run.GroupAdd),
		Labels:          mapToSortedKVs(run.Labels),
		StopSignal:      run.StopSignal,
		StopGracePeriod: run.StopGracePeriod,
		Healthcheck:     run.HealthCheck,
		Logging:         run.Logging,
		Restart:         run.Restart,
		Platform:        run.Platform,
	}
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

func mapToSortedKVs(values map[string]string) []envKV {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]envKV, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, envKV{Key: key, Value: values[key]})
	}
	return entries
}

func normalizeNetworks(networks map[string]config.NetworkSpec) []networkFingerprint {
	if len(networks) == 0 {
		return nil
	}
	keys := make([]string, 0, len(networks))
	for key := range networks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]networkFingerprint, 0, len(keys))
	for _, key := range keys {
		aliases := NormalizeTrimmedSlice(networks[key].Aliases)
		entries = append(entries, networkFingerprint{Name: key, Aliases: aliases})
	}
	return entries
}

func mergeLabels(labels map[string]string, fingerprint string) map[string]string {
	merged := map[string]string{containerFingerprintLabel: fingerprint}
	maps.Copy(merged, labels)
	return merged
}

func parseStopTimeout(value string) (int, bool, error) {
	if strings.TrimSpace(value) == "" {
		return 0, false, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, false, fmt.Errorf("invalid run.stop_grace_period: %w", err)
	}
	if duration < 0 {
		return 0, false, errors.New("invalid run.stop_grace_period: must be >= 0")
	}
	seconds := int(duration.Seconds())
	return seconds, true, nil
}

func buildHealthcheck(spec *config.HealthCheckSpec) (*container.HealthConfig, bool, error) {
	if spec == nil {
		return nil, false, nil
	}
	if spec.Disable {
		return &container.HealthConfig{Test: []string{"NONE"}}, true, nil
	}
	hc := &container.HealthConfig{Test: spec.Test}
	interval, err := parseDuration(spec.Interval, "run.healthcheck.interval")
	if err != nil {
		return nil, false, err
	}
	hc.Interval = interval
	timeout, err := parseDuration(spec.Timeout, "run.healthcheck.timeout")
	if err != nil {
		return nil, false, err
	}
	hc.Timeout = timeout
	startPeriod, err := parseDuration(spec.StartPeriod, "run.healthcheck.start_period")
	if err != nil {
		return nil, false, err
	}
	hc.StartPeriod = startPeriod
	startInterval, err := parseDuration(spec.StartInterval, "run.healthcheck.start_interval")
	if err != nil {
		return nil, false, err
	}
	hc.StartInterval = startInterval
	if spec.Retries != nil {
		hc.Retries = *spec.Retries
	}
	return hc, true, nil
}

func parseDuration(value string, field string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("invalid %s: must be >= 0", field)
	}
	return duration, nil
}

func addExposedPorts(exposed mobynet.PortSet, extra []string) (mobynet.PortSet, error) {
	if len(extra) == 0 {
		return exposed, nil
	}
	if exposed == nil {
		exposed = mobynet.PortSet{}
	}
	for _, raw := range extra {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		port, err := mobynet.ParsePort(spec)
		if err != nil {
			return nil, fmt.Errorf("invalid expose port %q: %w", raw, err)
		}
		exposed[port] = struct{}{}
	}
	return exposed, nil
}

func parseDNS(specs []string) ([]netip.Addr, error) {
	addrs := make([]netip.Addr, 0, len(specs))
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		addr, err := netip.ParseAddr(spec)
		if err != nil {
			return nil, fmt.Errorf("invalid dns entry %q: %w", raw, err)
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

func parseTmpfs(specs []string) (map[string]string, error) {
	if len(specs) == 0 {
		return map[string]string{}, nil
	}
	entries := map[string]string{}
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		parts := strings.SplitN(spec, ":", tmpfsSpecParts)
		path := strings.TrimSpace(parts[0])
		if path == "" {
			return nil, fmt.Errorf("invalid tmpfs entry %q", raw)
		}
		options := ""
		if len(parts) == tmpfsSpecParts {
			options = strings.TrimSpace(parts[1])
		}
		entries[path] = options
	}
	return entries, nil
}

func parseDeviceSpecs(specs []string) ([]container.DeviceMapping, error) {
	devices := make([]container.DeviceMapping, 0, len(specs))
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		parts := strings.SplitN(spec, ":", deviceSpecParts)
		mapping := container.DeviceMapping{}
		switch len(parts) {
		case deviceSpecHostOnly:
			mapping.PathOnHost = parts[0]
			mapping.PathInContainer = parts[0]
			mapping.CgroupPermissions = "rwm"
		case deviceSpecHostTarget:
			mapping.PathOnHost = parts[0]
			mapping.PathInContainer = parts[1]
			mapping.CgroupPermissions = "rwm"
		case deviceSpecHostPerm:
			mapping.PathOnHost = parts[0]
			mapping.PathInContainer = parts[1]
			mapping.CgroupPermissions = parts[2]
		default:
			return nil, fmt.Errorf("invalid device entry %q", raw)
		}
		if mapping.PathOnHost == "" || mapping.PathInContainer == "" {
			return nil, fmt.Errorf("invalid device entry %q", raw)
		}
		devices = append(devices, mapping)
	}
	return devices, nil
}

func buildNetworkingConfig(networks map[string]config.NetworkSpec) *mobynet.NetworkingConfig {
	if len(networks) == 0 {
		return nil
	}
	endpoints := map[string]*mobynet.EndpointSettings{}
	for name, spec := range networks {
		aliases := NormalizeTrimmedSlice(spec.Aliases)
		endpoints[name] = &mobynet.EndpointSettings{Aliases: aliases}
	}
	return &mobynet.NetworkingConfig{EndpointsConfig: endpoints}
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
	if run.User != "" {
		return run.User
	}
	if run.UID > 0 && run.GID > 0 {
		return fmt.Sprintf("%d:%d", run.UID, run.GID)
	}
	return ""
}

func buildResources(
	spec *config.ResourcesSpec,
	ulimits []config.UlimitSpec,
	devices []string,
) (container.Resources, error) {
	resources := container.Resources{}
	if err := applyResourceSpec(&resources, spec); err != nil {
		return resources, err
	}
	if len(ulimits) > 0 {
		resources.Ulimits = buildUlimits(ulimits)
	}
	if len(devices) > 0 {
		parsed, err := parseDeviceSpecs(devices)
		if err != nil {
			return resources, err
		}
		resources.Devices = parsed
	}
	return resources, nil
}

func applyResourceSpec(resources *container.Resources, spec *config.ResourcesSpec) error {
	if spec == nil {
		return nil
	}
	if spec.CPUs > 0 {
		resources.NanoCPUs = int64(spec.CPUs * nanoCPUsPerCPU)
	}
	resources.CPUShares = spec.CPUShares
	resources.CPUQuota = spec.CPUQuota
	resources.CPUPeriod = spec.CPUPeriod
	resources.CpusetCpus = spec.CPUSetCPUs
	resources.CpusetMems = spec.CPUSetMems
	resources.CgroupParent = spec.CgroupParent
	resources.OomKillDisable = spec.OomKillDisable
	resources.PidsLimit = spec.PidsLimit
	if err := applyMemoryLimit(&resources.Memory, spec.Memory, "run.resources.memory"); err != nil {
		return err
	}
	if err := applyMemoryLimit(
		&resources.MemoryReservation,
		spec.MemoryReservation,
		"run.resources.memory_reservation",
	); err != nil {
		return err
	}
	if err := applyMemoryLimit(&resources.MemorySwap, spec.MemorySwap, "run.resources.memory_swap"); err != nil {
		return err
	}
	return nil
}

func applyMemoryLimit(target *int64, value string, field string) error {
	if value == "" {
		return nil
	}
	mem, err := units.RAMInBytes(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", field, err)
	}
	*target = mem
	return nil
}

func buildHostConfig(
	run config.RunSpec,
	resources container.Resources,
	autoRemove bool,
) (*container.HostConfig, error) {
	hostCfg := &container.HostConfig{
		AutoRemove:     autoRemove,
		Privileged:     run.Privileged,
		NetworkMode:    container.NetworkMode(run.NetworkMode),
		ExtraHosts:     run.ExtraHosts,
		Mounts:         ToDockerMounts(run.Volumes),
		Resources:      resources,
		ReadonlyRootfs: run.ReadOnly,
		CapAdd:         run.CapAdd,
		CapDrop:        run.CapDrop,
		SecurityOpt:    run.SecurityOpt,
		Sysctls:        run.Sysctls,
		GroupAdd:       run.GroupAdd,
		Runtime:        run.Runtime,
	}

	if len(run.DNS) > 0 {
		dns, err := parseDNS(run.DNS)
		if err != nil {
			return nil, err
		}
		hostCfg.DNS = dns
	}
	if len(run.DNSOptions) > 0 {
		hostCfg.DNSOptions = run.DNSOptions
	}
	if len(run.DNSSearch) > 0 {
		hostCfg.DNSSearch = run.DNSSearch
	}

	if run.IPC != "" {
		hostCfg.IpcMode = container.IpcMode(run.IPC)
	}
	if run.PID != "" {
		hostCfg.PidMode = container.PidMode(run.PID)
	}
	if run.UTS != "" {
		hostCfg.UTSMode = container.UTSMode(run.UTS)
	}

	if len(run.Tmpfs) > 0 {
		tmpfs, err := parseTmpfs(run.Tmpfs)
		if err != nil {
			return nil, err
		}
		hostCfg.Tmpfs = tmpfs
	}

	if run.Logging != nil {
		hostCfg.LogConfig = container.LogConfig{
			Type:   run.Logging.Driver,
			Config: run.Logging.Options,
		}
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

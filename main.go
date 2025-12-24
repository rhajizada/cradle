package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	mobynet "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/term"
)

func main() {
	log.SetFlags(0)

	cfgPath, args := popFlag("-c", "--config")
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := client.New(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	cmd := args[0]
	switch cmd {
	case "aliases", "ls":
		cmdAliases(cfg)
	case "build":
		if len(args) < 2 {
			log.Fatal("usage: cradle build <alias|all>")
		}
		target := args[1]
		if target == "all" {
			for name := range cfg.Aliases {
				if err := buildAlias(ctx, cli, cfg, name); err != nil {
					log.Fatalf("build %s: %v", name, err)
				}
			}
			return
		}
		if err := buildAlias(ctx, cli, cfg, target); err != nil {
			log.Fatal(err)
		}
	case "run":
		if len(args) < 2 {
			log.Fatal("usage: cradle run <alias>")
		}
		aliasName := args[1]
		if err := runAliasInteractive(ctx, cancel, cli, cfg, aliasName); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `cradle -c <config.yaml> <command>

Commands:
  aliases|ls                 List aliases
  build <alias|all>          Build (or pull) images
  run <alias>                Run alias interactively (pull/build first)

Examples:
  cradle -c cradle.yaml aliases
  cradle -c cradle.yaml build ubuntu-dev
  cradle -c cradle.yaml run ubuntu-dev
`)
}

func cmdAliases(cfg *config.Config) {
	// keep it simple (no table formatting yet)
	for name, a := range cfg.Aliases {
		img := ""
		switch {
		case a.Image.Pull != nil:
			img = "pull: " + a.Image.Pull.Ref
		case a.Image.Build != nil:
			img = "build: " + a.Image.Build.Cwd
		default:
			img = "image: <invalid>"
		}
		fmt.Printf("%-20s %s\n", name, img)
	}
}

// ---------- Build / Pull ----------

func ensureImage(ctx context.Context, cli *client.Client, cfg *config.Config, aliasName string) (string, error) {
	a, ok := cfg.Aliases[aliasName]
	if !ok {
		return "", fmt.Errorf("unknown alias %q", aliasName)
	}

	if a.Image.Pull != nil {
		ref := normalizeImageRef(a.Image.Pull.Ref)
		if err := pullImage(ctx, cli, ref); err != nil {
			return "", err
		}
		return ref, nil
	}

	// Build
	tag := fmt.Sprintf("cradle/%s:latest", aliasName)
	if err := buildImage(ctx, cli, cfg, aliasName, tag); err != nil {
		return "", err
	}
	return tag, nil
}

func buildAlias(ctx context.Context, cli *client.Client, cfg *config.Config, aliasName string) error {
	_, err := ensureImage(ctx, cli, cfg, aliasName)
	return err
}

func pullImage(ctx context.Context, cli *client.Client, ref string) error {
	fmt.Println("pull:", ref)
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()

	// ImagePullResponse implements io.ReadCloser, so io.Copy works. :contentReference[oaicite:1]{index=1}
	_, _ = io.Copy(os.Stdout, resp)
	return nil
}

func buildImage(ctx context.Context, cli *client.Client, cfg *config.Config, aliasName, tag string) error {
	a := cfg.Aliases[aliasName]
	b := a.Image.Build
	if b == nil {
		return fmt.Errorf("alias %q is not a build alias", aliasName)
	}

	// Resolve context root relative to config file
	contextDir := resolvePath(cfg.BaseDir, b.Cwd)

	// dockerfile path is relative inside context root
	dockerfile := b.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	tar, err := tarDir(contextDir)
	if err != nil {
		return err
	}
	defer tar.Close()

	buildArgs := map[string]*string{}
	for k, v := range b.Args {
		vv := v
		buildArgs[k] = &vv
	}

	platforms, err := parsePlatformList(b.Platforms)
	if err != nil {
		return err
	}

	opts := client.ImageBuildOptions{
		Tags:        []string{tag},
		Dockerfile:  dockerfile,
		Remove:      true,
		ForceRemove: true,

		BuildArgs:  buildArgs,
		Target:     b.Target,
		Labels:     b.Labels,
		NoCache:    b.NoCache,
		PullParent: b.PullParent,
		CacheFrom:  b.CacheFrom,
		Platforms:  platforms,

		NetworkMode: b.Network,
		ExtraHosts:  b.ExtraHosts,

		// Default to BuildKit (your requirement).
		Version: build.BuilderBuildKit,
	}

	fmt.Println("build:", tag, "from", contextDir)
	res, err := cli.ImageBuild(ctx, tar, opts) // ImageBuild takes context io.Reader and returns result with Body. :contentReference[oaicite:2]{index=2}
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, _ = io.Copy(os.Stdout, res.Body)
	return nil
}

// ---------- Run (interactive shell attach) ----------

func runAliasInteractive(ctx context.Context, cancel context.CancelFunc, cli *client.Client, cfg *config.Config, aliasName string) error {
	a, ok := cfg.Aliases[aliasName]
	if !ok {
		return fmt.Errorf("unknown alias %q", aliasName)
	}

	imageRef, err := ensureImage(ctx, cli, cfg, aliasName)
	if err != nil {
		return err
	}

	createName := a.Run.Name
	if createName == "" {
		createName = fmt.Sprintf("cradle-%s", aliasName)
	}

	tty := boolDefault(a.Run.TTY, true)
	stdinOpen := boolDefault(a.Run.StdinOpen, true)
	autoRemove := boolDefault(a.Run.AutoRemove, true)
	attach := boolDefault(a.Run.Attach, true)

	env := []string{}
	for k, v := range a.Run.Env {
		env = append(env, k+"="+v)
	}

	// For your “uid/gid/username only” model:
	// Use numeric user mapping; this works even if no /etc/passwd entry exists.
	userSpec := fmt.Sprintf("%d:%d", a.Run.UID, a.Run.GID)

	resources := container.Resources{}
	if a.Run.Resources.CPUs > 0 {
		resources.NanoCPUs = int64(a.Run.Resources.CPUs * 1e9)
	}
	if a.Run.Resources.Memory != "" {
		mem, err := units.RAMInBytes(a.Run.Resources.Memory)
		if err != nil {
			return fmt.Errorf("invalid run.resources.memory: %w", err)
		}
		resources.Memory = mem
	}

	hostCfg := &container.HostConfig{
		AutoRemove:  autoRemove,
		Privileged:  a.Run.Privileged,
		NetworkMode: container.NetworkMode(a.Run.Network),
		ExtraHosts:  a.Run.ExtraHosts,
		Mounts:      toDockerMounts(cfg.BaseDir, a.Run.Mounts),
		Resources:   resources,
	}
	if a.Run.Restart != "" {
		hostCfg.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(a.Run.Restart)}
	}
	if a.Run.Resources.ShmSize != "" {
		shm, err := units.RAMInBytes(a.Run.Resources.ShmSize)
		if err != nil {
			return fmt.Errorf("invalid run.resources.shm_size: %w", err)
		}
		hostCfg.ShmSize = shm
	}

	exposed, bindings, err := parsePorts(a.Run.Ports)
	if err != nil {
		return err
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
			return err
		}
		createOpts.Platform = platform
	}

	created, err := cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return err
	}
	id := created.ID
	fmt.Println("container:", id)

	cleanup := func() {
		_, _ = cli.ContainerRemove(context.Background(), id, client.ContainerRemoveOptions{Force: true})
	}
	if !autoRemove {
		defer cleanup()
	}

	if _, err := cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		return err
	}

	if !attach {
		return nil
	}

	attached, err := cli.ContainerAttach(ctx, id, client.ContainerAttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return err
	}
	defer attached.Close()

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
		}
	}

	// signals: cancel + best-effort cleanup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		if !autoRemove {
			cleanup()
		}
		os.Exit(130)
	}()

	// TTY resize
	resize := func() {
		w, h, err := term.GetSize(fd)
		if err != nil {
			return
		}
		_, _ = cli.ContainerResize(ctx, id, client.ContainerResizeOptions{
			Width:  uint(w),
			Height: uint(h),
		})
	}
	resize()
	go func() {
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		for range winch {
			resize()
		}
	}()

	go func() { _, _ = io.Copy(attached.Conn, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, attached.Reader) // TTY mode: no stdcopy demux needed

	// Wait
	wait := cli.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case err := <-wait.Error:
		return err
	case <-wait.Result:
		return nil
	}
}

// ---------- small helpers ----------

func boolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func resolvePath(baseDir, p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

func normalizeImageRef(ref string) string {
	// keep it simple for now; you can add docker.io/library behavior later
	return strings.TrimSpace(ref)
}

func toDockerMounts(baseDir string, ms []config.MountSpec) []mount.Mount {
	out := make([]mount.Mount, 0, len(ms))
	for _, m := range ms {
		switch m.Type {
		case "bind":
			src := resolvePath(baseDir, m.Source)
			out = append(out, mount.Mount{
				Type:     mount.TypeBind,
				Source:   src,
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
		default:
			// config.Validate() should prevent this
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

		// Supports:
		//   "80"                    (expose only)
		//   "8080:80"               (bind hostPort -> containerPort)
		//   "127.0.0.1:8080:80"     (bind hostIP:hostPort -> containerPort)
		//   "[::1]:8080:80"         (IPv6 host bind; bracketed)
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

		// containerPortStr may be "80" or "80/tcp" or "53/udp"
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
				HostIP:   hostIP,      // netip.Addr (zero value = “unspecified”)
				HostPort: hostPortStr, // string
			})
		}
	}

	return exposed, bindings, nil
}

func popFlag(short, long string) (string, []string) {
	args := os.Args[1:]
	out := []string{}
	var val string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == short || a == long {
			if i+1 >= len(args) {
				log.Fatalf("missing value for %s", a)
			}
			val = args[i+1]
			i++
			continue
		}
		out = append(out, a)
	}
	return val, out
}

func defaultConfigPath() string {
	if xdg, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdg != "" {
		return filepath.Join(xdg, "cradle", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".cradle.yaml"
	}
	return filepath.Join(home, ".config", "cradle", "config.yaml")
}

// tarDir is intentionally isolated: later you can switch to excludes, gitignore, etc.
func tarDir(dir string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.Close()
		}()

		walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			rel = filepath.ToSlash(rel)
			info, err := d.Info()
			if err != nil {
				return err
			}

			var link string
			if info.Mode()&os.ModeSymlink != 0 {
				link, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}

			hdr, err := tar.FileInfoHeader(info, link)
			if err != nil {
				return err
			}
			hdr.Name = rel
			if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				if _, err := io.Copy(tw, f); err != nil {
					_ = f.Close()
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			}
			return nil
		})
		if walkErr != nil {
			_ = pw.CloseWithError(walkErr)
		}
	}()

	return pr, nil
}

func parsePlatform(s string) (*ocispec.Platform, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid platform %q (expected os/arch[/variant])", s)
	}
	p := &ocispec.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

func parsePlatformList(specs []string) ([]ocispec.Platform, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	platforms := make([]ocispec.Platform, 0, len(specs))
	for _, s := range specs {
		p, err := parsePlatform(s)
		if err != nil {
			return nil, err
		}
		platforms = append(platforms, *p)
	}
	return platforms, nil
}

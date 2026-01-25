package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/session"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/term"
)

const (
	scannerBufferSize   = 64 * 1024
	scannerMaxTokenSize = 1024 * 1024
	minLayerIDLength    = 12
)

func pullImage(ctx context.Context, cli *client.Client, ref string, out io.Writer) (err error) {
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	return renderDockerJSON(out, resp)
}

func buildImage(ctx context.Context, cli *client.Client, b *config.BuildSpec, tag string, out io.Writer) error {
	if b == nil {
		return errors.New("missing build spec")
	}

	contextDir := b.Cwd
	opts, err := BuildOptionsFromSpec(b, tag)
	if err != nil {
		return err
	}

	if buildErr := runImageBuild(ctx, cli, contextDir, opts.Dockerfile, opts, out); buildErr != nil {
		if strings.Contains(buildErr.Error(), "no active sessions") {
			opts.Version = build.BuilderV1
			opts.Platforms = nil
			return runImageBuild(ctx, cli, contextDir, opts.Dockerfile, opts, out)
		}
		return buildErr
	}

	return nil
}

func BuildOptionsFromSpec(b *config.BuildSpec, tag string) (client.ImageBuildOptions, error) {
	if b == nil {
		return client.ImageBuildOptions{}, errors.New("missing build spec")
	}

	dockerfile := b.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	buildArgs := map[string]*string{}
	for k, v := range b.Args {
		vv := v
		buildArgs[k] = &vv
	}

	tags := []string{tag}
	if len(b.Tags) > 0 {
		tags = append(tags, b.Tags...)
	}

	remove := true
	if b.Remove != nil {
		remove = *b.Remove
	}
	forceRemove := true
	if b.ForceRemove != nil {
		forceRemove = *b.ForceRemove
	}

	platforms, err := ParsePlatformList(b.Platforms)
	if err != nil {
		return client.ImageBuildOptions{}, err
	}

	return client.ImageBuildOptions{
		Tags:           tags,
		Dockerfile:     dockerfile,
		SuppressOutput: b.SuppressOutput,
		RemoteContext:  b.RemoteContext,
		Remove:         remove,
		ForceRemove:    forceRemove,
		Isolation:      container.Isolation(b.Isolation),
		CPUSetCPUs:     b.CPUSetCPUs,
		CPUSetMems:     b.CPUSetMems,
		CPUShares:      b.CPUShares,
		CPUQuota:       b.CPUQuota,
		CPUPeriod:      b.CPUPeriod,
		Memory:         b.Memory,
		MemorySwap:     b.MemorySwap,
		CgroupParent:   b.CgroupParent,
		ShmSize:        b.ShmSize,
		Ulimits:        buildUlimits(b.Ulimits),

		BuildArgs:   buildArgs,
		AuthConfigs: buildAuthConfigs(b.AuthConfigs),
		Target:      b.Target,
		Labels:      b.Labels,
		NoCache:     b.NoCache,
		PullParent:  b.PullParent,
		Squash:      b.Squash,
		CacheFrom:   b.CacheFrom,
		SecurityOpt: b.SecurityOpt,
		Platforms:   platforms,

		NetworkMode: b.Network,
		ExtraHosts:  b.ExtraHosts,
		BuildID:     b.BuildID,
		Outputs:     buildOutputs(b.Outputs),

		Version: build.BuilderBuildKit,
	}, nil
}

func ParsePlatformList(specs []string) ([]ocispec.Platform, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	platforms := make([]ocispec.Platform, 0, len(specs))
	for _, s := range specs {
		p, err := ParsePlatform(s)
		if err != nil {
			return nil, err
		}
		platforms = append(platforms, *p)
	}
	return platforms, nil
}

func runImageBuild(
	ctx context.Context,
	cli *client.Client,
	contextDir, dockerfile string,
	opts client.ImageBuildOptions,
	out io.Writer,
) (err error) {
	var tar io.ReadCloser
	if opts.RemoteContext != "" {
		tar = io.NopCloser(strings.NewReader(""))
	} else {
		tar = TarDir(contextDir)
	}
	defer func() {
		if cerr := tar.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	opts.Dockerfile = dockerfile

	var sess *session.Session
	var sessErrCh chan error
	if opts.Version == build.BuilderBuildKit {
		sess, err = session.NewSession(ctx, "")
		if err != nil {
			return err
		}
		opts.SessionID = sess.ID()
		sessErrCh = make(chan error, 1)
		go func() {
			sessErrCh <- sess.Run(ctx, func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
				return cli.DialHijack(ctx, "/session", proto, meta)
			})
		}()
		defer func() {
			_ = sess.Close()
			if serr := <-sessErrCh; serr != nil && err == nil {
				err = serr
			}
		}()
	}

	res, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := res.Body.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	return renderDockerJSON(out, res.Body)
}

func buildUlimits(specs []config.UlimitSpec) []*container.Ulimit {
	if len(specs) == 0 {
		return nil
	}
	ulimits := make([]*container.Ulimit, 0, len(specs))
	for _, spec := range specs {
		ulimits = append(ulimits, &container.Ulimit{
			Name: spec.Name,
			Soft: spec.Soft,
			Hard: spec.Hard,
		})
	}
	return ulimits
}

func buildAuthConfigs(specs map[string]config.BuildAuthConfig) map[string]registry.AuthConfig {
	if len(specs) == 0 {
		return nil
	}
	authConfigs := make(map[string]registry.AuthConfig, len(specs))
	for host, spec := range specs {
		authConfigs[host] = registry.AuthConfig{
			Username:      spec.Username,
			Password:      spec.Password,
			Auth:          spec.Auth,
			ServerAddress: spec.ServerAddress,
			IdentityToken: spec.IdentityToken,
			RegistryToken: spec.RegistryToken,
		}
	}
	return authConfigs
}

func buildOutputs(specs []config.BuildOutputSpec) []client.ImageBuildOutput {
	if len(specs) == 0 {
		return nil
	}
	outputs := make([]client.ImageBuildOutput, 0, len(specs))
	for _, spec := range specs {
		outputs = append(outputs, client.ImageBuildOutput{
			Type:  spec.Type,
			Attrs: spec.Attrs,
		})
	}
	return outputs
}

type buildMessage struct {
	ID       string          `json:"id,omitempty"`
	Status   string          `json:"status,omitempty"`
	Progress string          `json:"progress,omitempty"`
	Stream   string          `json:"stream,omitempty"`
	Error    string          `json:"error,omitempty"`
	Aux      json.RawMessage `json:"aux,omitempty"`
}

func renderDockerJSON(out io.Writer, in io.Reader) error {
	renderer := newDockerRenderer(out)
	scanner := bufio.NewScanner(in)
	buf := make([]byte, 0, scannerBufferSize)
	scanner.Buffer(buf, scannerMaxTokenSize)

	for scanner.Scan() {
		if err := renderer.process(scanner.Bytes()); err != nil {
			return err
		}
	}

	return scanner.Err()
}

type dockerRenderer struct {
	style     OutStyle
	out       io.Writer
	last      map[string]string
	lastTrace string
}

func newDockerRenderer(out io.Writer) *dockerRenderer {
	return &dockerRenderer{
		style: OutputStyle(out),
		out:   out,
		last:  map[string]string{},
	}
}

func (r *dockerRenderer) process(line []byte) error {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil
	}

	var msg buildMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		_, writeErr := fmt.Fprintln(r.out, trimmed)
		return writeErr
	}

	return r.dispatch(msg)
}

func (r *dockerRenderer) dispatch(msg buildMessage) error {
	switch {
	case msg.Error != "":
		return fmt.Errorf("%s", msg.Error)
	case msg.Stream != "":
		return writeString(r.out, r.style.Prefixed(msg.Stream))
	case msg.ID == "moby.buildkit.trace" && len(msg.Aux) > 0:
		return r.handleTrace(msg.Aux)
	case msg.Status != "":
		return r.handleStatus(msg)
	default:
		return nil
	}
}

func (r *dockerRenderer) handleTrace(raw json.RawMessage) error {
	lines, ok := DecodeBuildkitTrace(raw)
	if !ok {
		return nil
	}

	for _, line := range lines {
		if line == "" || line == r.lastTrace {
			continue
		}
		r.lastTrace = line
		if err := writeString(r.out, r.style.Line("🏗️", colorCyan, line)); err != nil {
			return err
		}
	}

	return nil
}

func (r *dockerRenderer) handleStatus(msg buildMessage) error {
	key := msg.ID + "|" + msg.Status + "|" + msg.Progress
	if r.last[msg.ID] == key {
		return nil
	}
	r.last[msg.ID] = key

	label := LabelFor(msg.ID, msg.Status)
	switch {
	case msg.Progress != "":
		return writeString(r.out, r.style.Line("📦", colorYellow, label, msg.Status, msg.Progress))
	case msg.ID != "":
		return writeString(r.out, r.style.Line(StatusEmoji(msg.Status), colorCyan, label, msg.Status))
	default:
		return writeString(r.out, r.style.Line(StatusEmoji(msg.Status), colorGreen, msg.Status))
	}
}

func writeString(out io.Writer, s string) error {
	_, err := io.WriteString(out, s)
	return err
}

type OutStyle struct {
	Color bool
}

func OutputStyle(out io.Writer) OutStyle {
	f, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return OutStyle{}
	}
	if v, found := os.LookupEnv("NO_COLOR"); found && v != "" {
		return OutStyle{}
	}
	return OutStyle{Color: term.IsTerminal(int(f.Fd()))}
}

func (s OutStyle) Prefixed(text string) string {
	if !s.Color {
		return text
	}
	return colorDim + text + colorReset
}

func (s OutStyle) Line(emoji, color string, parts ...string) string {
	text := strings.Join(FilterEmpty(parts), " ")
	if !s.Color {
		return fmt.Sprintf("%s %s\n", emoji, text)
	}
	return fmt.Sprintf("%s %s%s%s\n", emoji, color, text, colorReset)
}

func FilterEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func DecodeBuildkitTrace(raw json.RawMessage) ([]string, bool) {
	dt, ok := decodeTrace(raw)
	if !ok {
		return nil, false
	}

	sr, ok := unmarshalStatus(dt)
	if !ok {
		return nil, false
	}

	lines := collectTraceLines(sr)
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

func decodeTrace(raw json.RawMessage) ([]byte, bool) {
	var dt []byte
	if err := json.Unmarshal(raw, &dt); err != nil || len(dt) == 0 {
		return nil, false
	}
	return dt, true
}

func unmarshalStatus(data []byte) (*controlapi.StatusResponse, bool) {
	var sr controlapi.StatusResponse
	if err := proto.Unmarshal(data, &sr); err != nil {
		return nil, false
	}
	return &sr, true
}

func collectTraceLines(sr *controlapi.StatusResponse) []string {
	lines := make([]string, 0)
	lines = append(lines, vertexLines(sr.GetVertexes())...)
	lines = append(lines, logLines(sr.GetLogs())...)
	lines = append(lines, warningLines(sr.GetWarnings())...)
	return lines
}

func vertexLines(vertexes []*controlapi.Vertex) []string {
	lines := make([]string, 0, len(vertexes))
	for _, vtx := range vertexes {
		name := strings.TrimSpace(vtx.GetName())
		if name == "" {
			continue
		}
		if vtx.GetCached() {
			name += " (cached)"
		}
		if vtx.GetError() != "" {
			name += " (error: " + strings.TrimSpace(vtx.GetError()) + ")"
		}
		lines = append(lines, name)
	}
	return lines
}

func logLines(logs []*controlapi.VertexLog) []string {
	lines := make([]string, 0)
	for _, log := range logs {
		text := string(log.GetMsg())
		for line := range strings.SplitSeq(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func warningLines(warnings []*controlapi.VertexWarning) []string {
	lines := make([]string, 0)
	for _, warn := range warnings {
		short := strings.TrimSpace(string(warn.GetShort()))
		if short != "" {
			lines = append(lines, "warning: "+short)
		}
		for _, detail := range warn.GetDetail() {
			line := strings.TrimSpace(string(detail))
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func StatusEmoji(status string) string {
	switch {
	case strings.HasPrefix(status, "Pulling"):
		return "📥"
	case strings.HasPrefix(status, "Digest"):
		return "🔍"
	case strings.HasPrefix(status, "Status"):
		return "✅"
	default:
		return "🔧"
	}
}

func LabelFor(id, status string) string {
	if id == "" {
		return ""
	}
	if LooksNumeric(id) && strings.HasPrefix(status, "Pulling from") {
		return ""
	}
	if LooksLayerID(id) {
		return "layer " + id + ":"
	}
	return id + ":"
}

func LooksNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func LooksLayerID(s string) bool {
	if len(s) < minLayerIDLength {
		return false
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

const (
	colorReset  = "\x1b[0m"
	colorDim    = "\x1b[2m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorCyan   = "\x1b[36m"
)

package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/term"
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

func buildImage(ctx context.Context, cli *client.Client, b *config.BuildSpec, tag string, out io.Writer) (err error) {
	if b == nil {
		return fmt.Errorf("missing build spec")
	}

	contextDir := b.Cwd
	dockerfile := b.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

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

		Version: build.BuilderBuildKit,
	}

	if err := runImageBuild(ctx, cli, contextDir, dockerfile, opts, out); err != nil {
		if strings.Contains(err.Error(), "no active sessions") {
			opts.Version = build.BuilderV1
			opts.Platforms = nil
			return runImageBuild(ctx, cli, contextDir, dockerfile, opts, out)
		}
		return err
	}

	return nil
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

func runImageBuild(ctx context.Context, cli *client.Client, contextDir, dockerfile string, opts client.ImageBuildOptions, out io.Writer) (err error) {
	tar, err := tarDir(contextDir)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := tar.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	opts.Dockerfile = dockerfile
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

type buildMessage struct {
	ID       string          `json:"id,omitempty"`
	Status   string          `json:"status,omitempty"`
	Progress string          `json:"progress,omitempty"`
	Stream   string          `json:"stream,omitempty"`
	Error    string          `json:"error,omitempty"`
	Aux      json.RawMessage `json:"aux,omitempty"`
}

func renderDockerJSON(out io.Writer, in io.Reader) error {
	style := outputStyle(out)
	scanner := bufio.NewScanner(in)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	last := map[string]string{}
	lastTrace := ""

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var msg buildMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			if _, err := fmt.Fprintln(out, string(line)); err != nil {
				return err
			}
			continue
		}
		if msg.Error != "" {
			return fmt.Errorf("%s", msg.Error)
		}
		if msg.Stream != "" {
			if err := writeString(out, style.prefixed(msg.Stream)); err != nil {
				return err
			}
			continue
		}
		if msg.ID == "moby.buildkit.trace" && len(msg.Aux) > 0 {
			lines, ok := decodeBuildkitTrace(msg.Aux)
			if ok {
				for _, line := range lines {
					if line == "" || line == lastTrace {
						continue
					}
					lastTrace = line
					if err := writeString(out, style.line("🏗️", colorCyan, line)); err != nil {
						return err
					}
				}
			}
			continue
		}
		if msg.Status != "" {
			key := msg.ID + "|" + msg.Status + "|" + msg.Progress
			if last[msg.ID] == key {
				continue
			}
			last[msg.ID] = key

			label := labelFor(msg.ID, msg.Status)
			if msg.Progress != "" {
				if err := writeString(out, style.line("📦", colorYellow, label, msg.Status, msg.Progress)); err != nil {
					return err
				}
			} else if msg.ID != "" {
				if err := writeString(out, style.line(statusEmoji(msg.Status), colorCyan, label, msg.Status)); err != nil {
					return err
				}
			} else {
				if err := writeString(out, style.line(statusEmoji(msg.Status), colorGreen, msg.Status)); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func writeString(out io.Writer, s string) error {
	_, err := io.WriteString(out, s)
	return err
}

type outStyle struct {
	color bool
}

func outputStyle(out io.Writer) outStyle {
	f, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return outStyle{}
	}
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return outStyle{}
	}
	return outStyle{color: term.IsTerminal(int(f.Fd()))}
}

func (s outStyle) prefixed(text string) string {
	if !s.color {
		return text
	}
	return colorDim + text + colorReset
}

func (s outStyle) line(emoji, color string, parts ...string) string {
	text := strings.Join(filterEmpty(parts), " ")
	if !s.color {
		return fmt.Sprintf("%s %s\n", emoji, text)
	}
	return fmt.Sprintf("%s %s%s%s\n", emoji, color, text, colorReset)
}

func filterEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

func decodeBuildkitTrace(raw json.RawMessage) ([]string, bool) {
	var dt []byte
	if err := json.Unmarshal(raw, &dt); err != nil || len(dt) == 0 {
		return nil, false
	}
	var sr controlapi.StatusResponse
	if err := proto.Unmarshal(dt, &sr); err != nil {
		return nil, false
	}
	lines := make([]string, 0)
	for _, vtx := range sr.GetVertexes() {
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
	for _, log := range sr.GetLogs() {
		text := string(log.GetMsg())
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lines = append(lines, line)
		}
	}
	for _, warn := range sr.GetWarnings() {
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
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

func statusEmoji(status string) string {
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

func labelFor(id, status string) string {
	if id == "" {
		return ""
	}
	if looksNumeric(id) && strings.HasPrefix(status, "Pulling from") {
		return ""
	}
	if looksLayerID(id) {
		return "layer " + id + ":"
	}
	return id + ":"
}

func looksNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func looksLayerID(s string) bool {
	if len(s) < 12 {
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

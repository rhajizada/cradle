package service

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/moby/moby/client"
)

type Service struct {
	cfg *config.Config
	cli *client.Client
}

func New(cfg *config.Config) (*Service, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Service{cfg: cfg, cli: cli}, nil
}

func (s *Service) Close() error {
	return s.cli.Close()
}

type ImageKind string

const (
	ImagePull  ImageKind = "pull"
	ImageBuild ImageKind = "build"
)

type AliasInfo struct {
	Name string
	Kind ImageKind
	Ref  string
	Cwd  string
	Tag  string
}

func (a AliasInfo) Description() string {
	switch a.Kind {
	case ImagePull:
		return "pull: " + a.Ref
	case ImageBuild:
		return "build: " + a.Cwd
	default:
		return "image: <invalid>"
	}
}

func (s *Service) ListAliases() []AliasInfo {
	names := make([]string, 0, len(s.cfg.Aliases))
	for name := range s.cfg.Aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]AliasInfo, 0, len(names))
	for _, name := range names {
		info, err := s.AliasInfo(name)
		if err == nil {
			infos = append(infos, info)
		}
	}
	return infos
}

func (s *Service) AliasInfo(name string) (AliasInfo, error) {
	a, ok := s.cfg.Aliases[name]
	if !ok {
		return AliasInfo{}, fmt.Errorf("unknown alias %q", name)
	}
	info := AliasInfo{Name: name}
	if a.Image.Pull != nil {
		info.Kind = ImagePull
		info.Ref = normalizeImageRef(a.Image.Pull.Ref)
		return info, nil
	}
	if a.Image.Build != nil {
		info.Kind = ImageBuild
		info.Cwd = a.Image.Build.Cwd
		info.Tag = imageTag(name)
		return info, nil
	}
	return AliasInfo{}, fmt.Errorf("alias %q has no image", name)
}

func (s *Service) Build(ctx context.Context, alias string, out io.Writer) error {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return fmt.Errorf("unknown alias %q", alias)
	}
	if a.Image.Pull != nil {
		ref := normalizeImageRef(a.Image.Pull.Ref)
		return pullImage(ctx, s.cli, ref, out)
	}
	if a.Image.Build != nil {
		return buildImage(ctx, s.cli, a.Image.Build, imageTag(alias), out)
	}
	return fmt.Errorf("alias %q has no image", alias)
}

func (s *Service) ensureImage(ctx context.Context, alias string, out io.Writer) (string, error) {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return "", fmt.Errorf("unknown alias %q", alias)
	}

	if a.Image.Pull != nil {
		ref := normalizeImageRef(a.Image.Pull.Ref)
		if err := pullImage(ctx, s.cli, ref, out); err != nil {
			return "", err
		}
		return ref, nil
	}

	if a.Image.Build == nil {
		return "", fmt.Errorf("alias %q has no image", alias)
	}

	tag := imageTag(alias)
	if err := buildImage(ctx, s.cli, a.Image.Build, tag, out); err != nil {
		return "", err
	}
	return tag, nil
}

func imageTag(alias string) string {
	return fmt.Sprintf("cradle/%s:latest", alias)
}

func normalizeImageRef(ref string) string {
	return strings.TrimSpace(ref)
}

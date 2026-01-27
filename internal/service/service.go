package service

import (
	"context"
	"errors"
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

func NewWithClient(cfg *config.Config, cli *client.Client) *Service {
	return &Service{cfg: cfg, cli: cli}
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

type ImagePolicyOverrides struct {
	Build *config.ImagePolicy
	Pull  *config.ImagePolicy
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
		info.Ref = NormalizeImageRef(a.Image.Pull.Ref)
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

func (s *Service) Build(ctx context.Context, alias string, out io.Writer, overrides ImagePolicyOverrides) error {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return fmt.Errorf("unknown alias %q", alias)
	}
	if a.Image.Pull != nil {
		ref := NormalizeImageRef(a.Image.Pull.Ref)
		policy := resolveImagePolicy(a.Image.Pull.Policy, overrides.Pull)
		return s.ensurePull(ctx, a.Image.Pull, ref, out, policy)
	}
	if a.Image.Build != nil {
		policy := resolveImagePolicy(a.Image.Build.Policy, overrides.Build)
		return s.ensureBuild(ctx, alias, out, policy)
	}
	return fmt.Errorf("alias %q has no image", alias)
}

func (s *Service) EnsureImage(
	ctx context.Context,
	alias string,
	out io.Writer,
	overrides ImagePolicyOverrides,
) (string, error) {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return "", fmt.Errorf("unknown alias %q", alias)
	}

	if a.Image.Pull != nil {
		ref := NormalizeImageRef(a.Image.Pull.Ref)
		policy := resolveImagePolicy(a.Image.Pull.Policy, overrides.Pull)
		if err := s.ensurePull(ctx, a.Image.Pull, ref, out, policy); err != nil {
			return "", err
		}
		return ref, nil
	}

	if a.Image.Build == nil {
		return "", fmt.Errorf("alias %q has no image", alias)
	}

	tag := imageTag(alias)
	policy := resolveImagePolicy(a.Image.Build.Policy, overrides.Build)
	if err := s.ensureBuild(ctx, alias, out, policy); err != nil {
		return "", err
	}
	return tag, nil
}

func resolveImagePolicy(policy config.ImagePolicy, override *config.ImagePolicy) config.ImagePolicy {
	if override != nil {
		return *override
	}
	if policy == "" {
		return config.ImagePolicyAlways
	}
	return policy
}

func (s *Service) ensurePull(
	ctx context.Context,
	spec *config.PullSpec,
	ref string,
	out io.Writer,
	policy config.ImagePolicy,
) error {
	exists, err := s.imageExists(ctx, ref)
	if err != nil {
		return err
	}
	options, err := PullOptionsFromSpec(spec)
	if err != nil {
		return err
	}
	switch policy {
	case config.ImagePolicyAlways:
		return pullImage(ctx, s.cli, ref, options, out)
	case config.ImagePolicyIfMissing:
		if exists {
			return nil
		}
		return pullImage(ctx, s.cli, ref, options, out)
	case config.ImagePolicyNever:
		if exists {
			return nil
		}
		return fmt.Errorf("image %q not found (pull policy: never)", ref)
	default:
		return fmt.Errorf("unknown pull policy %q", policy)
	}
}

func (s *Service) ensureBuild(
	ctx context.Context,
	alias string,
	out io.Writer,
	policy config.ImagePolicy,
) error {
	if s.cfg == nil {
		return errors.New("missing config")
	}
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return fmt.Errorf("unknown alias %q", alias)
	}
	if a.Image.Build == nil {
		return fmt.Errorf("alias %q has no build image", alias)
	}
	tag := imageTag(alias)
	exists, err := s.imageExists(ctx, tag)
	if err != nil {
		return err
	}
	switch policy {
	case config.ImagePolicyAlways:
		return buildImage(ctx, s.cli, a.Image.Build, tag, out)
	case config.ImagePolicyIfMissing:
		if exists {
			return nil
		}
		return buildImage(ctx, s.cli, a.Image.Build, tag, out)
	case config.ImagePolicyNever:
		if exists {
			return nil
		}
		return fmt.Errorf("image %q not found (build policy: never)", tag)
	default:
		return fmt.Errorf("unknown build policy %q", policy)
	}
}

func imageTag(alias string) string {
	return fmt.Sprintf("cradle/%s:latest", alias)
}

func NormalizeImageRef(ref string) string {
	return strings.TrimSpace(ref)
}

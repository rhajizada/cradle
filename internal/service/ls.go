package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

type AliasStatus struct {
	Name             string
	Kind             ImageKind
	ImageRef         string
	ImagePresent     bool
	ContainerName    string
	ContainerPresent bool
	ContainerStatus  string
}

func (s *Service) ListStatuses(ctx context.Context) ([]AliasStatus, error) {
	names := make([]string, 0, len(s.cfg.Aliases))
	for name := range s.cfg.Aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]AliasStatus, 0, len(names))
	for _, name := range names {
		status, err := s.aliasStatus(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, status)
	}

	return out, nil
}

func (s *Service) aliasStatus(ctx context.Context, name string) (AliasStatus, error) {
	info, err := s.AliasInfo(name)
	if err != nil {
		return AliasStatus{}, err
	}

	imageRef := resolveImageRef(info)
	imagePresent, err := s.imageExists(ctx, imageRef)
	if err != nil {
		return AliasStatus{}, err
	}

	containerName := s.resolveContainerName(name)
	containerPresent, containerStatus, err := s.containerInfo(ctx, containerName)
	if err != nil {
		return AliasStatus{}, err
	}

	return AliasStatus{
		Name:             info.Name,
		Kind:             info.Kind,
		ImageRef:         imageRef,
		ImagePresent:     imagePresent,
		ContainerName:    containerName,
		ContainerPresent: containerPresent,
		ContainerStatus:  containerStatus,
	}, nil
}

func resolveImageRef(info AliasInfo) string {
	if info.Kind == ImageBuild {
		return info.Tag
	}
	return info.Ref
}

func (s *Service) imageExists(ctx context.Context, ref string) (bool, error) {
	if _, err := s.cli.ImageInspect(ctx, ref); err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Service) resolveContainerName(alias string) string {
	if name := s.cfg.Aliases[alias].Run.Name; name != "" {
		return name
	}
	return fmt.Sprintf("cradle-%s", alias)
}

func (s *Service) containerInfo(ctx context.Context, name string) (bool, string, error) {
	ctr, err := s.cli.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, "missing", nil
		}
		return false, "", err
	}

	status := "unknown"
	if ctr.Container.State != nil {
		if state := string(ctr.Container.State.Status); state != "" {
			status = state
		}
	}

	return true, status, nil
}

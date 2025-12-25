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
		info, err := s.AliasInfo(name)
		if err != nil {
			return nil, err
		}

		imageRef := info.Ref
		if info.Kind == ImageBuild {
			imageRef = info.Tag
		}

		imagePresent := false
		if _, err := s.cli.ImageInspect(ctx, imageRef); err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, err
			}
		} else {
			imagePresent = true
		}

		a := s.cfg.Aliases[name]
		containerName := a.Run.Name
		if containerName == "" {
			containerName = fmt.Sprintf("cradle-%s", name)
		}

		containerPresent := false
		containerStatus := "missing"
		ctr, err := s.cli.ContainerInspect(ctx, containerName, client.ContainerInspectOptions{})
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, err
			}
		} else {
			containerPresent = true
			if ctr.Container.State != nil {
				if status := string(ctr.Container.State.Status); status != "" {
					containerStatus = status
				} else {
					containerStatus = "unknown"
				}
			} else {
				containerStatus = "unknown"
			}
		}

		out = append(out, AliasStatus{
			Name:             info.Name,
			Kind:             info.Kind,
			ImageRef:         imageRef,
			ImagePresent:     imagePresent,
			ContainerName:    containerName,
			ContainerPresent: containerPresent,
			ContainerStatus:  containerStatus,
		})
	}

	return out, nil
}

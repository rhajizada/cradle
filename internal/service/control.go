package service

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

func (s *Service) Start(ctx context.Context, alias string, out io.Writer) (string, error) {
	result, err := s.Run(ctx, alias, out)
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

func (s *Service) Stop(ctx context.Context, alias string) (string, error) {
	a, ok := s.cfg.Aliases[alias]
	if !ok {
		return "", fmt.Errorf("unknown alias %q", alias)
	}

	name := a.Run.Name
	if name == "" {
		name = fmt.Sprintf("cradle-%s", alias)
	}

	ctr, err := s.cli.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return "", fmt.Errorf("container %q not found", name)
		}
		return "", err
	}

	if ctr.Container.State != nil && ctr.Container.State.Running {
		if _, err := s.cli.ContainerStop(ctx, ctr.Container.ID, client.ContainerStopOptions{}); err != nil {
			return "", err
		}
	}

	return ctr.Container.ID, nil
}

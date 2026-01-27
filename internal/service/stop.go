package service

import (
	"context"
	"fmt"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

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
		if _, stopErr := s.cli.ContainerStop(ctx, ctr.Container.ID, client.ContainerStopOptions{}); stopErr != nil {
			return "", stopErr
		}
	}

	return ctr.Container.ID, nil
}

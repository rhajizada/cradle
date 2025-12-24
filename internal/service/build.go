package service

import (
	"context"
	"fmt"
	"io"

	"github.com/rhajizada/cradle/internal/config"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func pullImage(ctx context.Context, cli *client.Client, ref string, out io.Writer) error {
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()

	_, _ = io.Copy(out, resp)
	return nil
}

func buildImage(ctx context.Context, cli *client.Client, b *config.BuildSpec, tag string, out io.Writer) error {
	if b == nil {
		return fmt.Errorf("missing build spec")
	}

	contextDir := b.Cwd
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

		Version: build.BuilderBuildKit,
	}

	res, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, _ = io.Copy(out, res.Body)
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

package service_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func requireDocker(t *testing.T) *client.Client {
	t.Helper()

	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("docker client unavailable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, pingErr := cli.Ping(ctx, client.PingOptions{}); pingErr != nil {
		t.Skipf("docker not available: %v", pingErr)
	}
	return cli
}

func requireImage(t *testing.T, cli *client.Client, ref string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := cli.ImageInspect(ctx, ref); err == nil {
		return
	} else if !errdefs.IsNotFound(err) {
		t.Fatalf("image inspect failed: %v", err)
	}

	list, err := cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		t.Fatalf("image list failed: %v", err)
	}
	for _, item := range list.Items {
		for _, tag := range item.RepoTags {
			if tag == ref || strings.HasSuffix(tag, "/"+ref) {
				return
			}
		}
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer pullCancel()
	reader, err := cli.ImagePull(pullCtx, ref, client.ImagePullOptions{})
	if err != nil {
		t.Skipf("image %q not present and pull failed: %v", ref, err)
	}
	defer func() { _ = reader.Close() }()
	_, _ = io.Copy(io.Discard, reader)
}

func waitForExit(t *testing.T, cli *client.Client, id string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wait := cli.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case <-wait.Result:
	case err := <-wait.Error:
		if err != nil {
			t.Fatalf("container wait error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for container to stop")
	}
}

func TestRunReusesContainer(t *testing.T) {
	cli := requireDocker(t)
	const baseImage = "alpine:3.20"
	requireImage(t, cli, baseImage)

	ctx := context.Background()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(
		"FROM "+baseImage+"\nCMD [\"sh\",\"-lc\",\"exit 0\"]\n",
	), 0o600); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	name := fmt.Sprintf("cradle-test-%d", time.Now().UnixNano())
	attach := false
	tty := false
	stdin := false
	autoRemove := false

	alias := "test"
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			alias: {
				Image: config.ImageSpec{
					Build: &config.BuildSpec{Cwd: tmp},
				},
				Run: config.RunSpec{
					Name:       name,
					Attach:     &attach,
					TTY:        &tty,
					StdinOpen:  &stdin,
					AutoRemove: &autoRemove,
					Cmd:        []string{"sh", "-lc", "exit 0"},
				},
			},
		},
	}

	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service init: %v", err)
	}
	defer func() { _ = svc.Close() }()

	defer func() {
		_, _ = cli.ContainerRemove(context.Background(), name, client.ContainerRemoveOptions{Force: true})
		_, _ = cli.ImageRemove(
			context.Background(),
			"cradle/"+alias+":latest",
			client.ImageRemoveOptions{Force: true, PruneChildren: true},
		)
	}()

	first, err := svc.Run(ctx, alias, io.Discard, service.ImagePolicyOverrides{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	waitForExit(t, cli, first.ID)

	second, err := svc.Run(ctx, alias, io.Discard, service.ImagePolicyOverrides{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	waitForExit(t, cli, second.ID)

	if first.ID != second.ID {
		t.Fatalf("expected container reuse, got %q and %q", first.ID, second.ID)
	}

	cfg.Aliases[alias] = config.Alias{
		Image: cfg.Aliases[alias].Image,
		Run: config.RunSpec{
			Name:       name,
			Attach:     &attach,
			TTY:        &tty,
			StdinOpen:  &stdin,
			AutoRemove: &autoRemove,
			Cmd:        []string{"sh", "-lc", "echo ok"},
		},
	}

	third, err := svc.Run(ctx, alias, io.Discard, service.ImagePolicyOverrides{})
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	waitForExit(t, cli, third.ID)

	if third.ID == first.ID {
		t.Fatalf("expected container to be recreated after config change")
	}
}

func TestListStatusesIntegration(t *testing.T) {
	cli := requireDocker(t)
	const baseImage = "alpine:3.20"
	requireImage(t, cli, baseImage)

	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"alpine": {
				Image: config.ImageSpec{
					Pull: &config.PullSpec{Ref: baseImage},
				},
			},
		},
	}

	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service init: %v", err)
	}
	defer func() { _ = svc.Close() }()

	items, err := svc.ListStatuses(context.Background())
	if err != nil {
		t.Fatalf("ListStatuses error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "alpine" {
		t.Fatalf("unexpected items: %+v", items)
	}
	if !items[0].ImagePresent {
		t.Fatalf("expected image to be present")
	}
}

func TestStopRunningContainer(t *testing.T) {
	cli := requireDocker(t)
	const baseImage = "alpine:3.20"
	requireImage(t, cli, baseImage)

	name := fmt.Sprintf("cradle-stop-%d", time.Now().UnixNano())
	attach := false
	tty := false
	stdin := false
	autoRemove := false

	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"sleep": {
				Image: config.ImageSpec{
					Pull: &config.PullSpec{Ref: baseImage},
				},
				Run: config.RunSpec{
					Name:       name,
					Attach:     &attach,
					TTY:        &tty,
					StdinOpen:  &stdin,
					AutoRemove: &autoRemove,
					Cmd:        []string{"sh", "-lc", "sleep 60"},
				},
			},
		},
	}

	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service init: %v", err)
	}
	defer func() { _ = svc.Close() }()
	defer func() {
		_, _ = cli.ContainerRemove(context.Background(), name, client.ContainerRemoveOptions{Force: true})
	}()

	run, err := svc.Run(context.Background(), "sleep", io.Discard, service.ImagePolicyOverrides{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if _, stopErr := svc.Stop(context.Background(), "sleep"); stopErr != nil {
		t.Fatalf("stop error: %v", stopErr)
	}

	ctr, err := cli.ContainerInspect(context.Background(), run.ID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect error: %v", err)
	}
	if ctr.Container.State != nil && ctr.Container.State.Running {
		t.Fatalf("expected container to be stopped")
	}
}

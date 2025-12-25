package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/render"
	"github.com/rhajizada/cradle/internal/service"

	"github.com/spf13/cobra"
)

type appCtx struct {
	cfg      *config.Config
	svc      *service.Service
	renderer *render.Renderer
}

func newApp(cfgPath string, log *slog.Logger) (*appCtx, error) {
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	svc, err := service.New(cfg)
	if err != nil {
		return nil, err
	}
	return &appCtx{
		cfg:      cfg,
		svc:      svc,
		renderer: render.New(log),
	}, nil
}

func newBuildCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build <alias|all>",
		Short: "Build or pull images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if err := app.svc.Close(); err != nil {
					log.Warn("service close failed", "error", err)
				}
			}()

			target := args[0]
			if target == "all" {
				for _, info := range app.svc.ListAliases() {
					app.renderer.BuildStart(info)
					if err := app.svc.Build(cmd.Context(), info.Name, os.Stdout); err != nil {
						return fmt.Errorf("build %s: %w", info.Name, err)
					}
				}
				return nil
			}

			info, err := app.svc.AliasInfo(target)
			if err != nil {
				return err
			}
			app.renderer.BuildStart(info)
			return app.svc.Build(cmd.Context(), target, os.Stdout)
		},
	}
}

func newLsCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List aliases and status",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if err := app.svc.Close(); err != nil {
					log.Warn("service close failed", "error", err)
				}
			}()

			items, err := app.svc.ListStatuses(cmd.Context())
			if err != nil {
				return err
			}
			app.renderer.ListStatuses(items)
			return nil
		},
	}
}

func newRunCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "run <alias>",
		Short: "Run alias interactively",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if err := app.svc.Close(); err != nil {
					log.Warn("service close failed", "error", err)
				}
			}()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			info, err := app.svc.AliasInfo(args[0])
			if err != nil {
				return err
			}
			app.renderer.BuildStart(info)

			result, err := app.svc.Run(ctx, args[0], os.Stdout)
			if err != nil {
				return err
			}
			app.renderer.RunStart(result.ID)
			if !result.Attach {
				return nil
			}

			attachOpts := service.AttachOptions{
				ID:         result.ID,
				AutoRemove: result.AutoRemove,
				TTY:        result.TTY,
				Stdin:      os.Stdin,
				Stdout:     os.Stdout,
			}
			return app.svc.AttachAndWait(ctx, attachOpts)
		},
	}
}

func newStopCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <alias>",
		Short: "Stop alias container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if err := app.svc.Close(); err != nil {
					log.Warn("service close failed", "error", err)
				}
			}()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			id, err := app.svc.Stop(ctx, args[0])
			if err != nil {
				return err
			}
			app.renderer.RunStop(id)
			return nil
		},
	}
}

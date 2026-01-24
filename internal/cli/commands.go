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

type App struct {
	Cfg      *config.Config
	Svc      *service.Service
	Renderer *render.Renderer
}

func NewApp(cfgPath string, log *slog.Logger) (*App, error) {
	if cfgPath == "" {
		cfgPath = DefaultConfigPath()
	}
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	svc, err := service.New(cfg)
	if err != nil {
		return nil, err
	}
	return &App{
		Cfg:      cfg,
		Svc:      svc,
		Renderer: render.New(log, os.Stdout),
	}, nil
}

func NewBuildCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build <alias|all>",
		Short: "Build or pull images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := app.Svc.Close(); closeErr != nil {
					log.Warn("service close failed", "error", closeErr)
				}
			}()

			target := args[0]
			if target == "all" {
				for _, info := range app.Svc.ListAliases() {
					app.Renderer.BuildStart(info)
					if buildErr := app.Svc.Build(cmd.Context(), info.Name, os.Stdout); buildErr != nil {
						return fmt.Errorf("build %s: %w", info.Name, buildErr)
					}
				}
				return nil
			}

			info, err := app.Svc.AliasInfo(target)
			if err != nil {
				return err
			}
			app.Renderer.BuildStart(info)
			return app.Svc.Build(cmd.Context(), target, os.Stdout)
		},
	}
}

func NewLsCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List aliases and status",
		RunE: func(_ *cobra.Command, _ []string) error {
			app, err := NewApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := app.Svc.Close(); closeErr != nil {
					log.Warn("service close failed", "error", closeErr)
				}
			}()

			items, err := app.Svc.ListStatuses(context.Background())
			if err != nil {
				return err
			}
			app.Renderer.ListStatuses(items)
			return nil
		},
	}
}

func NewRunCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "run <alias>",
		Short: "Run alias interactively",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			app, err := NewApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := app.Svc.Close(); closeErr != nil {
					log.Warn("service close failed", "error", closeErr)
				}
			}()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			info, err := app.Svc.AliasInfo(args[0])
			if err != nil {
				return err
			}
			app.Renderer.BuildStart(info)

			result, err := app.Svc.Run(ctx, args[0], os.Stdout)
			if err != nil {
				return err
			}
			app.Renderer.RunStart(result.ID)
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
			return app.Svc.AttachAndWait(ctx, attachOpts)
		},
	}
}

func NewStopCmd(cfgPath *string, log *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <alias>",
		Short: "Stop alias container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive // cmd needed for cobra signature
			app, err := NewApp(*cfgPath, log)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := app.Svc.Close(); closeErr != nil {
					log.Warn("service close failed", "error", closeErr)
				}
			}()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			id, err := app.Svc.Stop(ctx, args[0])
			if err != nil {
				return err
			}
			app.Renderer.RunStop(id)
			return nil
		},
	}
}

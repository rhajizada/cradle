package cli

import (
	"log/slog"
	"os"

	"github.com/rhajizada/cradle/internal/logging"

	"github.com/spf13/cobra"
)

func Execute(version string) {
	if err := ExecuteArgs(version, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func ExecuteArgs(version string, args []string) error {
	log := logging.New(os.Stdout)
	errLog := logging.New(os.Stderr)

	root := NewRootCmd(version, log)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		errLog.Error("command failed", "error", err)
		return err
	}

	return nil
}

func NewRootCmd(version string, log *slog.Logger) *cobra.Command {
	var cfgPath string
	var showVersion bool

	root := &cobra.Command{
		Use:           "cradle",
		Short:         "Build and launch preconfigured Docker dev shells",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				log.Info("version", "version", version)
				return nil
			}
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = version

	root.PersistentFlags().
		StringVarP(&cfgPath, "config", "c", "", "config file (default is $XDG_CONFIG_HOME/cradle/config.yaml)")
	root.Flags().BoolVarP(&showVersion, "version", "V", false, "print version")

	root.AddCommand(
		NewBuildCmd(&cfgPath, log),
		NewLsCmd(&cfgPath, log),
		NewRunCmd(&cfgPath, log),
		NewStopCmd(&cfgPath, log),
	)

	return root
}

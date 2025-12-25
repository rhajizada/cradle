package cli

import (
	"log/slog"
	"os"

	"github.com/rhajizada/cradle/internal/logging"

	"github.com/spf13/cobra"
)

func Execute(version string) {
	log := logging.New(os.Stdout)
	errLog := logging.New(os.Stderr)

	root := newRootCmd(version, log)
	if err := root.Execute(); err != nil {
		errLog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func newRootCmd(version string, log *slog.Logger) *cobra.Command {
	var cfgPath string
	var showVersion bool

	root := &cobra.Command{
		Use:           "cradle",
		Short:         "Build and launch preconfigured Docker dev shells",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				log.Info("version", "version", version)
				return nil
			}
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = version

	root.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file (default is $XDG_CONFIG_HOME/cradle/config.yaml)")
	root.Flags().BoolVarP(&showVersion, "version", "V", false, "print version")

	root.AddCommand(
		newBuildCmd(&cfgPath, log),
		newRunCmd(&cfgPath, log),
		newStopCmd(&cfgPath, log),
	)

	return root
}

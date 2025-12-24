package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute(version string) {
	root := newRootCmd(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd(version string) *cobra.Command {
	var cfgPath string
	var showVersion bool

	root := &cobra.Command{
		Use:           "cradle",
		Short:         "cradle - build and run container aliases",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), version)
				return nil
			}
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = version

	root.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file (default is XDG_CONFIG_HOME/cradle/config.yaml)")
	root.Flags().BoolVarP(&showVersion, "version", "V", false, "print version")

	root.AddCommand(newAliasesCmd(&cfgPath), newBuildCmd(&cfgPath), newRunCmd(&cfgPath))

	return root
}

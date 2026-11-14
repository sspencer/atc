// Package commands defines the atc command line interface.
package commands

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute builds the root command and runs it with the given arguments.
func Execute(args []string) error {
	root := newRootCmd()
	root.SetArgs(args)
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	v := viper.New()

	root := &cobra.Command{
		Use:           "atc",
		Short:         "atc is an API gateway",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Let ATC_* environment variables supply values for any flag.
			v.SetEnvPrefix("ATC")
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.AutomaticEnv()
			return v.BindPFlags(cmd.Flags())
		},
	}

	root.AddCommand(newVersionCmd())
	return root
}

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current atc version.
const Version = "0.0.1"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the atc version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}

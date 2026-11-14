package commands

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time from git, e.g.:
//
//	go build -ldflags "-X github.com/sspencer/atc/commands.version=$(git describe --tags --always --dirty)"
//
// The Makefile does this automatically.
var version string

// Version reports the atc version. It prefers the value stamped in by the
// build, then the module version Go records from VCS (which covers
// `go install github.com/sspencer/atc@v1.2.3` and plain `go build` in a git
// checkout), and finally "dev".
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the atc version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version())
		},
	}
}

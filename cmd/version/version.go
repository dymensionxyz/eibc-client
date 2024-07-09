package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dymensionxyz/eibc-client/version"
)

func Cmd() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of roller",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.BuildVersion)
		},
	}
	return versionCmd
}

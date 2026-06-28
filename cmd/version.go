package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "development"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of db-schema-differ",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("db-schema-differ version %s (built on %s)\n", Version, BuildDate)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

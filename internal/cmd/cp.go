package cmd

import (
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:   "cp <name> <src> <dest>",
	Short: "Copy files to/from a sandbox",
	Long: `Copy files between local machine and sandbox using scp.
Prefix remote paths with ":" to indicate they are on the sandbox.

Examples:
  gjoll cp mybox ./config.env :/home/fedora/   Upload local file
  gjoll cp mybox :/home/fedora/output.log ./   Download remote file`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		src := args[1]
		dest := args[2]

		// Verify instance exists
		if _, err := state.Load(name); err != nil {
			return err
		}

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}

		configPath := remote.SSHConfigPath(instanceDir)
		return remote.Copy(configPath, name, src, dest)
	},
}

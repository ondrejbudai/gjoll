package cmd

import (
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <name> [branch-name]",
	Short: "Git fetch from sandbox VM and create local branch",
	Long: `Fetches from the sandbox's git remote and creates a local branch with the changes.

The remote is created automatically if it doesn't exist yet, so you can pull
without running "gjoll push" first.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		branchName := ""
		if len(args) > 1 {
			branchName = args[1]
		}

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}

		configPath := remote.SSHConfigPath(instanceDir)
		return remote.GitPull(configPath, name, "", branchName)
	},
}

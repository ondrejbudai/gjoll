package cmd

import (
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/spf13/cobra"
)

var pushPath string

var pushCmd = &cobra.Command{
	Use:   "push <name>",
	Short: "Git push current repo to sandbox VM",
	Long:  "Pushes the current local git repository to the sandbox. Sets up the remote repo on first push.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}

		configPath := remote.SSHConfigPath(instanceDir)

		return remote.GitPush(configPath, name, pushPath)
	},
}

func init() {
	pushCmd.Flags().StringVar(&pushPath, "path", "~/project", "remote path for the git repo")
}

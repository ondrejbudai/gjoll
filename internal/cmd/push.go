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
	Long: `Pushes the current local git repository to the sandbox VM.

On first push the remote repo is initialized automatically with
receive.denyCurrentBranch=updateInstead so the working tree on the VM updates
immediately. A local git remote named "gjoll-<name>" is added (or updated).`,
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
	pushCmd.Flags().StringVar(&pushPath, "path", "~/project", "path to the git repo on the VM")
}

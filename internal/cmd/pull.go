package cmd

import (
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/spf13/cobra"
)

var pullPath string

var pullCmd = &cobra.Command{
	Use:   "pull <name> [remote-branch[:local-branch]]",
	Short: "Git fetch from sandbox VM and create local branch",
	Long: `Fetches from the sandbox's git remote and creates a local branch with the changes.

The remote is created automatically if it doesn't exist yet, so you can pull
without running "gjoll push" first.

The optional refspec argument controls which branch to fetch and what to name
the local branch:

  gjoll pull my-vm                       # auto-detect remote branch → gjoll/my-vm
  gjoll pull my-vm feature               # fetch "feature" → gjoll/my-vm
  gjoll pull my-vm feature:my-branch     # fetch "feature" → my-branch
  gjoll pull my-vm :my-branch            # auto-detect remote branch → my-branch`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		var refspec string
		if len(args) > 1 {
			refspec = args[1]
		}
		remoteBranch, localBranch := remote.ParseRefspec(refspec)

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}

		configPath := remote.SSHConfigPath(instanceDir)
		return remote.GitPull(configPath, name, pullPath, remoteBranch, localBranch)
	},
}

func init() {
	pullCmd.Flags().StringVar(&pullPath, "path", "~/project", "path to the git repo on the VM")
}

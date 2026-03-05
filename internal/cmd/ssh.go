package cmd

import (
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <name> [-- command...]",
	Short: "SSH into a running sandbox",
	Long: `Opens an interactive SSH session to the named sandbox.

If a command is given after --, it is executed on the sandbox instead of
starting an interactive shell.

Examples:
  gjoll ssh mybox              Interactive shell
  gjoll ssh mybox -- uname -a  Run a command`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Verify instance exists
		if _, err := state.Load(name); err != nil {
			return err
		}

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}

		configPath := remote.SSHConfigPath(instanceDir)
		return remote.Connect(configPath, name, args[1:]...)
	},
}

func init() {
	sshCmd.Flags().SetInterspersed(false)
}

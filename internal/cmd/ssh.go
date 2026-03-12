package cmd

import (
	"fmt"

	"github.com/obudai/gjoll/internal/engine"
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var sshWakeup bool

var sshCmd = &cobra.Command{
	Use:   "ssh <name> [-- command...]",
	Short: "SSH into a running sandbox",
	Long: `Opens an interactive SSH session to the named sandbox.

If a command is given after --, it is executed on the sandbox instead of
starting an interactive shell.

With --wakeup, a stopped sandbox is started before connecting and stopped
again after the command finishes (requires a command).

Examples:
  gjoll ssh mybox              Interactive shell
  gjoll ssh mybox -- uname -a  Run a command
  gjoll ssh mybox --wakeup -- uname -a  Start, run, stop`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if sshWakeup && len(args) < 2 {
			return fmt.Errorf("--wakeup requires a command (e.g., gjoll ssh --wakeup %s -- uname -a)", name)
		}

		if sshWakeup {
			inst, err := state.Load(name)
			if err != nil {
				return err
			}

			if inst.Status == "stopped" {
				lock, err := state.Lock(name)
				if err != nil {
					return err
				}

				fmt.Println("Starting sandbox...")
				if err := engine.Start(name); err != nil {
					state.Unlock(lock)
					return err
				}
				state.Unlock(lock)

				defer func() {
					lock, err := state.Lock(name)
					if err != nil {
						fmt.Printf("Warning: could not lock for stop: %v\n", err)
						return
					}
					fmt.Println("Stopping sandbox...")
					if err := engine.Stop(name); err != nil {
						fmt.Printf("Warning: could not stop sandbox: %v\n", err)
					}
					state.Unlock(lock)
				}()
			}
		} else {
			// Verify instance exists
			if _, err := state.Load(name); err != nil {
				return err
			}
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
	sshCmd.Flags().BoolVar(&sshWakeup, "wakeup", false, "start a stopped sandbox, run the command, then stop it")
}

package cmd

import (
	"github.com/ondrejbudai/gjoll/internal/engine"
	"github.com/ondrejbudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name> [name...]",
	Short: "Stop a running sandbox",
	Long: `Stops a running sandbox by setting gjoll_instance_state to "stopped"
and running tofu apply. The instance is preserved and can be started again
with "gjoll start".`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, name := range args {
			lock, err := state.Lock(name)
			if err != nil {
				return err
			}

			if err := engine.Stop(name); err != nil {
				state.Unlock(lock)
				return err
			}
			state.Unlock(lock)
		}
		return nil
	},
}

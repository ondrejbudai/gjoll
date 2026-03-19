package cmd

import (
	"github.com/ondrejbudai/gjoll/internal/engine"
	"github.com/ondrejbudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down <name> [name...]",
	Short: "Destroy a sandbox and all its resources",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, name := range args {
			lock, err := state.Lock(name)
			if err != nil {
				return err
			}

			if err := engine.Destroy(name); err != nil {
				state.Unlock(lock)
				return err
			}
			state.Unlock(lock)
		}
		return nil
	},
}

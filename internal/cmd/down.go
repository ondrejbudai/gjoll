package cmd

import (
	"github.com/obudai/gjoll/internal/engine"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down <name>",
	Short: "Destroy a sandbox and all its resources",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		lock, err := state.Lock(name)
		if err != nil {
			return err
		}
		defer state.Unlock(lock)

		return engine.Destroy(name)
	},
}

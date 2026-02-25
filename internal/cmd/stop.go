package cmd

import (
	"github.com/obudai/gjoll/internal/engine"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a sandbox VM (preserves disk)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		lock, err := state.Lock(name)
		if err != nil {
			return err
		}
		defer state.Unlock(lock)

		return engine.Stop(name)
	},
}

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Resume a stopped sandbox VM",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		lock, err := state.Lock(name)
		if err != nil {
			return err
		}
		defer state.Unlock(lock)

		return engine.Start(name)
	},
}

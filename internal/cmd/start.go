package cmd

import (
	"github.com/ondrejbudai/gjoll/internal/engine"
	"github.com/ondrejbudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a stopped sandbox",
	Long: `Starts a previously stopped sandbox by setting gjoll_instance_state to
"running" and running tofu apply. The IP address may change after a restart,
so the SSH config is automatically updated.`,
	Args: cobra.ExactArgs(1),
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

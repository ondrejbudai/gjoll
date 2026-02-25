package cmd

import (
	"fmt"

	"github.com/obudai/gjoll/internal/engine"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var upName string

var upCmd = &cobra.Command{
	Use:   "up <env>",
	Short: "Create and launch a sandbox VM",
	Long:  "Provisions a new VM from an OpenTofu environment (single .tf file or directory).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envPath := args[0]
		name := upName
		if name == "" {
			name = engine.DeriveName(envPath)
		}

		// Check if already exists
		if _, err := state.Load(name); err == nil {
			return fmt.Errorf("sandbox %q already exists — use 'gjoll down %s' first", name, name)
		}

		lock, err := state.Lock(name)
		if err != nil {
			return err
		}
		defer state.Unlock(lock)

		return engine.Provision(name, envPath)
	},
}

func init() {
	upCmd.Flags().StringVarP(&upName, "name", "n", "", "sandbox name (default: derived from env path)")
}

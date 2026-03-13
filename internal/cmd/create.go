package cmd

import (
	"fmt"

	"github.com/ondrejbudai/gjoll/internal/engine"
	"github.com/ondrejbudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var createName string

var createCmd = &cobra.Command{
	Use:   "create <env>",
	Short: "Create a sandbox VM and stop it after initialization",
	Long: `Provisions a new VM like "gjoll up", but stops the instance after the init
script and file copies complete. Useful for pre-building sandboxes that
can be started later with "gjoll start".`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envPath := args[0]
		name := createName
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

		if err := engine.Provision(name, envPath); err != nil {
			return err
		}

		fmt.Println("\nStopping sandbox after initialization...")
		return engine.Stop(name)
	},
}

func init() {
	createCmd.Flags().StringVarP(&createName, "name", "n", "", "sandbox name (default: derived from env path)")
}

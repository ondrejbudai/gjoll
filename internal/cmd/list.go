package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sandboxes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		instances, err := state.List()
		if err != nil {
			return err
		}

		if len(instances) == 0 {
			fmt.Println("No sandboxes.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tIP\tUSER\tINSTANCE ID")
		for _, inst := range instances {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				inst.Name, inst.Status, inst.PublicIP, inst.SSHUser, inst.InstanceID)
		}
		return w.Flush()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show sandbox details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inst, err := state.Load(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Name:        %s\n", inst.Name)
		fmt.Printf("Status:      %s\n", inst.Status)
		fmt.Printf("IP:          %s\n", inst.PublicIP)
		fmt.Printf("User:        %s\n", inst.SSHUser)
		fmt.Printf("Instance ID: %s\n", inst.InstanceID)
		fmt.Printf("Env Path:    %s\n", inst.EnvPath)
		fmt.Printf("Created:     %s\n", inst.CreatedAt.Format("2006-01-02 15:04:05"))

		return nil
	},
}

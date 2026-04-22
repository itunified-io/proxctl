package root

import "github.com/spf13/cobra"

func newVMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "vm",
		Short: "Manage individual VMs (create, start, stop, delete, list, status)",
	}
	c.AddCommand(
		&cobra.Command{Use: "create NAME", Short: "Create a VM from env spec", Args: cobra.ExactArgs(1), RunE: notImplemented("vm create")},
		&cobra.Command{Use: "start NAME", Short: "Start a VM", Args: cobra.ExactArgs(1), RunE: notImplemented("vm start")},
		&cobra.Command{Use: "stop NAME", Short: "Stop a VM (ACPI shutdown; --force for hard stop)", Args: cobra.ExactArgs(1), RunE: notImplemented("vm stop")},
		&cobra.Command{Use: "reboot NAME", Short: "Reboot a VM", Args: cobra.ExactArgs(1), RunE: notImplemented("vm reboot")},
		&cobra.Command{Use: "delete NAME", Short: "Delete a VM (double-confirm gate)", Args: cobra.ExactArgs(1), RunE: notImplemented("vm delete")},
		&cobra.Command{Use: "list", Short: "List VMs known to state.db", RunE: notImplemented("vm list")},
		&cobra.Command{Use: "status NAME", Short: "Print live VM status (PVE + state.db)", Args: cobra.ExactArgs(1), RunE: notImplemented("vm status")},
	)
	return c
}

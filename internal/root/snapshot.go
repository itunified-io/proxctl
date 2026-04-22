package root

import "github.com/spf13/cobra"

func newSnapshotCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage VM snapshots",
	}
	c.AddCommand(
		&cobra.Command{Use: "create VM NAME", Short: "Create a snapshot", Args: cobra.ExactArgs(2), RunE: notImplemented("snapshot create")},
		&cobra.Command{Use: "restore VM NAME", Short: "Restore a snapshot (double-confirm gate)", Args: cobra.ExactArgs(2), RunE: notImplemented("snapshot restore")},
		&cobra.Command{Use: "list VM", Short: "List snapshots for a VM", Args: cobra.ExactArgs(1), RunE: notImplemented("snapshot list")},
		&cobra.Command{Use: "delete VM NAME", Short: "Delete a snapshot", Args: cobra.ExactArgs(2), RunE: notImplemented("snapshot delete")},
	)
	return c
}

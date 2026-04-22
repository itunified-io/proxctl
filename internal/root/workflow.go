package root

import "github.com/spf13/cobra"

func newWorkflowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "workflow",
		Short: "Multi-VM idempotent orchestration (plan, up, down, status, verify)",
	}
	c.AddCommand(
		&cobra.Command{Use: "plan", Short: "Dry-run: print DAG and diff desired vs current", RunE: notImplemented("workflow plan")},
		&cobra.Command{Use: "up", Short: "Apply the workflow (idempotent)", RunE: notImplemented("workflow up")},
		&cobra.Command{Use: "down", Short: "Tear down the workflow (double-confirm gate)", RunE: notImplemented("workflow down")},
		&cobra.Command{Use: "status", Short: "Show current vs desired state", RunE: notImplemented("workflow status")},
		&cobra.Command{Use: "verify", Short: "Post-deploy SSH health checks", RunE: notImplemented("workflow verify")},
	)
	return c
}

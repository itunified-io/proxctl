package root

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Manage proxclt contexts (~/.proxclt/config.yaml)",
	}
	c.AddCommand(
		&cobra.Command{Use: "validate", Short: "Validate schema of an env or all registered envs", RunE: notImplemented("config validate")},
		&cobra.Command{Use: "render", Short: "Render composed env (inline $refs, resolve secrets)", RunE: notImplemented("config render")},
		&cobra.Command{Use: "use-context NAME", Short: "Switch current context", Args: cobra.ExactArgs(1), RunE: notImplemented("config use-context")},
		&cobra.Command{Use: "current-context", Short: "Print current context", RunE: notImplemented("config current-context")},
		&cobra.Command{Use: "get-contexts", Short: "List all configured contexts", RunE: notImplemented("config get-contexts")},
	)
	return c
}

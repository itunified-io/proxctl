package root

import "github.com/spf13/cobra"

func newKickstartCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "kickstart",
		Short: "Render kickstart configs, build + upload install ISOs",
	}
	c.AddCommand(
		&cobra.Command{Use: "generate", Short: "Render kickstart to stdout or file", RunE: notImplemented("kickstart generate")},
		&cobra.Command{Use: "build-iso", Short: "Remaster install ISO with the rendered kickstart", RunE: notImplemented("kickstart build-iso")},
		&cobra.Command{Use: "upload FILE", Short: "Upload an ISO to PVE storage", Args: cobra.ExactArgs(1), RunE: notImplemented("kickstart upload")},
		&cobra.Command{Use: "distros", Short: "List supported distros", RunE: notImplemented("kickstart distros")},
	)
	return c
}

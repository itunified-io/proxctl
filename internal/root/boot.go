package root

import "github.com/spf13/cobra"

func newBootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "boot",
		Short: "Configure first-boot ISO + post-install ISO ejection",
	}
	c.AddCommand(
		&cobra.Command{Use: "configure-first-boot VM", Short: "Attach the kickstart ISO and set boot order", Args: cobra.ExactArgs(1), RunE: notImplemented("boot configure-first-boot")},
		&cobra.Command{Use: "eject-iso VM", Short: "Detach the install ISO after first boot", Args: cobra.ExactArgs(1), RunE: notImplemented("boot eject-iso")},
	)
	return c
}

package root

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// boot commands use Client.Do() directly because pkg/proxmox has not exposed
// dedicated AttachISOAsCDROM/SetBootOrder/ConfigureFirstBoot helpers yet.
func newBootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "boot",
		Short: "Configure first-boot ISO + post-install ISO ejection",
	}

	var bootISO string
	var bootIDE string
	var bootOrder string

	c.AddCommand(
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "configure-first-boot NAME",
				Short: "Attach the kickstart ISO and set boot order",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					env, err := loadEnvManifest("")
					if err != nil {
						return err
					}
					node, vmid, err := resolveNodeRef(env, args[0])
					if err != nil {
						return err
					}
					client, err := loadProxmoxClient()
					if err != nil {
						return err
					}
					if bootISO == "" {
						return fmt.Errorf("--iso required (e.g. local:iso/kickstart.iso)")
					}
					form := url.Values{}
					form.Set(bootIDE, bootISO+",media=cdrom")
					if bootOrder != "" {
						form.Set("boot", "order="+bootOrder)
					}
					path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
					return client.Do(context.Background(), http.MethodPut, path, form, nil)
				},
			}
			cc.Flags().StringVar(&bootISO, "iso", "", "PVE ISO ref (e.g. local:iso/kickstart.iso)")
			cc.Flags().StringVar(&bootIDE, "ide", "ide3", "ide slot to attach the ISO to")
			cc.Flags().StringVar(&bootOrder, "order", "ide3;scsi0", "boot order string")
			return cc
		}(),
		func() *cobra.Command {
			var ejectIDE string
			cc := &cobra.Command{
				Use:   "eject-iso NAME",
				Short: "Detach the install ISO after first boot",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					env, err := loadEnvManifest("")
					if err != nil {
						return err
					}
					node, vmid, err := resolveNodeRef(env, args[0])
					if err != nil {
						return err
					}
					client, err := loadProxmoxClient()
					if err != nil {
						return err
					}
					form := url.Values{}
					form.Set(ejectIDE, "none,media=cdrom")
					path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
					return client.Do(context.Background(), http.MethodPut, path, form, nil)
				},
			}
			cc.Flags().StringVar(&ejectIDE, "ide", "ide3", "ide slot to eject")
			return cc
		}(),
	)
	return c
}

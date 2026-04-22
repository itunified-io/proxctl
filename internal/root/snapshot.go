package root

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// snapshot commands use Client.Do() directly since pkg/proxmox does not expose
// dedicated snapshot helpers yet.
func newSnapshotCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage VM snapshots",
	}

	c.AddCommand(
		&cobra.Command{
			Use:   "create VM SNAPNAME",
			Short: "Create a snapshot",
			Args:  cobra.ExactArgs(2),
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
				form.Set("snapname", args[1])
				path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
				return client.Do(context.Background(), http.MethodPost, path, form, nil)
			},
		},
		&cobra.Command{
			Use:   "restore VM SNAPNAME",
			Short: "Rollback a snapshot",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				if !flagYes {
					return fmt.Errorf("refusing to restore without --yes")
				}
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
				path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/rollback", node, vmid, args[1])
				return client.Do(context.Background(), http.MethodPost, path, url.Values{}, nil)
			},
		},
		&cobra.Command{
			Use:   "list VM",
			Short: "List snapshots",
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
				path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
				var raws []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					SnapTime    int64  `json:"snaptime"`
					VMState     int    `json:"vmstate"`
				}
				if err := client.Do(context.Background(), http.MethodGet, path, nil, &raws); err != nil {
					return err
				}
				if flagJSON {
					return json.NewEncoder(os.Stdout).Encode(raws)
				}
				tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tTIME\tDESCRIPTION")
				for _, s := range raws {
					fmt.Fprintf(tw, "%s\t%d\t%s\n", s.Name, s.SnapTime, s.Description)
				}
				return tw.Flush()
			},
		},
		&cobra.Command{
			Use:   "delete VM SNAPNAME",
			Short: "Delete a snapshot",
			Args:  cobra.ExactArgs(2),
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
				path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s", node, vmid, args[1])
				return client.Do(context.Background(), http.MethodDelete, path, nil, nil)
			},
		},
	)
	return c
}

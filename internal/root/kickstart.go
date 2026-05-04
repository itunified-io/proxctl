package root

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/itunified-io/proxctl/pkg/kickstart"
	"github.com/itunified-io/proxctl/pkg/proxmox"
	"github.com/spf13/cobra"
)

// copyISOFile is a tiny helper for cross-device os.Rename fallback in build-stack.
func copyISOFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func newKickstartCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "kickstart",
		Short: "Render kickstart configs, build + upload install ISOs",
	}

	var genNode string
	var genOutDir string
	var buildOut string
	var buildBootloader string
	var uploadStorage string
	var uploadNode string
	var ubuntuNode string
	var ubuntuSourceISO string
	var ubuntuOut string
	var ubuntuWorkDir string

	var stackNode string
	var stackSourceISO string
	var stackOutDir string
	var stackUpload bool
	var stackKeep bool

	c.AddCommand(
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "generate [ENV_FILE]",
				Short: "Render kickstart to a file",
				Args:  cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					var envPath string
					if len(args) > 0 {
						envPath = args[0]
					}
					env, err := loadEnvManifest(envPath)
					if err != nil {
						return err
					}
					rnd, err := kickstart.NewRenderer()
					if err != nil {
						return err
					}
					nodes := []string{genNode}
					if genNode == "" {
						hyp := env.Spec.Hypervisor.Resolved()
						if hyp == nil {
							return fmt.Errorf("hypervisor not resolved")
						}
						nodes = nil
						for n := range hyp.Nodes {
							nodes = append(nodes, n)
						}
					}
					if genOutDir == "" {
						genOutDir = "."
					}
					if err := os.MkdirAll(genOutDir, 0o755); err != nil {
						return err
					}
					for _, n := range nodes {
						content, err := rnd.Render(env, n)
						if err != nil {
							return fmt.Errorf("render %s: %w", n, err)
						}
						out := filepath.Join(genOutDir, n+".ks")
						if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
							return err
						}
						fmt.Fprintf(os.Stderr, "wrote %s\n", out)
					}
					return nil
				},
			}
			cc.Flags().StringVar(&genNode, "node", "", "single node name (default: render all)")
			cc.Flags().StringVarP(&genOutDir, "out", "o", ".", "output directory")
			return cc
		}(),
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "build-iso KSFILE",
				Short: "Remaster install ISO with the rendered kickstart",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					ksData, err := os.ReadFile(args[0])
					if err != nil {
						return err
					}
					if buildBootloader == "" {
						return fmt.Errorf("--bootloader-dir required")
					}
					b := kickstart.NewISOBuilder(buildBootloader)
					hostname := strings.TrimSuffix(filepath.Base(args[0]), ".ks")
					path, err := b.Build(string(ksData), hostname)
					if err != nil {
						return err
					}
					if buildOut != "" && buildOut != path {
						if err := os.Rename(path, buildOut); err != nil {
							return err
						}
						path = buildOut
					}
					fmt.Println(path)
					return nil
				},
			}
			cc.Flags().StringVar(&buildBootloader, "bootloader-dir", "", "directory containing isolinux.bin + vmlinuz + initrd.img")
			cc.Flags().StringVarP(&buildOut, "out", "o", "", "output ISO path (default: tempdir)")
			return cc
		}(),
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "upload FILE",
				Short: "Upload an ISO to a Proxmox storage",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					if uploadStorage == "" || uploadNode == "" {
						return fmt.Errorf("--storage and --node required")
					}
					client, err := loadProxmoxClient()
					if err != nil {
						return err
					}
					return client.UploadISO(context.Background(), uploadNode, uploadStorage, args[0], filepath.Base(args[0]))
				},
			}
			cc.Flags().StringVar(&uploadStorage, "storage", "", "PVE storage name")
			cc.Flags().StringVar(&uploadNode, "node", "", "PVE node name")
			return cc
		}(),
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "build-ubuntu [ENV_FILE]",
				Short: "Build Ubuntu 22.04+ Subiquity autoinstall ISO (cidata + GRUB cmdline injection)",
				Long: `Renders user-data + meta-data from the env manifest, embeds them at /cidata/
in a remastered copy of the upstream Ubuntu live-server install ISO, and
patches the GRUB linux/linuxefi/isolinux append lines to add
` + "`autoinstall ds=nocloud\\;s=/cidata/`" + ` to the kernel cmdline. The
result is a standalone ISO that boots Ubuntu unattended via Subiquity.

Use this when env.spec.hypervisor.kickstart.distro = "ubuntu2404".`,
				Args: cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					if ubuntuSourceISO == "" {
						return fmt.Errorf("--source-iso required (path to upstream Ubuntu live-server ISO)")
					}
					if ubuntuNode == "" {
						return fmt.Errorf("--node required")
					}
					var envPath string
					if len(args) > 0 {
						envPath = args[0]
					}
					env, err := loadEnvManifest(envPath)
					if err != nil {
						return err
					}
					rnd, err := kickstart.NewRenderer()
					if err != nil {
						return err
					}
					userData, err := rnd.RenderTemplate(env, ubuntuNode, "user-data.tmpl")
					if err != nil {
						return fmt.Errorf("render user-data: %w", err)
					}
					metaData, err := rnd.RenderTemplate(env, ubuntuNode, "meta-data.tmpl")
					if err != nil {
						return fmt.Errorf("render meta-data: %w", err)
					}
					cidataDir, err := os.MkdirTemp("", "proxctl-ubuntu-cidata-")
					if err != nil {
						return err
					}
					defer os.RemoveAll(cidataDir)
					if err := os.WriteFile(filepath.Join(cidataDir, "user-data"), []byte(userData), 0o644); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(cidataDir, "meta-data"), []byte(metaData), 0o644); err != nil {
						return err
					}
					b := kickstart.NewUbuntuISOBuilder(ubuntuSourceISO, cidataDir)
					if ubuntuWorkDir != "" {
						b.WorkDir = ubuntuWorkDir
					}
					path, err := b.Build(ubuntuNode)
					if err != nil {
						return err
					}
					if ubuntuOut != "" && ubuntuOut != path {
						if err := os.Rename(path, ubuntuOut); err != nil {
							return err
						}
						path = ubuntuOut
					}
					fmt.Println(path)
					return nil
				},
			}
			cc.Flags().StringVar(&ubuntuNode, "node", "", "Node name from hypervisor.nodes (used as hostname)")
			cc.Flags().StringVar(&ubuntuSourceISO, "source-iso", "", "Path to upstream Ubuntu live-server install ISO")
			cc.Flags().StringVarP(&ubuntuOut, "out", "o", "", "Output ISO path (default: tempdir under workdir)")
			cc.Flags().StringVar(&ubuntuWorkDir, "workdir", "", "Scratch directory (default: $TMPDIR)")
			return cc
		}(),
		func() *cobra.Command {
			cc := &cobra.Command{
				Use:   "build-stack [ENV_FILE]",
				Short: "Build per-node OL8/OL9 install ISOs from a stack manifest (integrated render+extract+build+upload)",
				Long: `Mirrors build-ubuntu for OL8/OL9 (and other RHEL-family) distros: extracts
the bootloader (isolinux.bin, ldlinux.c32, vmlinuz, initrd.img) from the
upstream install ISO once, then for each node renders ks.cfg, builds a
kickstart-only boot ISO, and optionally uploads it to the node's
hypervisor.iso.kickstart_storage.

Errors out for ubuntu* distros — use ` + "`build-ubuntu`" + ` instead.`,
				Args: cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					if stackSourceISO == "" {
						return fmt.Errorf("--source-iso required (path to upstream OL install ISO)")
					}
					var envPath string
					if len(args) > 0 {
						envPath = args[0]
					}
					env, err := loadEnvManifest(envPath)
					if err != nil {
						return err
					}
					hyp := env.Spec.Hypervisor.Resolved()
					if hyp == nil {
						return fmt.Errorf("env.spec.hypervisor not resolved")
					}
					if hyp.Kickstart == nil {
						return fmt.Errorf("env has no kickstart config")
					}
					distro := hyp.Kickstart.Distro
					if strings.HasPrefix(distro, "ubuntu") {
						return fmt.Errorf("distro %q is Ubuntu-family — use `proxctl kickstart build-ubuntu` instead", distro)
					}

					// Pick nodes.
					var nodes []string
					if stackNode != "" {
						if _, ok := hyp.Nodes[stackNode]; !ok {
							return fmt.Errorf("node %q not found in hypervisor.nodes", stackNode)
						}
						nodes = []string{stackNode}
					} else {
						for n := range hyp.Nodes {
							nodes = append(nodes, n)
						}
					}
					if len(nodes) == 0 {
						return fmt.Errorf("no nodes to build")
					}

					// Extract bootloader once.
					bootloaderDir, err := os.MkdirTemp("", "proxctl-bootloader-")
					if err != nil {
						return fmt.Errorf("mkdtemp bootloader: %w", err)
					}
					defer os.RemoveAll(bootloaderDir)
					if err := kickstart.ExtractBootloader(stackSourceISO, bootloaderDir); err != nil {
						return fmt.Errorf("extract bootloader: %w", err)
					}
					fmt.Fprintf(os.Stderr, "extracted bootloader from %s -> %s\n", stackSourceISO, bootloaderDir)

					// Read upstream ISO volume label so Anaconda can locate
					// stage2 + repo on the second CDROM drive (Path B-lite,
					// fixes dracut-initqueue DNS errors fetching stage2 from
					// the network).
					sourceLabel, err := kickstart.ReadVolumeLabel(stackSourceISO)
					if err != nil {
						return fmt.Errorf("read source ISO volume label: %w", err)
					}
					fmt.Fprintf(os.Stderr, "source ISO volume label: %s\n", sourceLabel)

					// Output dir for ISOs.
					outDir := stackOutDir
					if outDir == "" {
						outDir, err = os.MkdirTemp("", "proxctl-stack-iso-")
						if err != nil {
							return fmt.Errorf("mkdtemp out: %w", err)
						}
					} else {
						if err := os.MkdirAll(outDir, 0o755); err != nil {
							return err
						}
					}

					rnd, err := kickstart.NewRenderer()
					if err != nil {
						return err
					}
					builder := kickstart.NewISOBuilder(bootloaderDir)
					builder.SourceISOLabel = sourceLabel

					var pveClient *proxmox.Client
					if stackUpload {
						client, err := loadProxmoxClient()
						if err != nil {
							return fmt.Errorf("load proxmox client: %w", err)
						}
						pveClient = client
						if hyp.ISO == nil || hyp.ISO.KickstartStorage == "" {
							return fmt.Errorf("--upload requires hypervisor.iso.kickstart_storage in env manifest")
						}
					}

					for _, n := range nodes {
						nodeCfg, ok := hyp.Nodes[n]
						if !ok {
							return fmt.Errorf("node %q vanished from hypervisor.nodes", n)
						}
						content, err := rnd.Render(env, n)
						if err != nil {
							return fmt.Errorf("render %s: %w", n, err)
						}
						isoPath, err := builder.Build(content, n)
						if err != nil {
							return fmt.Errorf("build iso %s: %w", n, err)
						}
						// Move into outDir if it's not already under there.
						destPath := filepath.Join(outDir, filepath.Base(isoPath))
						if isoPath != destPath {
							if err := os.Rename(isoPath, destPath); err != nil {
								// Cross-device fallback: copy + remove.
								if err2 := copyISOFile(isoPath, destPath); err2 != nil {
									return fmt.Errorf("relocate iso %s -> %s: %w (copy fallback: %v)", isoPath, destPath, err, err2)
								}
								_ = os.Remove(isoPath)
							}
							isoPath = destPath
						}
						fmt.Println(isoPath)

						if stackUpload {
							storage := hyp.ISO.KickstartStorage
							pveNode := nodeCfg.Proxmox.NodeName
							if pveNode == "" {
								return fmt.Errorf("node %q has no proxmox.node_name", n)
							}
							if err := pveClient.UploadISO(context.Background(), pveNode, storage, isoPath, filepath.Base(isoPath)); err != nil {
								return fmt.Errorf("upload %s -> %s/%s: %w", isoPath, pveNode, storage, err)
							}
							fmt.Fprintf(os.Stderr, "uploaded %s to %s:%s\n", filepath.Base(isoPath), pveNode, storage)
							if !stackKeep {
								_ = os.Remove(isoPath)
							}
						}
					}
					return nil
				},
			}
			cc.Flags().StringVar(&stackNode, "node", "", "single node name (default: build all nodes in env)")
			cc.Flags().StringVar(&stackSourceISO, "source-iso", "", "path to upstream OL install ISO (required)")
			cc.Flags().StringVar(&stackOutDir, "out-dir", "", "output directory for per-host ISOs (default: tempdir)")
			cc.Flags().BoolVar(&stackUpload, "upload", false, "upload each ISO to its node's hypervisor.iso.kickstart_storage")
			cc.Flags().BoolVar(&stackKeep, "keep", false, "keep local ISOs after upload (default: delete)")
			return cc
		}(),
		&cobra.Command{
			Use:   "distros",
			Short: "List supported distros",
			RunE: func(cmd *cobra.Command, args []string) error {
				rnd, err := kickstart.NewRenderer()
				if err != nil {
					return err
				}
				for _, d := range rnd.SupportedDistros() {
					fmt.Println(d)
				}
				return nil
			},
		},
	)
	return c
}

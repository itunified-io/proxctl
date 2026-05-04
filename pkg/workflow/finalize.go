package workflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/itunified-io/proxctl/pkg/config"
	"github.com/itunified-io/proxctl/pkg/proxmox"
)

// DefaultFinalizeTimeout is the maximum time finalize waits for SSH-up
// before giving up. Generous because a kickstart install on slow disks +
// slow networks can take 20+ min; we still want a hard ceiling.
const DefaultFinalizeTimeout = 30 * time.Minute

// DefaultFinalizePollInterval is how often finalize retries the SSH-up
// probe between attempts.
const DefaultFinalizePollInterval = 10 * time.Second

// sshDialer is the net.Dial signature, parameterised so tests can inject a
// fake. Production code wires this to a net.Dialer with timeout.
type sshDialer func(ctx context.Context, network, address string) (net.Conn, error)

// FinalizeOptions controls SSH-up polling and post-install cleanup.
//
// The defaults (Timeout = 30m, PollInterval = 10s) are sized for a slow
// kickstart install on commodity SATA + 1G LAN. Override via
// SingleVMWorkflow / MultiNodeWorkflow fields when calling Up.
type FinalizeOptions struct {
	// Timeout caps the SSH-up wait. Zero → DefaultFinalizeTimeout.
	Timeout time.Duration
	// PollInterval gates retry frequency. Zero → DefaultFinalizePollInterval.
	PollInterval time.Duration
	// SSHPort is the TCP port to probe. Zero → 22.
	SSHPort int
	// Dialer overrides the default TCP dialer (for tests). Production callers
	// leave this nil.
	Dialer sshDialer
	// SkipSSHWait is for tests / re-runs where SSH-up has already been verified
	// out-of-band. Production callers leave this false.
	SkipSSHWait bool
}

func (o *FinalizeOptions) timeout() time.Duration {
	if o == nil || o.Timeout <= 0 {
		return DefaultFinalizeTimeout
	}
	return o.Timeout
}

func (o *FinalizeOptions) pollInterval() time.Duration {
	if o == nil || o.PollInterval <= 0 {
		return DefaultFinalizePollInterval
	}
	return o.PollInterval
}

func (o *FinalizeOptions) sshPort() int {
	if o == nil || o.SSHPort <= 0 {
		return 22
	}
	return o.SSHPort
}

func (o *FinalizeOptions) dialer() sshDialer {
	if o != nil && o.Dialer != nil {
		return o.Dialer
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return d.DialContext(ctx, network, address)
	}
}

// pickFinalizeIP selects the IP address to probe for SSH-up.
//
// Priority: management → public → private → any (first by key sort).
// Returns the IP string and a label describing which slot was chosen, or
// ("", "") if no IPs are configured.
func pickFinalizeIP(ips map[string]string) (ip, label string) {
	if len(ips) == 0 {
		return "", ""
	}
	for _, k := range []string{"management", "public", "private"} {
		if v, ok := ips[k]; ok && v != "" {
			return v, k
		}
	}
	keys := make([]string, 0, len(ips))
	for k := range ips {
		if ips[k] != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "", ""
	}
	sort.Strings(keys)
	return ips[keys[0]], keys[0]
}

// waitForSSHUp polls a TCP :22 connection until the SSH banner ("SSH-") is
// observed, or the deadline fires. Returns nil on success.
func waitForSSHUp(ctx context.Context, ip string, opts *FinalizeOptions) error {
	if opts != nil && opts.SkipSSHWait {
		return nil
	}
	if ip == "" {
		return errors.New("finalize: no IP available for SSH-up probe")
	}
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", opts.sshPort()))
	dial := opts.dialer()

	deadline := time.Now().Add(opts.timeout())
	interval := opts.pollInterval()

	var lastErr error
	for {
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("timeout after %s", opts.timeout())
			}
			return fmt.Errorf("finalize: ssh-up wait %s: %w", addr, lastErr)
		}
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		conn, err := dial(dctx, "tcp", addr)
		cancel()
		if err == nil {
			// Read up to ~32 bytes of banner; SSH servers send "SSH-2.0-..."
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			buf := make([]byte, 64)
			n, rerr := conn.Read(buf)
			_ = conn.Close()
			if rerr == nil && n > 0 && strings.HasPrefix(string(buf[:n]), "SSH-") {
				return nil
			}
			lastErr = fmt.Errorf("connected but no SSH banner (read=%d, err=%v)", n, rerr)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// FinalizeSingle detaches the install + kickstart ISOs from one VM and
// resets the boot order to just scsi0. Caller is responsible for ensuring
// the VM has reached a steady state (typically via waitForSSHUp).
//
// The detach calls are best-effort — Proxmox returns 200 + null when the
// slot is already absent, which is fine for re-runs.
func FinalizeSingle(ctx context.Context, client *proxmox.Client, node string, vmid int) error {
	if client == nil {
		return errors.New("finalize: Client not set")
	}
	// Order: kickstart (ide3) first, then install (ide2). If a partial
	// previous run left ide3 detached, EjectISO is idempotent enough that
	// the second call still works.
	if err := client.EjectISO(ctx, node, vmid, "ide3"); err != nil {
		return fmt.Errorf("finalize: detach ide3 (kickstart): %w", err)
	}
	if err := client.EjectISO(ctx, node, vmid, "ide2"); err != nil {
		return fmt.Errorf("finalize: detach ide2 (install): %w", err)
	}
	if err := client.SetBootOrder(ctx, node, vmid, "scsi0"); err != nil {
		return fmt.Errorf("finalize: reset boot order: %w", err)
	}
	return nil
}

// Finalize is the SingleVMWorkflow finalize entrypoint. It is idempotent and
// safe to re-run on already-finalised VMs.
func (w *SingleVMWorkflow) Finalize(ctx context.Context) error {
	r, err := w.resolve()
	if err != nil {
		return err
	}
	if w.Client == nil {
		return errors.New("finalize: Client not set")
	}
	ip, label := pickFinalizeIP(r.node.IPs)
	if ip == "" {
		return fmt.Errorf("finalize: node %q has no IPs configured (cannot probe SSH-up)", w.NodeName)
	}
	opts := w.FinalizeOptions
	fmt.Fprintf(os.Stderr, "[finalize:%s] waiting for SSH-up at %s (%s, timeout %s)\n",
		w.NodeName, ip, label, opts.timeout())
	if err := waitForSSHUp(ctx, ip, &opts); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[finalize:%s] SSH-up ✓ — detaching ide3+ide2, resetting boot order\n", w.NodeName)
	return FinalizeSingle(ctx, w.Client, r.node.Proxmox.NodeName, r.node.Proxmox.VMID)
}

// nodesForFinalize returns the deterministic sorted node list (mirrors
// MultiNodeWorkflow.nodeNames so it works without exporting that helper).
func (m *MultiNodeWorkflow) nodesForFinalize() ([]string, *config.Hypervisor, error) {
	if m.Config == nil {
		return nil, nil, errors.New("finalize: Config is nil")
	}
	hyp := m.Config.Spec.Hypervisor.Resolved()
	if hyp == nil {
		return nil, nil, errors.New("finalize: hypervisor not resolved")
	}
	names := make([]string, 0, len(hyp.Nodes))
	for n := range hyp.Nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, hyp, nil
}

// Finalize fans the per-node finalize step out across the manifest.
//
// Errors are aggregated so the caller sees every node that failed.
func (m *MultiNodeWorkflow) Finalize(ctx context.Context) error {
	names, _, err := m.nodesForFinalize()
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range names {
		w := m.perNode(name)
		w.FinalizeOptions = m.FinalizeOptions
		if err := w.Finalize(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

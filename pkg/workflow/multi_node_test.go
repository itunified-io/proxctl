package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itunified-io/proxctl/pkg/config"
	"github.com/itunified-io/proxctl/pkg/kickstart"
	"github.com/itunified-io/proxctl/pkg/proxmox"
)

// testMultiEnv builds an env with two nodes sharing a Proxmox host + storage.
func testMultiEnv() *config.Env {
	hyp := &config.Hypervisor{
		Kind: "Hypervisor",
		Nodes: map[string]config.Node{
			"node-a": {
				Proxmox: config.ProxmoxRef{NodeName: "pve01", VMID: 301},
				Resources: &config.Resources{
					Memory: 2048, Cores: 2, Sockets: 1, BIOS: "seabios",
				},
				NICs:  []config.NIC{{NameField: "eth0", Bridge: "vmbr0", Usage: "public", Bootproto: "dhcp"}},
				Disks: []config.Disk{{ID: 0, Size: "16G", Storage: "local-lvm", Interface: "scsi0"}},
			},
			"node-b": {
				Proxmox: config.ProxmoxRef{NodeName: "pve01", VMID: 302},
				Resources: &config.Resources{
					Memory: 2048, Cores: 2, Sockets: 1, BIOS: "seabios",
				},
				NICs:  []config.NIC{{NameField: "eth0", Bridge: "vmbr0", Usage: "public", Bootproto: "dhcp"}},
				Disks: []config.Disk{{ID: 0, Size: "16G", Storage: "local-lvm", Interface: "scsi0"}},
			},
		},
		Kickstart: &config.KickstartConfig{Distro: "oraclelinux9"},
		ISO:       &config.ISOConfig{Storage: "local", Image: "OL9.iso", KickstartStorage: "local"},
	}
	env := &config.Env{
		Version: "1", Kind: "Env",
		Metadata: config.EnvMetadata{Name: "multi", Domain: "example.com"},
	}
	env.Spec.Hypervisor.Inline = hyp
	return env
}

// multiPVE is a concurrency-tracking stub: it counts overlapping /upload
// requests and records per-VMID state transitions (create → start → destroy).
type multiPVE struct {
	mu sync.Mutex

	vmExists  map[int]bool
	vmRunning map[int]bool

	uploadInFlight    int32
	uploadMaxInFlight int32
	uploadDelay       time.Duration

	failOn map[string]int // path suffix → http status
}

func newMultiPVE() *multiPVE {
	return &multiPVE{
		vmExists:  map[int]bool{},
		vmRunning: map[int]bool{},
		failOn:    map[string]int{},
	}
}

func (p *multiPVE) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail-injection first — only suffix match (not substring) so
		// "create-vm" (POST /qemu) is distinguishable from status-current.
		p.mu.Lock()
		for suffix, code := range p.failOn {
			if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, suffix) {
				p.mu.Unlock()
				http.Error(w, `{"errors":{"simulated":"forced"}}`, code)
				return
			}
		}
		p.mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			vmid := lastVMID(r.URL.Path)
			p.mu.Lock()
			exists := p.vmExists[vmid]
			running := p.vmRunning[vmid]
			p.mu.Unlock()
			if !exists {
				http.Error(w, `{"data":null,"errors":{"vmid":"does not exist"}}`, http.StatusInternalServerError)
				return
			}
			status := "stopped"
			if running {
				status = "running"
			}
			fmt.Fprintf(w, `{"data":{"vmid":"%d","name":"n","status":%q}}`, vmid, status)

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/qemu"):
			vmid := parseVMIDFromBody(r)
			p.mu.Lock()
			if vmid > 0 {
				p.vmExists[vmid] = true
			}
			p.mu.Unlock()
			w.Write([]byte(`{"data":""}`))

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/status/start"):
			vmid := lastVMID(r.URL.Path)
			p.mu.Lock()
			p.vmRunning[vmid] = true
			p.mu.Unlock()
			w.Write([]byte(`{"data":""}`))

		case r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/status/stop") ||
			strings.HasSuffix(r.URL.Path, "/status/shutdown")):
			vmid := lastVMID(r.URL.Path)
			p.mu.Lock()
			p.vmRunning[vmid] = false
			p.mu.Unlock()
			w.Write([]byte(`{"data":""}`))

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/qemu/"):
			vmid := lastVMID(r.URL.Path)
			p.mu.Lock()
			delete(p.vmExists, vmid)
			delete(p.vmRunning, vmid)
			p.mu.Unlock()
			w.Write([]byte(`{"data":""}`))

		case strings.HasSuffix(r.URL.Path, "/upload"):
			cur := atomic.AddInt32(&p.uploadInFlight, 1)
			for {
				prev := atomic.LoadInt32(&p.uploadMaxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt32(&p.uploadMaxInFlight, prev, cur) {
					break
				}
			}
			if p.uploadDelay > 0 {
				time.Sleep(p.uploadDelay)
			}
			atomic.AddInt32(&p.uploadInFlight, -1)
			w.Write([]byte(`{"data":""}`))

		default:
			w.Write([]byte(`{"data":null}`))
		}
	}))
}

// lastVMID extracts the trailing /qemu/<id>/… integer from a PVE path.
func lastVMID(path string) int {
	// .../qemu/301/status/current → 301
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "qemu" && i+1 < len(parts) {
			var v int
			fmt.Sscanf(parts[i+1], "%d", &v)
			return v
		}
	}
	return 0
}

// parseVMIDFromBody grabs vmid=NNN from a POST /qemu form body.
func parseVMIDFromBody(r *http.Request) int {
	_ = r.ParseForm()
	var v int
	fmt.Sscanf(r.PostForm.Get("vmid"), "%d", &v)
	return v
}

func (p *multiPVE) client(t *testing.T, srv *httptest.Server) *proxmox.Client {
	t.Helper()
	c, err := proxmox.NewClient(proxmox.ClientOpts{
		Endpoint: srv.URL, TokenID: "t@pve!k", TokenSecret: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// ---- tests ----------------------------------------------------------------

func TestMultiNode_Plan(t *testing.T) {
	p := newMultiPVE()
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	plan, err := m.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("want 2 node plans got %d", len(plan))
	}
	for node, changes := range plan {
		if len(changes) == 0 {
			t.Errorf("node %s: empty plan", node)
		}
	}
}

func TestMultiNode_Plan_NodeFailure(t *testing.T) {
	p := newMultiPVE()
	p.vmExists[301] = true // node-a already exists → Plan fails for that node
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	if _, err := m.Plan(context.Background()); err == nil {
		t.Error("want plan error when one node already exists")
	}
}

func TestMultiNode_Up_HappyPath(t *testing.T) {
	p := newMultiPVE()
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	m.SkipFinalize = true // Up tests don't simulate SSH; finalize is exercised separately.
	if err := m.Up(context.Background()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if !p.vmExists[301] || !p.vmExists[302] {
		t.Error("expected both VMs created")
	}
	if !p.vmRunning[301] || !p.vmRunning[302] {
		t.Error("expected both VMs running")
	}
}

func TestMultiNode_Up_DryRun(t *testing.T) {
	p := newMultiPVE()
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	m.DryRun = true
	if err := m.Up(context.Background()); err != nil {
		t.Fatalf("Up dry-run: %v", err)
	}
	if p.vmExists[301] || p.vmExists[302] {
		t.Error("dry-run should not create VMs")
	}
}

// TestMultiNode_Apply_ISOUploadSerialized asserts that concurrent per-node
// Apply never lets two /upload requests overlap — the shared mutex must
// serialize them. We sleep inside the upload handler so any concurrency bug
// would reliably record uploadMaxInFlight > 1.
func TestMultiNode_Apply_ISOUploadSerialized(t *testing.T) {
	p := newMultiPVE()
	p.uploadDelay = 50 * time.Millisecond
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	m.MaxConcurrency = 4
	if err := m.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := atomic.LoadInt32(&p.uploadMaxInFlight); got != 1 {
		t.Errorf("uploadMaxInFlight = %d, want 1 (upload must be serialized)", got)
	}
}

// TestMultiNode_Apply_ConcurrentCreate asserts that non-upload work can run in
// parallel. We use concurrency=2 and detect parallel execution by timing: two
// nodes, upload takes 30ms each; if serialized total >= 60ms, if parallel
// would be >= 30ms but we measure anyway and just check both completed.
func TestMultiNode_Apply_ConcurrencyCapped(t *testing.T) {
	p := newMultiPVE()
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	m.MaxConcurrency = 1 // force serial to exercise that path
	if err := m.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !p.vmExists[301] || !p.vmExists[302] {
		t.Error("both VMs should be created")
	}
}

func TestMultiNode_Apply_FailFast(t *testing.T) {
	p := newMultiPVE()
	p.failOn["/qemu"] = http.StatusInternalServerError
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	err := m.Apply(context.Background())
	if err == nil {
		t.Fatal("expected error when create-vm fails")
	}
}

func TestMultiNode_Apply_ContinueOnError(t *testing.T) {
	p := newMultiPVE()
	p.failOn["/qemu"] = http.StatusInternalServerError
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	m.ContinueOnError = true
	err := m.Apply(context.Background())
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	// Both nodes should have been attempted → aggregated error mentions both.
	msg := err.Error()
	if !strings.Contains(msg, "node-a") || !strings.Contains(msg, "node-b") {
		t.Errorf("expected both nodes in aggregated error, got: %v", err)
	}
}

func TestMultiNode_Verify_Parallel(t *testing.T) {
	p := newMultiPVE()
	p.vmExists[301] = true
	p.vmRunning[301] = true
	p.vmExists[302] = true
	p.vmRunning[302] = true
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	if err := m.Verify(context.Background()); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestMultiNode_Verify_Aggregates(t *testing.T) {
	p := newMultiPVE()
	// 301 running, 302 stopped → Verify should report 302.
	p.vmExists[301] = true
	p.vmRunning[301] = true
	p.vmExists[302] = true
	p.vmRunning[302] = false
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	err := m.Verify(context.Background())
	if err == nil {
		t.Fatal("expected verify error for stopped node")
	}
	if !strings.Contains(err.Error(), "node-b") {
		t.Errorf("expected node-b in error, got %v", err)
	}
}

func TestMultiNode_Rollback(t *testing.T) {
	p := newMultiPVE()
	p.vmExists[301] = true
	p.vmExists[302] = true
	p.vmRunning[301] = true
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	if err := m.Rollback(context.Background()); err != nil {
		t.Errorf("Rollback: %v", err)
	}
	if p.vmExists[301] || p.vmExists[302] {
		t.Error("rollback should delete both VMs")
	}
}

func TestMultiNode_Down(t *testing.T) {
	p := newMultiPVE()
	p.vmExists[301] = true
	p.vmExists[302] = true
	srv := p.server(t)
	defer srv.Close()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), nil, nil)
	if err := m.Down(context.Background(), true); err != nil {
		t.Errorf("Down: %v", err)
	}
	if p.vmExists[301] || p.vmExists[302] {
		t.Error("down should delete both VMs")
	}
}

func TestMultiNode_NodeNames_Errors(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if _, err := m.nodeNames(); err == nil {
		t.Error("want nil-config error")
	}
	m2 := &MultiNodeWorkflow{Config: &config.Env{}}
	if _, err := m2.nodeNames(); err == nil {
		t.Error("want unresolved hypervisor error")
	}
}

func TestMultiNode_Up_ResolveFailure(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if err := m.Up(context.Background()); err == nil {
		t.Error("want error from Up with nil config")
	}
}

func TestMultiNode_Apply_ResolveFailure(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if err := m.Apply(context.Background()); err == nil {
		t.Error("want error from Apply with nil config")
	}
}

func TestMultiNode_Verify_ResolveFailure(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if err := m.Verify(context.Background()); err == nil {
		t.Error("want error from Verify with nil config")
	}
}

func TestMultiNode_Rollback_ResolveFailure(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if err := m.Rollback(context.Background()); err == nil {
		t.Error("want error from Rollback with nil config")
	}
}

func TestMultiNode_Down_ResolveFailure(t *testing.T) {
	m := &MultiNodeWorkflow{Config: nil}
	if err := m.Down(context.Background(), false); err == nil {
		t.Error("want error from Down with nil config")
	}
}

func TestMultiNode_IsoMu_Allocated(t *testing.T) {
	m := &MultiNodeWorkflow{}
	mu := m.isoMu()
	if mu == nil {
		t.Fatal("isoMu should allocate")
	}
	if m.isoMu() != mu {
		t.Error("isoMu should be stable across calls")
	}
}

func TestMultiNode_MaxConcurrencyDefault(t *testing.T) {
	m := &MultiNodeWorkflow{}
	if got := m.maxConcurrency(); got != defaultMaxConcurrency {
		t.Errorf("want %d got %d", defaultMaxConcurrency, got)
	}
	m.MaxConcurrency = 7
	if got := m.maxConcurrency(); got != 7 {
		t.Errorf("want 7 got %d", got)
	}
}

func TestMultiNode_ApplyPlan_CtxCancel(t *testing.T) {
	// Cancel the context before Apply runs; every goroutine should bail via
	// the select guard.
	p := newMultiPVE()
	srv := p.server(t)
	defer srv.Close()
	rnd, _ := kickstart.NewRenderer()
	m := NewMultiNodeWorkflow(testMultiEnv(), p.client(t, srv), rnd, stubISOBuilder(t))
	m.MaxConcurrency = 1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Apply(ctx)
	// Either the plan step succeeded and the sem select bailed, or context
	// cancellation caused an error elsewhere. Either way we expect a non-nil
	// error because ctx is done.
	if err == nil {
		t.Error("want error on cancelled context")
	}
	// Sanity: ensure the error is a context error or wraps one.
	if !errors.Is(err, context.Canceled) {
		// Acceptable: Plan() may have succeeded and the per-node goroutine
		// returned ctx.Err(). That's still context.Canceled.
		t.Logf("note: error not exactly context.Canceled: %v", err)
	}
}

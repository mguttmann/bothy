// SPDX-License-Identifier: Apache-2.0

package edge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	tclog "github.com/testcontainers/testcontainers-go/log"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The upstream names of the example edge; the responders answer under these aliases.
var nodes = []string{"api", "api-2"}

// nodeHeader names the responder that answered a request.
const nodeHeader = "X-Test-Node"

// The port nginx listens on in every container, as in the example edge.
const nginxPort = "8080/tcp"

const (
	// startTimeout bounds the start of one container, including the image pull.
	startTimeout = 3 * time.Minute
	// requestTimeout bounds one request through the edge.
	requestTimeout = 10 * time.Second
	// sessionCount is the number of session IDs routed; with two nodes the chance that all
	// land on one node is 2^-63.
	sessionCount = 64
	// spreadRequests is the number of requests to a path that must not be pinned to one node.
	spreadRequests = 32
)

// responderConfig is the nginx configuration of a stand-in api node: every request gets 200 and
// a header naming the node.
func responderConfig(node string) string {
	return fmt.Sprintf(`pid /tmp/nginx.pid;
error_log /dev/stderr crit;
events {}
http {
    access_log off;
    client_body_temp_path /tmp/client_temp;
    proxy_temp_path /tmp/proxy_temp;
    fastcgi_temp_path /tmp/fastcgi_temp;
    uwsgi_temp_path /tmp/uwsgi_temp;
    scgi_temp_path /tmp/scgi_temp;
    server {
        listen 8080;
        location / {
            add_header %s %s always;
            return 200 "ok\n";
        }
    }
}
`, nodeHeader, node)
}

// edgeWorld is a running example edge in front of two responders.
type edgeWorld struct {
	edge *testcontainers.DockerContainer
	base string // http://127.0.0.1:<port>
}

// startEdgeWorld starts, on a fresh test network, the responders under the aliases api and
// api-2 and then the edge with infra/compose/edge/nginx.conf, hardened like the Compose service
// edge. Everything is removed in t.Cleanup.
func startEdgeWorld(t *testing.T) *edgeWorld {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*startTimeout)
	defer cancel()
	nw := newTestNetwork(ctx, t)
	dir := configDir(t)
	for _, node := range nodes {
		startResponder(ctx, t, nw, dir, node)
	}
	return startEdge(ctx, t, nw)
}

// newTestNetwork creates a labelled bridge network and removes it after the containers.
func newTestNetwork(ctx context.Context, t *testing.T) *testcontainers.DockerNetwork {
	t.Helper()
	nw, err := tcnetwork.New(ctx, tcnetwork.WithLabels(map[string]string{label: labelValue}))
	if err != nil {
		t.Fatalf("create test network: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := nw.Remove(ctx); err != nil {
			t.Errorf("remove test network: %v", err)
		}
	})
	return nw
}

// configDir is a fresh directory for the responder configurations, readable by the nginx user
// of the containers.
func configDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bothy-edge-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove %s: %v", dir, err)
		}
	})
	// #nosec G302 -- the containers run as another user and only read the files.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// startResponder starts a stand-in api node under the alias node; it publishes no port.
func startResponder(ctx context.Context, t *testing.T, nw *testcontainers.DockerNetwork, dir, node string) {
	t.Helper()
	path := filepath.Join(dir, node+".conf")
	// #nosec G306 -- a test configuration without secrets, read by the container's nginx user.
	if err := os.WriteFile(path, []byte(responderConfig(node)), 0o644); err != nil {
		t.Fatal(err)
	}
	ctr, err := testcontainers.Run(ctx, nginxImage(t),
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithLabels(map[string]string{label: labelValue}),
		tcnetwork.WithNetwork([]string{node}, nw),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.Binds = []string{path + ":/etc/nginx/nginx.conf:ro"}
		}),
		testcontainers.WithWaitStrategy(wait.ForExec([]string{"wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/"}).
			WithStartupTimeout(startTimeout)),
	)
	terminateOnCleanup(t, ctr)
	if err != nil {
		t.Fatalf("start responder %s: %v", node, err)
	}
}

// startEdge starts the example edge on nw with port 8080 published to 127.0.0.1 on a random
// host port: read-only root file system, all capabilities dropped, no-new-privileges and a
// tmpfs on /tmp, as the Compose service edge.
func startEdge(ctx context.Context, t *testing.T, nw *testcontainers.DockerNetwork) *edgeWorld {
	t.Helper()
	conf := filepath.Join(repoRoot(t), "infra", "compose", "edge", "nginx.conf")
	port := network.MustParsePort(nginxPort)
	ctr, err := testcontainers.Run(ctx, nginxImage(t),
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithLabels(map[string]string{label: labelValue}),
		tcnetwork.WithNetwork(nil, nw),
		testcontainers.WithExposedPorts(nginxPort),
		testcontainers.WithTmpfs(map[string]string{"/tmp": ""}),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.Binds = []string{conf + ":/etc/nginx/nginx.conf:ro"}
			hc.ReadonlyRootfs = true
			hc.CapDrop = []string{"ALL"}
			hc.SecurityOpt = []string{"no-new-privileges:true"}
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
		}),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/api/v1/healthz").WithPort(nginxPort).
			WithStartupTimeout(startTimeout)),
	)
	terminateOnCleanup(t, ctr)
	if err != nil {
		t.Fatalf("start edge: %v", err)
	}
	mapped, err := ctr.MappedPort(ctx, nginxPort)
	if err != nil {
		t.Fatalf("edge port: %v", err)
	}
	return &edgeWorld{edge: ctr, base: fmt.Sprintf("http://127.0.0.1:%d", mapped.Num())}
}

// terminateOnCleanup removes ctr when the test ends; Run returns a container also on a failed
// wait, so the caller registers this before checking the error.
func terminateOnCleanup(t *testing.T, ctr *testcontainers.DockerContainer) {
	t.Helper()
	if ctr == nil {
		return
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Errorf("terminate container %s: %v", ctr.GetContainerID(), err)
		}
	})
}

// node sends GET path through the edge and returns the node that answered.
func (w *edgeWorld) node(t *testing.T, path string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.base+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, resp.StatusCode)
	}
	node := resp.Header.Get(nodeHeader)
	if !slices.Contains(nodes, node) {
		t.Fatalf("GET %s: answered by %q, want one of %v", path, node, nodes)
	}
	return node
}

// exec runs cmd in the edge container and returns its exit code and its standard output and
// error, combined.
func (w *edgeWorld) exec(t *testing.T, cmd ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()
	code, r, err := w.edge.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		t.Fatalf("exec %v: %v", cmd, err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("exec %v: read output: %v", cmd, err)
	}
	return code, string(out)
}

// TestEdgeExample_HashRoutesBothStreamPaths proves the hash rule of the example edge (brief
// M2-47 design note 11, R-TM-09): for many UUIDv7 session IDs the device stream and the host
// data connection of a session reach the same node, both nodes occur, and the control
// connection and the gateway are not pinned to one node.
func TestEdgeExample_HashRoutesBothStreamPaths(t *testing.T) {
	t.Parallel()
	w := startEdgeWorld(t)
	seen := map[string]bool{}
	for range sessionCount {
		sid := ids.New().String()
		device := w.node(t, "/api/v1/terminal/sessions/"+sid+"/stream")
		host := w.node(t, "/api/v1/hosts/sessions/"+sid+"/stream")
		if device != host {
			t.Errorf("session %s: device stream on %s, host data connection on %s", sid, device, host)
		}
		seen[device] = true
	}
	if len(seen) != len(nodes) {
		t.Errorf("stream paths reached only %v of %v", seen, nodes)
	}
	for _, path := range []string{"/api/v1/hosts/connect", "/api/v1/gateway"} {
		spread := map[string]bool{}
		for range spreadRequests {
			spread[w.node(t, path)] = true
		}
		if len(spread) != len(nodes) {
			t.Errorf("%s is pinned to %v; it may land on any node", path, spread)
		}
	}
}

// TestEdgeExample_Config checks the example edge in its container (brief M2-47 design note 11):
// nginx -t passes, nginx -T shows the directives of the design note, the timeouts equal the
// defaults in docs/operations/configuration.md, and the container runs read-only, without
// capabilities and not as root.
func TestEdgeExample_Config(t *testing.T) {
	t.Parallel()
	w := startEdgeWorld(t)
	if code, out := w.exec(t, "nginx", "-t"); code != 0 {
		t.Fatalf("nginx -t: exit %d: %s", code, out)
	}
	code, dump := w.exec(t, "sh", "-c", "nginx -T 2>/dev/null")
	if code != 0 {
		t.Fatalf("nginx -T: exit %d", code)
	}
	conf, err := parseNginx(dump)
	if err != nil {
		t.Fatalf("parse nginx -T: %v", err)
	}
	checkMain(t, conf)
	httpCtx := one(t, conf, "http")
	checkHTTP(t, httpCtx)
	checkUpstreams(t, httpCtx)
	checkLocations(t, one(t, httpCtx.block, "server"))
	checkTimeouts(t, conf)
	checkHardening(t, w)
}

// checkMain checks the main context: pid under /tmp and only critical errors to stderr.
func checkMain(t *testing.T, conf []*directive) {
	t.Helper()
	if pid := one(t, conf, "pid"); !strings.HasPrefix(arg(t, pid, 0), "/tmp/") {
		t.Errorf("pid %v is not under /tmp", pid.args)
	}
	wantArgs(t, one(t, conf, "error_log"), "/dev/stderr", "crit")
}

// checkHTTP checks the http context: no access log, no body limit, temp paths under /tmp and
// the map of the session ID.
func checkHTTP(t *testing.T, httpCtx *directive) {
	t.Helper()
	for _, d := range walk(httpCtx.block) {
		if d.name == "access_log" && !slices.Equal(d.args, []string{"off"}) {
			t.Errorf("access_log %v; the edge writes no access log", d.args)
		}
		if strings.HasSuffix(d.name, "_temp_path") && !strings.HasPrefix(arg(t, d, 0), "/tmp/") {
			t.Errorf("%s %v is not under /tmp", d.name, d.args)
		}
	}
	wantArgs(t, one(t, httpCtx.block, "access_log"), "off")
	wantArgs(t, one(t, httpCtx.block, "client_max_body_size"), "0")
	var sessionMap *directive
	for _, m := range find(httpCtx.block, "map") {
		if slices.Equal(m.args, []string{"$uri", "$bothy_terminal_session"}) {
			sessionMap = m
		}
	}
	if sessionMap == nil {
		t.Fatal("no map $uri $bothy_terminal_session")
	}
	want := "~^/api/v1/(?:terminal|hosts)/sessions/(?<sid>[^/]+)/stream$"
	if !slices.ContainsFunc(sessionMap.block, func(d *directive) bool { return d.name == want }) {
		t.Errorf("map $uri $bothy_terminal_session lacks the entry %s", want)
	}
}

// checkUpstreams checks that only bothy_terminal_stream hashes, consistently over the session
// ID, and that both upstreams name both nodes.
func checkUpstreams(t *testing.T, httpCtx *directive) {
	t.Helper()
	ups := map[string]*directive{}
	for _, u := range find(httpCtx.block, "upstream") {
		ups[arg(t, u, 0)] = u
	}
	for _, name := range []string{"bothy_terminal_stream", "bothy_api"} {
		u, ok := ups[name]
		if !ok {
			t.Fatalf("no upstream %s", name)
		}
		var servers []string
		for _, s := range find(u.block, "server") {
			servers = append(servers, arg(t, s, 0))
		}
		if !slices.Equal(servers, []string{"api:8080", "api-2:8080"}) {
			t.Errorf("upstream %s servers %v, want api:8080 and api-2:8080", name, servers)
		}
	}
	for name, u := range ups {
		hashes := find(u.block, "hash")
		if name == "bothy_terminal_stream" {
			if len(hashes) != 1 || !slices.Equal(hashes[0].args, []string{"$bothy_terminal_session", "consistent"}) {
				t.Errorf("upstream %s: hash directives %v, want one hash $bothy_terminal_session consistent", name, hashes)
			}
			continue
		}
		if len(hashes) != 0 {
			t.Errorf("upstream %s hashes; only bothy_terminal_stream may", name)
		}
	}
}

// streamLocation is the location argument of the stream paths in the example edge.
const streamLocation = "^/api/v1/(?:terminal|hosts)/sessions/[^/]+/stream$"

// checkLocations checks listen 8080 and the proxy directives of both locations.
func checkLocations(t *testing.T, server *directive) {
	t.Helper()
	wantArgs(t, one(t, server.block, "listen"), "8080")
	stream, root := locations(t, server)
	for _, loc := range []*directive{stream, root} {
		wantArgs(t, one(t, loc.block, "proxy_http_version"), "1.1")
		headers := map[string]string{}
		for _, h := range find(loc.block, "proxy_set_header") {
			headers[arg(t, h, 0)] = arg(t, h, 1)
		}
		want := map[string]string{
			"Upgrade":         "$http_upgrade",
			"Connection":      "$connection_upgrade",
			"Host":            "$http_host",
			"X-Forwarded-For": "$proxy_add_x_forwarded_for",
		}
		for name, value := range want {
			if headers[name] != value {
				t.Errorf("location %v: proxy_set_header %s %q, want %q", loc.args, name, headers[name], value)
			}
		}
	}
	wantArgs(t, one(t, stream.block, "proxy_pass"), "http://bothy_terminal_stream")
	wantArgs(t, one(t, stream.block, "proxy_buffering"), "off")
	wantArgs(t, one(t, stream.block, "proxy_request_buffering"), "off")
	wantArgs(t, one(t, root.block, "proxy_pass"), "http://bothy_api")
	// Bodies and responses stream through, so nothing fills the tmpfs /tmp before the server
	// applies its limits.
	wantArgs(t, one(t, root.block, "proxy_request_buffering"), "off")
	wantArgs(t, one(t, root.block, "proxy_max_temp_file_size"), "0")
}

// locations returns the stream location and location /, the only two.
func locations(t *testing.T, server *directive) (stream, root *directive) {
	t.Helper()
	locs := find(server.block, "location")
	if len(locs) != 2 {
		t.Fatalf("%d locations, want the stream location and /", len(locs))
	}
	for _, loc := range locs {
		switch {
		case slices.Equal(loc.args, []string{"~", streamLocation}):
			stream = loc
		case slices.Equal(loc.args, []string{"/"}):
			root = loc
		}
	}
	if stream == nil || root == nil {
		t.Fatalf("locations %v and %v, want ~ %s and /", locs[0].args, locs[1].args, streamLocation)
	}
	return stream, root
}

// timeoutDirectives are the idle timeouts of a proxied connection.
var timeoutDirectives = []string{"proxy_read_timeout", "proxy_send_timeout"}

// checkTimeouts compares the timeouts with the defaults in docs/operations/configuration.md:
// the stream location keeps terminal.idle_timeout plus terminal.idle_grace, every other
// timeout is terminal.host_heartbeat_interval times terminal.host_heartbeat_miss_limit, which
// lies above sync.heartbeat_interval.
func checkTimeouts(t *testing.T, conf []*directive) {
	t.Helper()
	streamWant := configDuration(t, "terminal.idle_timeout") + configDuration(t, "terminal.idle_grace")
	misses, err := strconv.Atoi(configDefault(t, "terminal.host_heartbeat_miss_limit"))
	if err != nil {
		t.Fatalf("default of terminal.host_heartbeat_miss_limit: %v", err)
	}
	otherWant := configDuration(t, "terminal.host_heartbeat_interval") * time.Duration(misses)
	if gateway := configDuration(t, "sync.heartbeat_interval"); otherWant <= gateway {
		t.Errorf("control connection timeout %s is not above sync.heartbeat_interval %s", otherWant, gateway)
	}
	stream, root := locations(t, one(t, one(t, conf, "http").block, "server"))
	for _, name := range timeoutDirectives {
		if got := nginxDuration(t, one(t, stream.block, name)); got != streamWant {
			t.Errorf("stream location %s %s, want %s", name, got, streamWant)
		}
		one(t, root.block, name)
	}
	for _, d := range walk(conf) {
		if !slices.Contains(timeoutDirectives, d.name) || slices.Contains(stream.block, d) {
			continue
		}
		if got := nginxDuration(t, d); got != otherWant {
			t.Errorf("%s %s outside the stream location, want %s", d.name, got, otherWant)
		}
	}
}

// nginxDuration reads the single time argument of d; the example writes seconds or minutes,
// which time.ParseDuration reads as nginx does.
func nginxDuration(t *testing.T, d *directive) time.Duration {
	t.Helper()
	v, err := time.ParseDuration(arg(t, d, 0))
	if err != nil || len(d.args) != 1 {
		t.Fatalf("%s %v: not a single duration in seconds or minutes", d.name, d.args)
	}
	return v
}

// checkHardening checks that the edge container runs read-only, without capabilities and not
// as root.
func checkHardening(t *testing.T, w *edgeWorld) {
	t.Helper()
	info, err := w.edge.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !info.HostConfig.ReadonlyRootfs {
		t.Error("the edge root file system is writable")
	}
	if !slices.Equal(info.HostConfig.CapDrop, []string{"ALL"}) {
		t.Errorf("cap_drop %v, want [ALL]", info.HostConfig.CapDrop)
	}
	code, out := w.exec(t, "id", "-u")
	if uid := strings.TrimSpace(out); code != 0 || uid == "" || uid == "0" {
		t.Errorf("id -u: exit %d, uid %q; the edge must not run as root", code, uid)
	}
}

// wantArgs fails the test unless d has exactly the arguments want.
func wantArgs(t *testing.T, d *directive, want ...string) {
	t.Helper()
	if !slices.Equal(d.args, want) {
		t.Errorf("%s %v, want %v", d.name, d.args, want)
	}
}

// directive is one nginx directive with its arguments and, for a block, its children.
type directive struct {
	name  string
	args  []string
	block []*directive
}

// parseNginx parses the output of nginx -T into directives. It knows what the example edge
// uses: comments, double-quoted strings, blocks and simple directives.
func parseNginx(text string) ([]*directive, error) {
	toks, err := tokenize(text)
	if err != nil {
		return nil, err
	}
	dirs, rest, err := parseBlock(toks)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("unexpected %q", rest[0].text)
	}
	return dirs, nil
}

// token is a word of the configuration; special marks one of { } ; outside quotes.
type token struct {
	text    string
	special bool
}

// tokenize splits text into words and the marks { } ;, dropping comments and the quotes of
// double-quoted strings.
func tokenize(text string) ([]token, error) {
	var toks []token
	for i := 0; i < len(text); {
		c := text[i]
		switch c {
		case '#':
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case ' ', '\t', '\n', '\r':
			i++
		case '{', '}', ';':
			toks = append(toks, token{text: string(c), special: true})
			i++
		case '"':
			end := strings.IndexByte(text[i+1:], '"')
			if end < 0 {
				return nil, fmt.Errorf("unterminated string at offset %d", i)
			}
			toks = append(toks, token{text: text[i+1 : i+1+end]})
			i += end + 2
		default:
			j := i
			for j < len(text) && !strings.ContainsRune(" \t\n\r{};", rune(text[j])) {
				j++
			}
			toks = append(toks, token{text: text[i:j]})
			i = j
		}
	}
	return toks, nil
}

// parseBlock parses directives until a closing brace or the end and returns the tokens after
// the closing brace, which it consumes.
func parseBlock(toks []token) ([]*directive, []token, error) {
	var dirs []*directive
	for len(toks) > 0 {
		if toks[0].special && toks[0].text == "}" {
			return dirs, toks[1:], nil
		}
		if toks[0].special {
			return nil, nil, fmt.Errorf("unexpected %q", toks[0].text)
		}
		d := &directive{name: toks[0].text}
		toks = toks[1:]
		for len(toks) > 0 && !toks[0].special {
			d.args = append(d.args, toks[0].text)
			toks = toks[1:]
		}
		if len(toks) == 0 {
			return nil, nil, fmt.Errorf("directive %s is not terminated", d.name)
		}
		open := toks[0].text == "{"
		toks = toks[1:]
		if open {
			var err error
			if d.block, toks, err = parseBlock(toks); err != nil {
				return nil, nil, err
			}
		}
		dirs = append(dirs, d)
	}
	return dirs, toks, nil
}

// find returns the directives called name directly in dirs.
func find(dirs []*directive, name string) []*directive {
	var out []*directive
	for _, d := range dirs {
		if d.name == name {
			out = append(out, d)
		}
	}
	return out
}

// one returns the only directive called name directly in dirs and fails the test otherwise.
func one(t *testing.T, dirs []*directive, name string) *directive {
	t.Helper()
	found := find(dirs, name)
	if len(found) != 1 {
		t.Fatalf("%d directives %s, want one", len(found), name)
	}
	return found[0]
}

// walk returns every directive in dirs and in their blocks, depth first.
func walk(dirs []*directive) []*directive {
	var out []*directive
	for _, d := range dirs {
		out = append(out, d)
		out = append(out, walk(d.block)...)
	}
	return out
}

// arg returns the argument i of d and fails the test when d has fewer.
func arg(t *testing.T, d *directive, i int) string {
	t.Helper()
	if len(d.args) <= i {
		t.Fatalf("%s %v has no argument %d", d.name, d.args, i)
	}
	return d.args[i]
}

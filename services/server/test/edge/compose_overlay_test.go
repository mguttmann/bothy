// SPDX-License-Identifier: Apache-2.0

package edge

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// composeTimeout bounds one docker compose config call.
const composeTimeout = 2 * time.Minute

// overlayFile is the overlay under test, relative to the repository root.
const overlayFile = "infra/compose/compose.multi-node.yaml"

// service is one service of the rendered Compose model, as docker compose config --format json
// writes it.
type service map[string]any

// composeConfig renders compose.yaml with the overlay, .env.example and only the profile
// profile, and returns the services. --profile replaces COMPOSE_PROFILES of the env file.
func composeConfig(t *testing.T, profile string) map[string]service {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), composeTimeout)
	defer cancel()
	//nolint:gosec // G204: fixed arguments; running the Docker CLI is the test.
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"--project-directory", "infra/compose",
		"-f", "infra/compose/compose.yaml", "-f", overlayFile,
		"--env-file", "infra/compose/.env.example",
		"--profile", profile, "config", "--format", "json")
	cmd.Dir = repoRoot(t)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker compose config (profile %s): %v: %s", profile, err, errOut.String())
	}
	var model struct {
		Services map[string]service `json:"services"`
	}
	if err := json.Unmarshal(out.Bytes(), &model); err != nil {
		t.Fatalf("decode docker compose config: %v", err)
	}
	return model.Services
}

// sameAsAPI are the fields api-2 takes over from api through extends (brief M2-47 design note
// 12).
var sameAsAPI = []string{
	"image", "command", "volumes", "read_only", "cap_drop", "security_opt", "tmpfs",
	"healthcheck", "stop_grace_period", "depends_on", "networks",
}

// TestComposeMultiNodeOverlay renders the overlay with .env.example (brief M2-47 design note 12,
// requirement 21): two equal api nodes in mode multi without ports, the worker in mode multi,
// the hardened edge on 127.0.0.1 only with the pinned nginx tag, and nothing of it with the
// profile tailscale.
func TestComposeMultiNodeOverlay(t *testing.T) {
	t.Parallel()
	t.Run("direct", func(t *testing.T) {
		t.Parallel()
		services := composeConfig(t, "direct")
		checkAPINodes(t, services)
		checkClusterMode(t, services, "api", "api-2", "worker")
		checkEdgeService(t, services)
		checkLoopbackPorts(t, services)
	})
	t.Run("tailscale", func(t *testing.T) {
		t.Parallel()
		services := composeConfig(t, "tailscale")
		for _, name := range []string{"edge", "api-2"} {
			if _, ok := services[name]; ok {
				t.Errorf("profile tailscale renders the service %s; it runs one api node", name)
			}
		}
		checkLoopbackPorts(t, services)
	})
}

// checkAPINodes compares api-2 with api and checks that neither publishes a port and both
// mount the shared signing-keys store.
func checkAPINodes(t *testing.T, services map[string]service) {
	t.Helper()
	api, api2 := mustService(t, services, "api"), mustService(t, services, "api-2")
	for _, field := range sameAsAPI {
		if api[field] == nil {
			t.Errorf("api has no %s", field)
		}
		if !reflect.DeepEqual(api[field], api2[field]) {
			t.Errorf("api-2 %s = %v, want %v as api", field, api2[field], api[field])
		}
	}
	for name, svc := range map[string]service{"api": api, "api-2": api2} {
		if ports, _ := svc["ports"].([]any); len(ports) != 0 {
			t.Errorf("%s publishes %v; only the edge publishes a port", name, ports)
		}
		if !mountsVolume(svc, "signing-keys") {
			t.Errorf("%s does not mount the shared key store signing-keys", name)
		}
	}
}

// mountsVolume reports whether svc mounts the named volume.
func mountsVolume(svc service, name string) bool {
	vols, _ := svc["volumes"].([]any)
	return slices.ContainsFunc(vols, func(v any) bool {
		m, _ := v.(map[string]any)
		return m["type"] == "volume" && m["source"] == name
	})
}

// checkClusterMode checks BOTHY_CLUSTER__MODE=multi on the named services.
func checkClusterMode(t *testing.T, services map[string]service, names ...string) {
	t.Helper()
	for _, name := range names {
		env, _ := mustService(t, services, name)["environment"].(map[string]any)
		if env["BOTHY_CLUSTER__MODE"] != "multi" {
			t.Errorf("%s BOTHY_CLUSTER__MODE = %v, want multi", name, env["BOTHY_CLUSTER__MODE"])
		}
	}
}

// checkEdgeService checks the Compose service edge against design note 12.
func checkEdgeService(t *testing.T, services map[string]service) {
	t.Helper()
	edge := mustService(t, services, "edge")
	if got, want := edge["image"], nginxImage(t); got != want {
		t.Errorf("edge image %v, want %s", got, want)
	}
	checkNginxPins(t)
	checkEdgePorts(t, edge)
	checkEdgeHardening(t, edge, mustService(t, services, "api"))
	if nets, _ := edge["networks"].(map[string]any); len(nets) != 1 || !hasKey(nets, "default") {
		t.Errorf("edge networks %v, want only default", edge["networks"])
	}
	if !slices.Equal(stringList(edge["profiles"]), []string{"direct"}) {
		t.Errorf("edge profiles %v, want [direct]", edge["profiles"])
	}
	deps, _ := edge["depends_on"].(map[string]any)
	for _, node := range []string{"api", "api-2"} {
		dep, _ := deps[node].(map[string]any)
		// restart: nginx resolves the nodes only at start, so a recreated node restarts the edge.
		if dep["condition"] != "service_healthy" || dep["restart"] != true {
			t.Errorf("edge depends_on %s %v, want condition service_healthy with restart", node, deps[node])
		}
	}
	checkEdgeMount(t, edge)
}

// checkNginxPins keeps the tag in .env.example equal to the default in the overlay and to the
// row of docs/operations/configuration.md section 9.
func checkNginxPins(t *testing.T) {
	t.Helper()
	tag := envExampleValue(t, "BOTHY_NGINX_VERSION")
	m := regexp.MustCompile(`image: nginxinc/nginx-unprivileged:\$\{BOTHY_NGINX_VERSION:-([^}]+)\}`).
		FindAllStringSubmatch(readRepoFile(t, overlayFile), -1)
	if len(m) != 1 || m[0][1] != tag {
		t.Errorf("%s: image defaults %v, want one default %s as in .env.example", overlayFile, m, tag)
	}
	if doc := tableCell(t, "docs/operations/configuration.md", "BOTHY_NGINX_VERSION", 1); doc != tag {
		t.Errorf("configuration.md section 9 BOTHY_NGINX_VERSION %q, want %q", doc, tag)
	}
}

// checkEdgePorts checks that the edge publishes only 127.0.0.1:<BOTHY_HTTP_PUBLISH_PORT>:8080.
func checkEdgePorts(t *testing.T, edge service) {
	t.Helper()
	ports, _ := edge["ports"].([]any)
	want := map[string]any{
		"host_ip": "127.0.0.1", "published": envExampleValue(t, "BOTHY_HTTP_PUBLISH_PORT"),
		"target": float64(8080), "protocol": "tcp",
	}
	if len(ports) != 1 {
		t.Fatalf("edge ports %v, want exactly one", ports)
	}
	p, _ := ports[0].(map[string]any)
	for k, v := range want {
		if p[k] != v {
			t.Errorf("edge port %s = %v, want %v", k, p[k], v)
		}
	}
}

// checkEdgeHardening checks read_only, cap_drop, no-new-privileges, the tmpfs and the log
// driver of api.
func checkEdgeHardening(t *testing.T, edge, api service) {
	t.Helper()
	if edge["read_only"] != true {
		t.Error("edge is not read_only")
	}
	for field, want := range map[string][]string{
		"cap_drop":     {"ALL"},
		"security_opt": {"no-new-privileges:true"},
		"tmpfs":        {"/tmp"},
	} {
		if got := stringList(edge[field]); !slices.Equal(got, want) {
			t.Errorf("edge %s %v, want %v", field, got, want)
		}
	}
	if !reflect.DeepEqual(edge["logging"], api["logging"]) {
		t.Errorf("edge logging %v, want %v as the services of compose.yaml", edge["logging"], api["logging"])
	}
}

// checkEdgeMount checks that the edge mounts ./edge/nginx.conf read-only as its nginx.conf.
func checkEdgeMount(t *testing.T, edge service) {
	t.Helper()
	vols, _ := edge["volumes"].([]any)
	if len(vols) != 1 {
		t.Fatalf("edge volumes %v, want the configuration only", vols)
	}
	m, _ := vols[0].(map[string]any)
	src, _ := m["source"].(string)
	if m["type"] != "bind" || !strings.HasSuffix(src, "/infra/compose/edge/nginx.conf") ||
		m["target"] != "/etc/nginx/nginx.conf" || m["read_only"] != true {
		t.Errorf("edge volume %v, want ./edge/nginx.conf read-only at /etc/nginx/nginx.conf", m)
	}
}

// checkLoopbackPorts checks that every published port of every service binds 127.0.0.1.
func checkLoopbackPorts(t *testing.T, services map[string]service) {
	t.Helper()
	for name, svc := range services {
		ports, _ := svc["ports"].([]any)
		for _, p := range ports {
			if m, _ := p.(map[string]any); m["host_ip"] != "127.0.0.1" {
				t.Errorf("%s publishes %v beyond 127.0.0.1", name, p)
			}
		}
	}
}

// mustService returns the named service and fails the test when it is not rendered.
func mustService(t *testing.T, services map[string]service, name string) service {
	t.Helper()
	svc, ok := services[name]
	if !ok {
		t.Fatalf("service %s is not rendered", name)
	}
	return svc
}

// stringList converts a JSON list of strings; anything else gives nil.
func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil
		}
		out = append(out, s)
	}
	return out
}

// hasKey reports whether m has the key k.
func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

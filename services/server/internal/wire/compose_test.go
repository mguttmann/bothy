// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestComposeStopGraceAboveTotal holds 02 "Shutdown": the orchestrator waits longer than the
// server's whole shutdown budget, so Compose never kills a server that is still within
// shutdown.total_timeout. Every service of infra/compose/compose.yaml that runs `serve` needs a
// stop_grace_period above the default of the key (check D8 of make config-doc-check holds the
// same relation with the forms of entrypoint and command; this test reads the real file).
func TestComposeStopGraceAboveTotal(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Command         []string `yaml:"command"`
			StopGracePeriod string   `yaml:"stop_grace_period"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	total := v.Duration("shutdown.total_timeout")
	servers := 0
	for name, svc := range doc.Services {
		if len(svc.Command) == 0 || svc.Command[0] != "serve" {
			continue
		}
		servers++
		grace, err := time.ParseDuration(svc.StopGracePeriod)
		if err != nil {
			t.Errorf("%s: stop_grace_period %q: %v", name, svc.StopGracePeriod, err)
			continue
		}
		if grace <= total {
			t.Errorf("%s: stop_grace_period %s is not above shutdown.total_timeout %s", name, grace, total)
		}
	}
	if servers < 2 || !strings.Contains(string(raw), "--role=worker") {
		t.Fatalf("found %d server services; the test no longer reads the file it should", servers)
	}
}

// TestComposeTrustedProxiesPassthrough holds Scope 6 of brief M1-21 (16 "4.3 Edge and
// identity"): service api of profile direct passes server.trusted_proxies from the Compose
// variable BOTHY_TRUSTED_PROXIES with the loopback default of the key, so the operator can name
// the gateway of the Compose network that the reverse proxy appears as. Profile tailscale passes
// nothing: the server listens on loopback in the namespace of the sidecar, and the anchor
// x-server-env, which every server service merges, does not set the key either. The default in
// the file and in .env.example loads to the default of the schema.
func TestComposeTrustedProxiesPassthrough(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ServerEnv map[string]string `yaml:"x-server-env"`
		Services  map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	const (
		key  = "BOTHY_SERVER__TRUSTED_PROXIES"
		want = "${BOTHY_TRUSTED_PROXIES:-127.0.0.1/32,::1/128}"
	)
	api, ok := doc.Services["api"]
	if !ok || api.Environment[key] != want {
		t.Fatalf("service api: %s = %q, want %q", key, api.Environment[key], want)
	}
	if v, ok := doc.Services["api-tailscale"].Environment[key]; ok {
		t.Errorf("service api-tailscale sets %s = %q; the sidecar profile keeps the loopback default", key, v)
	}
	if v, ok := doc.ServerEnv[key]; ok {
		t.Errorf("x-server-env sets %s = %q; only service api passes it", key, v)
	}
	example, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	var exampleValue string
	for line := range strings.Lines(string(example)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "BOTHY_TRUSTED_PROXIES="); ok {
			exampleValue = v
		}
	}
	defaults, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	fallback := strings.TrimSuffix(strings.TrimPrefix(want, "${BOTHY_TRUSTED_PROXIES:-"), "}")
	for name, value := range map[string]string{"compose.yaml fallback": fallback, ".env.example": exampleValue} {
		v, err := config.Load(schema(t), config.Sources{Env: append(testEnv(), key+"="+value)})
		if err != nil {
			t.Fatalf("%s %q: %v", name, value, err)
		}
		if got, def := v.Text("server.trusted_proxies"), defaults.Text("server.trusted_proxies"); got != def {
			t.Errorf("%s loads to %q, want the schema default %q", name, got, def)
		}
	}
}

// TestComposePushPassthrough holds the way a Compose installation switches push on (13 "Push keys
// and white label", docs/operations/install-compose.md section 9). The anchor x-server-env, which
// api and worker merge, passes push.apns.team_id, push.apns.key_id, push.apns.topics and
// push.fcm.project_id from Compose variables, and names the supplied file apns_key.p8 in the
// secrets directory of the server services as push.apns.private_key_ref, which has no default.
// With the empty values of .env.example the configuration loads with both providers off, so an
// installation without push starts as before; with the variables set it loads with both on, and
// the service account of FCM is its default file in the same directory.
func TestComposePushPassthrough(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ServerEnv map[string]string `yaml:"x-server-env"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	// Server variable to the Compose variable of .env that fills it.
	variables := map[string]string{
		"BOTHY_PUSH__APNS__TEAM_ID":   "BOTHY_PUSH_APNS_TEAM_ID",
		"BOTHY_PUSH__APNS__KEY_ID":    "BOTHY_PUSH_APNS_KEY_ID",
		"BOTHY_PUSH__APNS__TOPICS":    "BOTHY_PUSH_APNS_TOPICS",
		"BOTHY_PUSH__FCM__PROJECT_ID": "BOTHY_PUSH_FCM_PROJECT_ID",
	}
	for server, variable := range variables {
		if got, want := doc.ServerEnv[server], "${"+variable+":-}"; got != want {
			t.Errorf("x-server-env: %s = %q, want %q", server, got, want)
		}
	}
	secretsDir := doc.ServerEnv["BOTHY_SECRETS__DIR"]
	const keyRefName = "BOTHY_PUSH__APNS__PRIVATE_KEY_REF"
	keyRef := "file:" + secretsDir + "/apns_key.p8"
	if got := doc.ServerEnv[keyRefName]; secretsDir == "" || got != keyRef {
		t.Fatalf("x-server-env: %s = %q, want %q (apns_key.p8 in BOTHY_SECRETS__DIR)", keyRefName, got, keyRef)
	}

	example, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	template := map[string]string{}
	for line := range strings.Lines(string(example)) {
		if name, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(name, "#") {
			template[name] = value
		}
	}
	// tryLoad reads the configuration of a server service whose Compose variables have the values.
	tryLoad := func(values map[string]string) (*config.Values, error) {
		env := append(testEnv(), "BOTHY_SECRETS__DIR="+secretsDir, keyRefName+"="+keyRef)
		for server, variable := range variables {
			env = append(env, server+"="+values[variable])
		}
		return config.Load(schema(t), config.Sources{Env: env})
	}
	load := func(values map[string]string) *config.Values {
		v, err := tryLoad(values)
		if err != nil {
			t.Fatalf("Compose variables %v: %v", values, err)
		}
		return v
	}

	for _, variable := range variables {
		if value, ok := template[variable]; !ok || value != "" {
			t.Errorf(".env.example: %s = %q (present %t), want the variable with an empty value", variable, value, ok)
		}
	}
	off := load(template)
	if apns, err := wire.APNsConfig(off); err != nil || apns.Enabled() {
		t.Errorf("the values of .env.example enable APNs (error %v)", err)
	}
	if fcm, err := wire.FCMConfig(off); err != nil || fcm.Enabled() {
		t.Errorf("the values of .env.example enable FCM (error %v)", err)
	}

	on := load(map[string]string{
		"BOTHY_PUSH_APNS_TEAM_ID":   "EXAMPLETEAM",
		"BOTHY_PUSH_APNS_KEY_ID":    "EXAMPLEKEY",
		"BOTHY_PUSH_APNS_TOPICS":    "com.example.chat,com.example.chat.mac",
		"BOTHY_PUSH_FCM_PROJECT_ID": "example-chat",
	})
	apns, err := wire.APNsConfig(on)
	if err != nil || !apns.Enabled() || apns.PrivateKeyRef != keyRef || len(apns.Topics) != 2 {
		t.Errorf("with the APNs variables set: enabled %t, key reference %q, topics %v, error %v; want APNs on with %s and two topics",
			apns.Enabled(), apns.PrivateKeyRef, apns.Topics, err, keyRef)
	}
	fcm, err := wire.FCMConfig(on)
	if want := "file:" + secretsDir + "/fcm_service_account.json"; err != nil || !fcm.Enabled() || fcm.ServiceAccountRef != want {
		t.Errorf("with the FCM variable set: enabled %t, service account %q, error %v; want FCM on with %s",
			fcm.Enabled(), fcm.ServiceAccountRef, err, want)
	}

	// The three APNs values go together (cross-check push_apns_complete): the guide says that a
	// partial set stops the start.
	if _, err := tryLoad(map[string]string{"BOTHY_PUSH_APNS_TEAM_ID": "EXAMPLETEAM"}); err == nil || !strings.Contains(err.Error(), "push.apns.team_id") {
		t.Errorf("with only the APNs team ID set: error %v, want the start refused naming the APNs keys", err)
	}
}

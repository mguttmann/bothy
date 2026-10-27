// SPDX-License-Identifier: Apache-2.0

package fakepush_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/test/fakepush"
)

// TestOverlayMatchesTheFake (brief M5-11 Design note 1): the Compose overlay configures api and
// worker with the example identity the fake verifies, reads the key, the service account and
// the CA where the fake writes them, and reaches the fake under the host name and ports it
// starts it with; a drift would only show as a refused provider token in a device run.
func TestOverlayMatchesTheFake(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "test", "compose.e2e-mobile.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
			Command     []string          `yaml:"command"`
			Volumes     []string          `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	fake := doc.Services["fakepush"]
	wantCmd := []string{"serve", "--dir", "/push", "--host", "fakepush", "--apns-addr", ":8443", "--fcm-addr", ":8444", "--capture-addr", ":8090"}
	if !slices.Equal(fake.Command, wantCmd) || !slices.Equal(fake.Volumes, []string{"fakepush-run:/push"}) {
		t.Fatalf("fakepush command %v volumes %v", fake.Command, fake.Volumes)
	}
	const dir = "/run/bothy-e2e/push/"
	for _, name := range []string{"api", "worker"} {
		svc := doc.Services[name]
		env := svc.Environment
		want := map[string]string{
			"BOTHY_PUSH__APNS__TEAM_ID":            fakepush.APNsTeamID,
			"BOTHY_PUSH__APNS__KEY_ID":             fakepush.APNsKeyID,
			"BOTHY_PUSH__APNS__PRIVATE_KEY_REF":    "file:" + dir + fakepush.FileAPNsKey,
			"BOTHY_PUSH__FCM__PROJECT_ID":          fakepush.FCMProjectID,
			"BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF": "file:" + dir + fakepush.FileFCMAccount,
			"SSL_CERT_FILE":                        dir + fakepush.FileCA,
		}
		for k, v := range want {
			if env[k] != v {
				t.Errorf("%s: %s = %q, want %q", name, k, env[k], v)
			}
		}
		var urls []string
		for _, v := range env {
			urls = append(urls, v)
		}
		for _, u := range []string{"https://fakepush:8443", "https://fakepush:8444"} {
			if !slices.Contains(urls, u) {
				t.Errorf("%s: no variable names the fake at %s", name, u)
			}
		}
		if !slices.Contains(svc.Volumes, "fakepush-run:"+dir[:len(dir)-1]+":ro") {
			t.Errorf("%s: volumes %v lack the read-only run volume", name, svc.Volumes)
		}
	}
}

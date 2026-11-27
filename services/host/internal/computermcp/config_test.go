// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	dir := t.TempDir()
	base := map[string]string{
		EnvDir: dir, EnvNonce: tNonce, EnvTimeoutMS: "30000", EnvPins: dir + "/computer/pins.json", EnvServer: "https://bothy.example.com",
	}
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cfg, err := ConfigFromEnv(get(base))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dir != dir || cfg.Nonce != tNonce || cfg.Timeout != 30*time.Second || cfg.Server.String() != "https://bothy.example.com" {
		t.Errorf("config %+v", cfg)
	}
	for name, change := range map[string][2]string{
		"no dir":           {EnvDir, ""},
		"relative dir":     {EnvDir, "requests"},
		"no nonce":         {EnvNonce, ""},
		"no timeout":       {EnvTimeoutMS, ""},
		"zero timeout":     {EnvTimeoutMS, "0"},
		"text timeout":     {EnvTimeoutMS, "30s"},
		"relative pins":    {EnvPins, "pins.json"},
		"no server":        {EnvServer, ""},
		"http elsewhere":   {EnvServer, "http://bothy.example.com"},
		"other scheme":     {EnvServer, "ftp://bothy.example.com"},
		"server with path": {EnvServer, "https://bothy.example.com/x?y=1"},
	} {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		m[change[0]] = change[1]
		if _, err := ConfigFromEnv(get(m)); err == nil || !strings.Contains(err.Error(), change[0]) {
			t.Errorf("%s: %v", name, err)
		}
	}
	m := map[string]string{}
	for k, v := range base {
		m[k] = v
	}
	m[EnvServer] = "http://127.0.0.1:8443"
	if _, err := ConfigFromEnv(get(m)); err != nil {
		t.Errorf("http on 127.0.0.1: %v", err)
	}
}

// TestCAFileJoinsTheSystemRoots holds D-CU-06-12: without BOTHY_COMPUTER_CA_FILE the attach client
// knows only the system roots and refuses a server of a private CA; with the CA file it connects.
func TestCAFileJoinsTheSystemRoots(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca-bundle.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		EnvDir: dir, EnvNonce: tNonce, EnvTimeoutMS: "30000", EnvPins: dir + "/pins.json", EnvServer: "https://bothy.example.com",
	}
	get := func(k string) string { return env[k] }

	cfg, err := ConfigFromEnv(get)
	if err != nil || cfg.RootCAs != nil {
		t.Fatalf("without the variable: %v %v", cfg.RootCAs, err)
	}
	get0 := func(roots *x509.CertPool) (*http.Response, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		return newHTTPClient(roots).Do(req)
	}
	resp, err := get0(cfg.RootCAs)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the system roots trusted the test CA")
	}

	env[EnvCAFile] = caFile
	if cfg, err = ConfigFromEnv(get); err != nil || cfg.RootCAs == nil {
		t.Fatalf("with the variable: %v", err)
	}
	resp, err = get0(cfg.RootCAs)
	if err != nil {
		t.Fatalf("with the CA file: %v", err)
	}
	_ = resp.Body.Close()

	empty := filepath.Join(dir, "empty.pem")
	if err := os.WriteFile(empty, []byte("no certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]string{"relative": "ca-bundle.pem", "missing": filepath.Join(dir, "none.pem"), "no PEM": empty} {
		env[EnvCAFile] = v
		if _, err := ConfigFromEnv(get); err == nil || !IsConfigError(err) || !strings.Contains(err.Error(), EnvCAFile) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
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

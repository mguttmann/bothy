// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// TestGetWebhookKeys_HTTP (M2-26): anonymous 200 with the next, active and retiring webhook_out
// keys as whpk_, not_after only on the retiring key, the Cache-Control and CORS answer of getJwks.
func TestGetWebhookKeys_HTTP(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	owner := e.person("owner", true, e.clk.Now())
	if _, err := e.keys.Rotate(ctx, owner, instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeScheduled, Reason: "Test"}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(keyCfg.JWKSMaxAge)
	if _, err := e.keys.RunRotation(ctx); err != nil {
		t.Fatal(err)
	}
	owner = e.person("owner2", true, e.clk.Now())
	if _, err := e.keys.Rotate(ctx, owner, instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeScheduled, Reason: "Test"}); err != nil {
		t.Fatal(err)
	}
	resp := e.h.Do(e.h.NewRequest(http.MethodGet, "/api/v1/instance/webhook-keys", "", "Origin", "https://app.example.com"))
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/json" ||
		resp.Header.Get("Cache-Control") != "public, max-age=1200" || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("GET: %d %v %s", resp.StatusCode, resp.Header, resp.Body)
	}
	var body struct {
		Keys []struct {
			Kid       string  `json:"kid"`
			PublicKey string  `json:"public_key"`
			State     string  `json:"state"`
			NotAfter  *string `json:"not_after"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatal(err)
	}
	var states []string
	for _, k := range body.Keys {
		states = append(states, k.State)
		der, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(k.PublicKey, "whpk_"))
		if err != nil || !strings.HasPrefix(k.PublicKey, "whpk_") {
			t.Fatalf("key %+v", k)
		}
		if pk, err := x509.ParsePKIXPublicKey(der); err != nil || len(pk.(ed25519.PublicKey)) != ed25519.PublicKeySize {
			t.Fatalf("key %+v: %v", k, err)
		}
		if (k.NotAfter != nil) != (k.State == "retiring") {
			t.Fatalf("not_after of %s: %v", k.State, k.NotAfter)
		}
	}
	if strings.Join(states, ",") != "active,next,retiring" {
		t.Fatalf("states %v", states)
	}
}

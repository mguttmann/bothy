// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// secretsOf returns the strings that must never reach a record or an error text: the device
// tokens and their five-character prefixes, every line of the PEM key, the client e-mail, the
// project ID, the IDs of n and the canary of the Fake's error.message.
func secretsOf(t *testing.T, w *world, n app.Notification) []string {
	t.Helper()
	out := []string{w.sam.Token, w.sam.Token[:5], w.robin.Token, w.robin.Token[:5], testEmail, projectID, canary,
		n.ID.String(), n.InstanceID.String(), n.ThreadID.String(), n.MessageID.String()}
	der, err := x509.MarshalPKCS8PrivateKey(w.key)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), "\n") {
		if len(l) > 8 && !strings.HasPrefix(l, "-----") {
			out = append(out, l)
		}
	}
	return out
}

// tokensOf returns the bearer tokens the Fake saw; with the assertions, they must never leak.
func tokensOf(w *world) []string {
	var out []string
	for _, r := range w.fake.Requests() {
		if r.Bearer != "" && !slices.Contains(out, r.Bearer) {
			out = append(out, r.Bearer)
		}
	}
	return out
}

// checkClean fails when a record or an error text contains a secret.
func checkClean(t *testing.T, w *world, n app.Notification, errs []string) {
	t.Helper()
	bad := append(secretsOf(t, w, n), tokensOf(w)...)
	for _, r := range w.log.records() {
		text := r.Message
		for k, v := range r.Attrs {
			text += " " + k + "=" + v
		}
		for _, s := range bad {
			if strings.Contains(text, s) {
				t.Fatalf("record %q contains a secret", text)
			}
		}
		if r.Level >= 8 {
			keys := make([]string, 0, len(r.Attrs))
			for k := range r.Attrs {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, []string{"bothy.push.provider", "error.type"}) {
				t.Fatalf("Error record attributes %v", keys)
			}
		}
	}
	for _, e := range errs {
		for _, s := range bad {
			if strings.Contains(e, s) {
				t.Fatalf("error %q contains a secret", e)
			}
		}
	}
}

func TestNoSecretsInLogsOrErrors(t *testing.T) {
	run := func(t *testing.T, w *world, n app.Notification) []string {
		t.Helper()
		_, err := w.a.Send(context.Background(), w.sam, n)
		if err != nil {
			return []string{err.Error()}
		}
		return nil
	}
	for _, c := range answerCases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			n := w.hint()
			w.fake.Script(w.sam.Token, c.script...)
			checkClean(t, w, n, run(t, w, n))
		})
	}
	t.Run("pause, probe, renewal and rotation", func(t *testing.T) {
		w := newWorld(t)
		n := w.hint()
		var errs []string
		w.fake.Script(w.sam.Token, fcmtest.Response{Status: 403, RPCStatus: "PERMISSION_DENIED"})
		errs = append(errs, run(t, w, n)...)
		errs = append(errs, run(t, w, n)...)
		w.clk.Advance(pauseFor)
		errs = append(errs, run(t, w, n)...)
		w.fake.RevokeTokens()
		errs = append(errs, run(t, w, n)...)
		next := newRSAKey(t)
		w.writeAccount(next)
		w.fake.SetPublicKey(&next.PublicKey)
		w.clk.Advance(lifetime)
		errs = append(errs, run(t, w, n)...)
		w.fake.ScriptToken(fcmtest.TokenResponse{Status: 400, Error: "invalid_grant"})
		w.clk.Advance(lifetime)
		errs = append(errs, run(t, w, n)...)
		checkClean(t, w, n, errs)
	})
	t.Run("closed Fake", func(t *testing.T) {
		w := newWorld(t)
		n := w.hint()
		w.send(w.sam, n)
		w.fake.Close()
		errs := run(t, w, n)
		if len(errs) != 1 {
			t.Fatal("no error against a closed Fake")
		}
		checkClean(t, w, n, errs)
	})
}

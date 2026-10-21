// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// secretsOf returns the strings that must never appear in a log record or an error text.
func secretsOf(t *testing.T, w *world, reg app.Registration, n app.Notification) []string {
	t.Helper()
	out := []string{reg.Token[:5], reg.AppID, n.ID.String(), w.instance.String()}
	if !n.ThreadID.IsZero() {
		out = append(out, n.ThreadID.String())
	}
	if !n.MessageID.IsZero() {
		out = append(out, n.MessageID.String())
	}
	for _, r := range w.prod.Requests() {
		if r.JWT != "" {
			out = append(out, r.JWT, strings.Split(r.JWT, ".")[2])
		}
	}
	for _, line := range strings.Split(string(pemPKCS8(t, w.key)), "\n") {
		if len(line) > 8 && !strings.HasPrefix(line, "-----") {
			out = append(out, line)
		}
	}
	return out
}

// leaks fails when text contains any of secrets.
func leaks(t *testing.T, what, text string, secrets []string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(text, s) {
			t.Errorf("%s leaks %q: %s", what, s, text)
		}
	}
}

// checkRecords checks every record of w.
func checkRecords(t *testing.T, w *world, secrets []string) {
	t.Helper()
	for _, r := range w.log.records() {
		text := r.Message
		for k, v := range r.Attrs {
			text += " " + k + "=" + v
		}
		leaks(t, "record", text, secrets)
		if r.Level == slog.LevelError {
			if keys := slices.Sorted(maps.Keys(r.Attrs)); !slices.Equal(keys, []string{"bothy.push.provider", "error.type"}) {
				t.Errorf("Error record %q carries %v", r.Message, keys)
			}
		}
	}
}

func TestNoSecretsInLogsOrErrors(t *testing.T) {
	t.Parallel()
	for _, row := range answerRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			n := w.hint()
			w.prod.Script(w.phone.Token, row.resp)
			if _, err := w.a.Send(t.Context(), w.phone, n); err != nil {
				leaks(t, "error", err.Error(), secretsOf(t, w, w.phone, n))
			}
			checkRecords(t, w, secretsOf(t, w, w.phone, n))
		})
	}
	t.Run("pause, probe, rotation and closed Fake", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		n := w.hint()
		w.prod.Script(w.phone.Token, apnstest.Response{Status: 403, Reason: "InvalidProviderToken"})
		w.send(w.phone, n)
		w.send(w.phone, n)
		w.clk.Advance(interval)
		w.send(w.phone, n)
		rotated := newKey(t)
		writeKey(t, w.keyFile, rotated)
		w.prod.SetPublicKey(&rotated.PublicKey)
		w.clk.Advance(interval)
		w.send(w.phone, n)
		w.resolver.setFail(errFailingResolver)
		w.clk.Advance(interval)
		w.send(w.phone, n)
		w.resolver.setFail(nil)
		w.prod.Close()
		w.clk.Advance(interval)
		_, err := w.a.Send(t.Context(), w.phone, n)
		if err == nil {
			t.Fatal("no error against a closed Fake")
		}
		s := secretsOf(t, w, w.phone, n)
		leaks(t, "error", err.Error(), s)
		checkRecords(t, w, s)
		if len(w.log.records()) == 0 {
			t.Fatal("no record captured")
		}
	})
}

// errFailingResolver stands for a failing secret provider; its text names nothing secret.
var errFailingResolver = failingError{}

type failingError struct{}

func (failingError) Error() string { return "secret provider unavailable" }

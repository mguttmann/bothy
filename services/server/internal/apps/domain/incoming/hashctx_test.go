// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestHashContext(t *testing.T) {
	webhook := ids.MustParse("017f22e2-79b0-7cc3-98c4-dc0c0c07398f")
	thread := ids.MustParse("017f22e2-79b0-7cc3-98c4-dc0c0c073990")
	title := "Nightly"

	if got, want := incoming.HashContext(webhook, &thread, &title),
		"webhook:017f22e2-79b0-7cc3-98c4-dc0c0c07398f|thread:017f22e2-79b0-7cc3-98c4-dc0c0c073990|title:Nightly"; got != want {
		t.Fatalf("HashContext = %q, want %q", got, want)
	}
	if got, want := incoming.HashContext(webhook, nil, nil), "webhook:017f22e2-79b0-7cc3-98c4-dc0c0c07398f|thread:|title:"; got != want {
		t.Fatalf("HashContext without thread and title = %q, want %q", got, want)
	}

	base := incoming.HashContext(webhook, &thread, &title)
	if incoming.HashContext(webhook, &thread, &title) != base {
		t.Fatal("not stable")
	}
	otherTitle := "Weekly"
	otherThread := ids.New()
	for name, got := range map[string]string{
		"webhook":   incoming.HashContext(ids.New(), &thread, &title),
		"thread":    incoming.HashContext(webhook, &otherThread, &title),
		"no thread": incoming.HashContext(webhook, nil, &title),
		"title":     incoming.HashContext(webhook, &thread, &otherTitle),
		"no title":  incoming.HashContext(webhook, &thread, nil),
	} {
		if got == base {
			t.Errorf("a different %s gives the same context", name)
		}
	}
}

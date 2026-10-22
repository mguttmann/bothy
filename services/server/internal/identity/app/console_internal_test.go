// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/consolesession"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// TestConsoleCookieNameMatches: the domain writes the cookie under the name the HTTP chain and
// the gateway read (M6-01).
func TestConsoleCookieNameMatches(t *testing.T) {
	t.Parallel()
	if consolesession.CookieName != actor.ConsoleCookieName {
		t.Fatalf("%q != %q", consolesession.CookieName, actor.ConsoleCookieName)
	}
}

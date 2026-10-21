// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestNoLinks (M2-43): the binding without control connections has no connected host, refuses
// to open a session with ErrHostNotConnected and closes nothing.
func TestNoLinks(t *testing.T) {
	t.Parallel()
	var l app.ControlLinks = app.NoLinks{}
	host := ids.New()
	if l.Connected(host) {
		t.Error("NoLinks reports a connected host")
	}
	if err := l.OpenSession(context.Background(), host, app.SessionOpen{}); !errors.Is(err, app.ErrHostNotConnected) {
		t.Errorf("OpenSession: %v", err)
	}
	if err := l.CloseSession(context.Background(), host, ids.New(), domain.CloseReason("access_revoked")); err != nil {
		t.Errorf("CloseSession: %v", err)
	}
	l.CloseHost(host, domain.CloseCode(4111))
	l.SendReady(host)
}

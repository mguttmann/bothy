// SPDX-License-Identifier: Apache-2.0

// Package importtool holds the real-server tests of the import tool (component testsupport): the
// tool logs in against the full server of the harness (M1-71) through device pairing.
package importtool

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	importapi "github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// syncBuffer is a bytes.Buffer safe for the writer of Login and the reader in the sleeper.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestLoginAgainstRealServer (M8-10, 08 5.3 and 6.4, ID-T29): Robin, an owner on an active
// device, approves the pairing code of bothy import login with step-up; the tool reaches getMe as
// a cli device; createStepUpChallenge from the tool's session answers 403 and approvePairing (an
// action with step-up) answers 401 step_up_required.
func TestLoginAgainstRealServer(t *testing.T) {
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	robin := s.NewUser(t, harness.UserSpec{Handle: "robin", DisplayName: "Robin"})
	harness.MustDo(t, s.Owner, harness.Request{Method: http.MethodPost, Path: "/owners",
		Body: map[string]any{"user_id": robin.PrincipalID, "reason": "import owner"}}, http.StatusCreated)

	path := filepath.Join(t.TempDir(), "bothy", "import-session.json")
	var stdout, stderr syncBuffer
	var approveErr error
	approved := false
	limits := importapi.DefaultLimits()
	limits.LoginPollInterval = 50 * time.Millisecond
	sleep := func(ctx context.Context, d time.Duration) error {
		if !approved {
			approved = true
			code := strings.TrimSpace(stdout.String())
			resp, err := harness.DoWithStepUp(ctx, robin, harness.Request{Method: http.MethodPost, Path: "/me/devices/pairings",
				Body: map[string]any{"pairing_code": code}})
			if err != nil || resp.Status/100 != 2 {
				approveErr = err
				if approveErr == nil {
					approveErr = &importapi.StatusError{Op: "approvePairing", Status: resp.Status}
				}
			}
		}
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	err := importapi.Login(t.Context(), importapi.LoginOptions{ServerURL: s.PublicURL, SessionFile: path, AppVersion: "1.0.0",
		Hostname: "workstation", Limits: limits, Deps: importapi.Deps{Sleep: sleep}, Stdout: &stdout, Stderr: &stderr})
	if approveErr != nil {
		t.Fatalf("approvePairing: %v", approveErr)
	}
	if err != nil {
		t.Fatalf("Login: %v\n%s", err, stderr.String())
	}
	c, err := importapi.New(t.Context(), importapi.Options{SessionFile: path, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	me, err := c.Me(t.Context())
	if err != nil {
		t.Fatalf("getMe: %v", err)
	}
	if me.Principal.Id.String() != robin.PrincipalID || !me.IsOwner {
		t.Errorf("me = %+v, want Robin as owner", me.Principal)
	}
	var platform string
	if err := s.AdminDB().QueryRow(t.Context(), "SELECT platform FROM devices WHERE id = $1", *me.DeviceId).Scan(&platform); err != nil {
		t.Fatal(err)
	}
	if platform != "cli" {
		t.Errorf("platform = %q, want cli", platform)
	}

	ch, err := c.CreateStepUpChallengeWithResponse(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if ch.StatusCode() != http.StatusForbidden {
		t.Errorf("createStepUpChallenge = %d, want 403", ch.StatusCode())
	}
	ap, err := c.ApprovePairingWithResponse(t.Context(), &restclient.ApprovePairingParams{}, restclient.ApprovePairingJSONRequestBody{PairingCode: "7KQ2-M9XD"})
	if err != nil {
		t.Fatal(err)
	}
	if ap.StatusCode() != http.StatusUnauthorized || ap.ApplicationproblemJSON401 == nil || ap.ApplicationproblemJSON401.Code != "step_up_required" {
		t.Errorf("approvePairing = %d %s, want 401 step_up_required", ap.StatusCode(), ap.Body)
	}
	if err := importapi.Logout(t.Context(), importapi.Options{SessionFile: path, Limits: limits}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

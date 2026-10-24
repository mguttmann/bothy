// SPDX-License-Identifier: Apache-2.0

package harness_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestExternal proves harness.External (M2-48): with an enrollment code of the owner of a server
// the harness did not start, it enrols the owner with fresh device keys, and NewUser creates and
// signs in a person through the public API of that server.
func TestExternal(t *testing.T) {
	t.Parallel()
	s := harness.Start(t, harness.Options{Config: harness.RaisedRateLimits()})
	resp, err := harness.DoWithStepUp(t.Context(), s.Owner, harness.Request{Method: http.MethodPost, Path: "/users/" + s.Owner.PrincipalID + "/enrollment-codes"})
	if err != nil || resp.Status != http.StatusCreated {
		t.Fatalf("createEnrollmentCode: %d %v", resp.Status, err)
	}
	var code struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(resp.Body, &code); err != nil {
		t.Fatal(err)
	}
	ext := harness.External(t, harness.ExternalOptions{PublicURL: s.PublicURL, HTTPClient: &http.Client{}, OwnerEnrollmentCode: code.Code})
	if ext.Owner.PrincipalID != s.Owner.PrincipalID || ext.Owner.DeviceID == s.Owner.DeviceID {
		t.Fatalf("the external owner is %s on device %s", ext.Owner.PrincipalID, ext.Owner.DeviceID)
	}
	alex := ext.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	me := alex.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/me"})
	if me.Status != http.StatusOK {
		t.Fatalf("getMe of the new person: %d", me.Status)
	}
	if ext.SecretsDir != "" || ext.SigningKeyDir != "" {
		t.Error("an external server names a secrets directory")
	}
	if _, err := ext.OwnerSession(t.Context()); err == nil {
		t.Error("OwnerSession works without the setup service")
	}
}

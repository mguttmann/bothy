// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"testing"

	audithttp "github.com/OWNER/bothy/services/server/internal/audit/adapters/httpapi"
	identityhttp "github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	instancehttp "github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
)

// TestHandlers_FillsBuiltAndKeepsInjected holds handlers() (INT-W12/3): every nil module
// handler becomes the one the start built, an injected one stays, and nothing is filled before
// its module started.
func TestHandlers_FillsBuiltAndKeepsInjected(t *testing.T) {
	id := &Identity{
		Handlers:           &identityhttp.Devices{},
		PeopleHandlers:     &identityhttp.People{},
		SessionHandlers:    &identityhttp.Sessions{},
		StepUpHandlers:     &identityhttp.StepUps{},
		ProviderHandlers:   &identityhttp.Providers{},
		CredentialHandlers: &identityhttp.Credentials{},
		TokenHandlers:      &identityhttp.Tokens{},
		EnrollmentHandlers: &identityhttp.Enrollment{},
		ApprovalHandlers:   &identityhttp.Approval{},
	}
	keys := &SigningKeys{Handlers: &instancehttp.Keys{}}
	log := &audithttp.Log{}
	injected := &identityhttp.Tokens{}

	s := &server{identity: id, signingKeys: keys, audit: &Audit{handlers: log}}
	s.deps.Handlers.Tokens = injected
	h := s.handlers()

	checks := []struct {
		name      string
		got, want any
	}{
		{"Devices", h.Devices, id.Handlers},
		{"People", h.People, id.PeopleHandlers},
		{"Sessions", h.Sessions, id.SessionHandlers},
		{"StepUps", h.StepUps, id.StepUpHandlers},
		{"Providers", h.Providers, id.ProviderHandlers},
		{"Credentials", h.Credentials, id.CredentialHandlers},
		{"Tokens", h.Tokens, injected},
		{"Enrollment", h.Enrollment, id.EnrollmentHandlers},
		{"Approval", h.Approval, id.ApprovalHandlers},
		{"Keys", h.Keys, keys.Handlers},
		{"Log", h.Log, log},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %p, want %p", c.name, c.got, c.want)
		}
	}

	empty := (&server{}).handlers()
	if empty.Devices != nil || empty.Tokens != nil || empty.Providers != nil || empty.Keys != nil || empty.Log != nil {
		t.Errorf("handlers() before the start = %+v, want only nil handlers", empty)
	}
}

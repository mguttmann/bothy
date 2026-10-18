// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// UserSpec describes a person NewUser creates.
type UserSpec struct {
	Handle, DisplayName, Email string
}

// AgentSpec describes an agent NewAgent creates. BillingMode is "organization" (default) or
// "personal"; CredentialHolder is the principal ID of the holder of a personal agent.
type AgentSpec struct {
	Handle, DisplayName, BillingMode, CredentialHolder string
}

// apiTokenTTL is the validity of the agent tokens of NewAgent.
const apiTokenTTL = 24 * time.Hour

// MustDo sends r as c and fails tb unless the status is want. A 401 step_up_required makes c
// perform a step-up and repeat r with the same Idempotency-Key (06 "Step-up").
func MustDo(tb testing.TB, c *Client, r Request, want int) Response {
	tb.Helper()
	resp, err := DoWithStepUp(tb.Context(), c, r)
	if err != nil {
		tb.Fatalf("harness: %v", err)
	}
	if resp.Status != want {
		tb.Fatalf("harness: %s %s = %d %s, want %d", r.Method, r.Path, resp.Status, resp.Body, want)
	}
	return resp
}

// DoWithStepUp sends r as c; a 401 step_up_required makes c perform a step-up and repeat r with
// the same Idempotency-Key (06 "Step-up"). The error is a failed step-up.
func DoWithStepUp(ctx context.Context, c *Client, r Request) (Response, error) {
	if (r.Method == http.MethodPost || r.Method == http.MethodPatch) && r.Header.Get("Idempotency-Key") == "" && !exempt(r.Method, r.Path) {
		r.Header = r.Header.Clone()
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set("Idempotency-Key", ids.New().String())
	}
	resp := c.Do(ctx, r)
	if resp.Status == http.StatusUnauthorized && problemCode(resp.Body) == "step_up_required" {
		if err := c.StepUp(ctx); err != nil {
			return resp, err
		}
		resp = c.Do(ctx, r)
	}
	return resp, nil
}

// NewUser creates a person with a local account through the public API as the owner (createUser
// with local_account), redeems the enrollment code with fresh device keys (redeemEnrollmentCode)
// and returns the signed-in client.
func (s *Server) NewUser(tb testing.TB, spec UserSpec) *Client {
	tb.Helper()
	body := map[string]any{"handle": spec.Handle, "display_name": spec.DisplayName, "local_account": true}
	if spec.Email != "" {
		body["email"] = spec.Email
	}
	resp := MustDo(tb, s.Owner, Request{Method: http.MethodPost, Path: "/users", Body: body}, http.StatusCreated)
	var created struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Enrollment struct {
			Code string `json:"code"`
		} `json:"enrollment"`
	}
	resp.JSON(tb, &created)
	c, err := s.redeem(tb.Context(), created.Enrollment.Code, spec.DisplayName+" device")
	if err != nil {
		tb.Fatalf("harness: %v", err)
	}
	c.PrincipalID = created.User.ID
	s.users.Store(spec.Handle, c)
	return c
}

// NewAgent creates an agent through the public API as the owner (createAgent, then
// setAgentBillingMode for a personal agent with its credential holder) and an API token bound to
// a fresh DPoP key (createToken with dpop_jkt), and returns the client of the agent.
func (s *Server) NewAgent(tb testing.TB, spec AgentSpec) *Client {
	tb.Helper()
	resp := MustDo(tb, s.Owner, Request{Method: http.MethodPost, Path: "/agents", Body: map[string]any{
		"handle": spec.Handle, "display_name": spec.DisplayName, "adapter_kind": "hermes",
	}}, http.StatusCreated)
	var agent struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	resp.JSON(tb, &agent)
	if spec.BillingMode == "personal" {
		body := map[string]any{"billing_mode": "personal", "reason": "harness"}
		if spec.CredentialHolder != "" {
			body["credential_holder_id"] = spec.CredentialHolder
		}
		MustDo(tb, s.Owner, Request{Method: http.MethodPut, Path: "/agents/" + agent.ID + "/billing-mode", Body: body,
			Header: ifMatch(agent.Version)}, http.StatusOK)
	}
	key := newDeviceKey()
	jwk, err := jose.ParseJWK(key.jwk)
	if err != nil {
		tb.Fatal(err)
	}
	tok := MustDo(tb, s.Owner, Request{Method: http.MethodPost, Path: "/principals/" + agent.ID + "/tokens", Body: map[string]any{
		"name": "harness", "dpop_jkt": jwk.Thumbprint(), "expires_at": s.clk.Now().Add(apiTokenTTL).UTC().Format("2006-01-02T15:04:05.000Z"),
	}}, http.StatusCreated)
	var token struct {
		Secret string `json:"secret"`
	}
	tok.JSON(tb, &token)
	return &Client{PrincipalID: agent.ID, publicURL: s.PublicURL, http: s.http, key: key, access: token.Secret}
}

// ifMatch returns an If-Match header for version.
func ifMatch(version int64) http.Header {
	h := http.Header{}
	h.Set("If-Match", `"`+jsonInt(version)+`"`)
	return h
}

// jsonInt formats v as decimal text.
func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// OwnerSession signs the first owner in on a new device with a recovery code of the setup
// service (bothy setup issue-enrollment-code), independent of every existing session, and returns
// its client. The credential broker of G7 uses it for fresh owner sessions.
func (s *Server) OwnerSession(ctx context.Context) (*Client, error) {
	setup, err := wire.NewSetup(ctx, s.values, s.env, slog.New(slog.DiscardHandler))
	if err != nil {
		return nil, err
	}
	defer setup.Close(context.WithoutCancel(ctx))
	code, err := setup.IssueRecoveryCode(ctx, ownerHandle)
	if err != nil {
		return nil, fmt.Errorf("issue-enrollment-code: %w", err)
	}
	return s.redeem(ctx, code.Code, "Owner device")
}

// UserSession signs the person userID in on a new device: a fresh owner session creates an
// enrollment code (createEnrollmentCode with step-up) and the device redeems it. Errors: the
// person is not active, or a step fails.
func (s *Server) UserSession(ctx context.Context, userID string) (*Client, error) {
	o, err := s.OwnerSession(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := DoWithStepUp(ctx, o, Request{Method: http.MethodPost, Path: "/users/" + userID + "/enrollment-codes"})
	if err != nil {
		return nil, err
	}
	if resp.Status != http.StatusCreated {
		return nil, fmt.Errorf("createEnrollmentCode: %d %s", resp.Status, resp.Body)
	}
	var code struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(resp.Body, &code); err != nil {
		return nil, err
	}
	c, err := s.redeem(ctx, code.Code, "Device")
	if err != nil {
		return nil, err
	}
	c.PrincipalID = userID
	return c, nil
}

// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// approvalPort answers every call with err, or with a waiting pairing.
type approvalPort struct{ err error }

func (p approvalPort) ApproveDevice(context.Context, ids.ID, string) (identity.Device, error) {
	return identity.Device{}, p.err
}

func (p approvalPort) BeginPairing(context.Context, identity.RegisterDevice, string, netip.Addr) (identity.PairingStarted, error) {
	return identity.PairingStarted{ID: ids.New(), Code: identity.NewToken("7KQ2-M9XD"), ExpiresAt: time.Unix(1, 0)}, p.err
}

func (p approvalPort) ApprovePairing(context.Context, string) (identity.Device, error) {
	return identity.Device{}, p.err
}

func (p approvalPort) CompletePairing(context.Context, ids.ID, string, netip.Addr) (identity.PairingOutcome, error) {
	return identity.PairingOutcome{PairingID: ids.New(), ExpiresAt: time.Unix(1, 0)}, p.err
}

// proofPort verifies every proof as stale, or fails with err.
type proofPort struct{ err error }

func (p proofPort) CurrentNonce() string { return "n2" }

func (p proofPort) Verify(context.Context, identity.ProofRequest) (identity.VerifiedProof, error) {
	return identity.VerifiedProof{JKT: "jkt", NonceStale: true}, p.err
}

// TestApprovalHandlers: a missing body is validation_failed, errors of the proof and of the port
// come back unchanged, a stale proof gets the current nonce, and a waiting pairing answers 202.
func TestApprovalHandlers(t *testing.T) {
	ctx := context.Background()
	failed := errs.New(errs.CodeConflict, "")
	ok := NewApproval(approvalPort{}, proofPort{})
	bad := NewApproval(approvalPort{err: failed}, proofPort{})
	badProof := NewApproval(approvalPort{}, proofPort{err: failed})
	dev := rest.DeviceRegistration{PublicJwk: rest.JsonObject{"kty": "EC"}}
	calls := map[string]func(*Approval, bool) error{
		"approve": func(h *Approval, body bool) error {
			r := rest.ApproveDeviceRequestObject{}
			if body {
				r.Body = &rest.ApproveDeviceJSONRequestBody{}
			}
			_, err := h.ApproveDevice(ctx, r)
			return err
		},
		"begin": func(h *Approval, body bool) error {
			r := rest.BeginPairingRequestObject{}
			if body {
				r.Body = &rest.BeginPairingJSONRequestBody{Device: dev}
			}
			_, err := h.BeginPairing(ctx, r)
			return err
		},
		"approve pairing": func(h *Approval, body bool) error {
			r := rest.ApprovePairingRequestObject{}
			if body {
				r.Body = &rest.ApprovePairingJSONRequestBody{}
			}
			_, err := h.ApprovePairing(ctx, r)
			return err
		},
		"complete": func(h *Approval, body bool) error {
			r := rest.CompletePairingRequestObject{}
			if body {
				r.Body = &rest.CompletePairingJSONRequestBody{}
			}
			_, err := h.CompletePairing(ctx, r)
			return err
		},
	}
	for name, call := range calls {
		if err := call(ok, false); errs.KindOf(err) != errs.KindBadRequest {
			t.Errorf("%s without body = %v", name, err)
		}
		if err := call(bad, true); !errors.Is(err, failed) {
			t.Errorf("%s port error = %v", name, err)
		}
	}
	for _, name := range []string{"begin", "complete"} {
		if err := calls[name](badProof, true); !errors.Is(err, failed) {
			t.Errorf("%s proof error = %v", name, err)
		}
	}
	headers := map[string]string{}
	hctx := reqmeta.WithResponseHeaders(ctx, func(k, v string) { headers[k] = v })
	resp, err := ok.CompletePairing(hctx, rest.CompletePairingRequestObject{Body: &rest.CompletePairingJSONRequestBody{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, waiting := resp.(rest.CompletePairing202JSONResponse); !waiting || headers[headerDPoPNonce] != "n2" {
		t.Errorf("waiting pairing = %T, headers %v", resp, headers)
	}
	if _, err := ok.BeginPairing(ctx, rest.BeginPairingRequestObject{Body: &rest.BeginPairingJSONRequestBody{Device: rest.DeviceRegistration{PublicJwk: rest.JsonObject{"x": func() {}}}}}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("begin with a bad registration = %v", err)
	}
}

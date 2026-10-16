// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// enrollmentPort records the redemption it got and answers with res and err.
type enrollmentPort struct {
	got identity.RedeemEnrollment
	res identity.RedeemResult
	err error
}

func (p *enrollmentPort) RedeemEnrollmentCode(_ context.Context, in identity.RedeemEnrollment) (identity.RedeemResult, error) {
	p.got = in
	return p.res, p.err
}

func (*enrollmentPort) CreateEnrollmentCode(context.Context, actor.Actor, ids.ID) (identity.EnrollmentCode, error) {
	return identity.EnrollmentCode{}, errors.New("not used")
}

// TestRegistration converts every optional part of a device registration and refuses a JWK that
// does not encode as JSON.
func TestRegistration(t *testing.T) {
	version := "1.4.0"
	att := rest.JsonObject{"fmt": "example"}
	d := rest.DeviceRegistration{
		DeviceId: rest.Uuid(ids.New()), Name: "Alex phone", Platform: "ios", Alg: "ES256",
		PublicJwk: rest.JsonObject{"kty": "EC"}, AppVersion: &version, Attestation: &att,
		StepUpKey: &rest.DeviceKeyRegistration{Alg: "ES256", PublicJwk: rest.JsonObject{"kty": "EC"}, Attestation: &att},
	}
	out, err := registration(d)
	if err != nil {
		t.Fatal(err)
	}
	if out.AppVersion != version || out.StepUpKey == nil || string(out.StepUpKey.Attestation) != `{"fmt":"example"}` ||
		string(out.DPoPKey.Attestation) != `{"fmt":"example"}` || out.StepUpKey.Alg != "ES256" {
		t.Fatalf("registration = %+v", out)
	}
	bad := rest.JsonObject{"x": func() {}}
	for name, mutate := range map[string]func(*rest.DeviceRegistration){
		"jwk":         func(r *rest.DeviceRegistration) { r.PublicJwk = bad },
		"attestation": func(r *rest.DeviceRegistration) { r.Attestation = &bad },
		"step-up jwk": func(r *rest.DeviceRegistration) { r.StepUpKey = &rest.DeviceKeyRegistration{PublicJwk: bad} },
		"step-up attestation": func(r *rest.DeviceRegistration) {
			r.StepUpKey = &rest.DeviceKeyRegistration{PublicJwk: rest.JsonObject{}, Attestation: &bad}
		},
	} {
		r := d
		mutate(&r)
		if _, err := registration(r); errs.KindOf(err) != errs.KindBadRequest {
			t.Errorf("%s: registration = %v, want validation_failed", name, err)
		}
	}
}

// TestLoginProof takes the proof facts of the chain, and falls back to the header and POST.
func TestLoginProof(t *testing.T) {
	ctx := reqmeta.WithProof(context.Background(), reqmeta.Proof{Method: http.MethodPut, HTU: "https://chat.example.com/x", Values: []string{"a", "b"}})
	if p := loginProof(ctx, "h"); p.Method != http.MethodPut || len(p.Proofs) != 2 || p.URL != "https://chat.example.com/x" {
		t.Errorf("chain proof = %+v", p)
	}
	if p := loginProof(context.Background(), "h"); p.Method != http.MethodPost || len(p.Proofs) != 1 || p.Proofs[0] != "h" {
		t.Errorf("header proof = %+v", p)
	}
}

// TestRedeemEnrollmentCodeHandler: no body and a refused registration are validation_failed, the
// client IP and the nonce pass through, and a port error comes back unchanged.
func TestRedeemEnrollmentCodeHandler(t *testing.T) {
	p := &enrollmentPort{res: identity.RedeemResult{Nonce: "n2"}}
	h := NewEnrollment(p)
	if _, err := h.RedeemEnrollmentCode(context.Background(), rest.RedeemEnrollmentCodeRequestObject{}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("no body = %v", err)
	}
	body := &rest.EnrollRedeemRequest{EnrollmentCode: "EXAMPLE", Device: rest.DeviceRegistration{PublicJwk: rest.JsonObject{"x": func() {}}}}
	if _, err := h.RedeemEnrollmentCode(context.Background(), rest.RedeemEnrollmentCodeRequestObject{Body: body}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("bad registration = %v", err)
	}
	headers := map[string]string{}
	ctx := reqmeta.WithClientIP(context.Background(), netip.MustParseAddr("192.0.2.1"))
	ctx = reqmeta.WithResponseHeaders(ctx, func(k, v string) { headers[k] = v })
	body.Device.PublicJwk = rest.JsonObject{"kty": "EC"}
	if _, err := h.RedeemEnrollmentCode(ctx, rest.RedeemEnrollmentCodeRequestObject{Body: body}); err != nil {
		t.Fatal(err)
	}
	if p.got.ClientIP.String() != "192.0.2.1" || headers[headerDPoPNonce] != "n2" {
		t.Errorf("client IP %v, headers %v", p.got.ClientIP, headers)
	}
	p.err = errs.New(errs.CodeUnauthenticated, "")
	if _, err := h.RedeemEnrollmentCode(ctx, rest.RedeemEnrollmentCodeRequestObject{Body: body}); !errors.Is(err, p.err) {
		t.Errorf("port error = %v", err)
	}
}

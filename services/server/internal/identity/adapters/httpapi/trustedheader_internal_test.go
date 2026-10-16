// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// headerLogins records the login it got and answers with res and err.
type headerLogins struct {
	got identity.TrustedHeaderLogin
	res identity.RedeemResult
	err error
}

func (l *headerLogins) LoginWithTrustedHeader(_ context.Context, in identity.TrustedHeaderLogin) (identity.RedeemResult, error) {
	l.got = in
	return l.res, l.err
}

// TestLoginWithTrustedHeaderHandler: no body and a refused registration are validation_failed;
// the enrollment code, the transport and the client IP pass through, the nonce comes back, and a
// port error is returned unchanged.
func TestLoginWithTrustedHeaderHandler(t *testing.T) {
	l := &headerLogins{res: identity.RedeemResult{Nonce: "n2"}}
	h := NewTrustedHeader(l)
	ctx := context.Background()
	if _, err := h.LoginWithTrustedHeader(ctx, rest.LoginWithTrustedHeaderRequestObject{}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("no body = %v", err)
	}
	body := &rest.LoginWithTrustedHeaderJSONRequestBody{Device: rest.DeviceRegistration{PublicJwk: rest.JsonObject{"x": func() {}}}}
	if _, err := h.LoginWithTrustedHeader(ctx, rest.LoginWithTrustedHeaderRequestObject{Body: body}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("bad registration = %v", err)
	}
	code := "EXAMPLE"
	body.Device.PublicJwk, body.EnrollmentCode = rest.JsonObject{"kty": "EC"}, &code
	headers := map[string]string{}
	rctx := reqmeta.WithTransport(ctx, reqmeta.Transport{Peer: netip.MustParseAddr("127.0.0.1"), Header: map[string][]string{"X": {"y"}}})
	rctx = reqmeta.WithClientIP(rctx, netip.MustParseAddr("192.0.2.1"))
	rctx = reqmeta.WithResponseHeaders(rctx, func(k, v string) { headers[k] = v })
	if _, err := h.LoginWithTrustedHeader(rctx, rest.LoginWithTrustedHeaderRequestObject{Body: body}); err != nil {
		t.Fatal(err)
	}
	if l.got.EnrollmentCode != code || l.got.Peer.String() != "127.0.0.1" || l.got.ClientIP.String() != "192.0.2.1" || headers[headerDPoPNonce] != "n2" {
		t.Errorf("login %+v, headers %v", l.got, headers)
	}
	l.err = errs.New(errs.CodeUnauthenticated, "")
	if _, err := h.LoginWithTrustedHeader(rctx, rest.LoginWithTrustedHeaderRequestObject{Body: body}); !errors.Is(err, l.err) {
		t.Errorf("port error = %v", err)
	}
}

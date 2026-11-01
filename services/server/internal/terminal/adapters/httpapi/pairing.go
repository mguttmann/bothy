// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// IssueHostPairingCode implements rest.StrictServerInterface (POST /hosts/{host_id}/pairing-code,
// M2-41): the new pairing code leaves the server only here, once; the Idempotency-Key stage
// stores no copy of it (MarkOneTimeSecret), and the chain answers with Cache-Control: no-store.
func (h *Hosts) IssueHostPairingCode(ctx context.Context, req rest.IssueHostPairingCodeRequestObject) (rest.IssueHostPairingCodeResponseObject, error) {
	idempotency.MarkOneTimeSecret(ctx)
	cmd := terminal.IssuePairingCode{HostID: ids.ID(req.HostId)}
	if b := req.Body; b != nil {
		cmd.Reason = b.Reason
	}
	issued, err := h.hosts.IssuePairingCode(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.IssueHostPairingCode201JSONResponse{Body: rest.HostPairingCode{
		HostId: rest.Uuid(issued.HostID), PairingCode: issued.Code.Reveal(), ExpiresAt: issued.ExpiresAt.UTC().Truncate(time.Millisecond),
	}}, nil
}

// PairHost implements rest.StrictServerInterface (POST /hosts/pair, M2-41): the public
// redemption of a pairing code by the host service. The answer holds only public data.
func (h *Hosts) PairHost(ctx context.Context, req rest.PairHostRequestObject) (rest.PairHostResponseObject, error) {
	var cmd terminal.PairHost
	if b := req.Body; b != nil {
		cmd = terminal.PairHost{
			Code: terminal.NewSecret(b.PairingCode), NoisePublicKey: b.NoisePublicKey, AuthPublicJWK: map[string]any(b.AuthPublicJwk),
			Platform: string(b.Platform), HostVersion: b.HostVersion,
		}
		if b.OsVersion != nil {
			cmd.OSVersion = *b.OsVersion
		}
		for _, p := range b.Profiles {
			cmd.Profiles = append(cmd.Profiles, terminal.Profile{Name: p.Name, DisplayName: p.DisplayName, Kind: string(p.Kind)})
		}
		for _, u := range b.SessionUsers {
			cmd.SessionUsers = append(cmd.SessionUsers, terminal.ReportedSessionUser{OSUser: u.OsUser, Workdirs: u.Workdirs, OwnerAccount: isTrue(u.OwnerAccount)})
		}
	}
	out, err := h.hosts.Pair(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.PairHost201JSONResponse{Body: rest.HostPairResult{
		HostId: rest.Uuid(out.HostID), KeyFingerprint: out.KeyFingerprint, InstanceId: rest.Uuid(out.InstanceID),
	}}, nil
}

// ConfirmHost implements rest.StrictServerInterface (POST /hosts/{host_id}/confirm, M2-41). A
// 409 fingerprint_mismatch keeps its effect under an Idempotency-Key (KeepEffectOnFinal), so
// every failed attempt counts and its replay returns the stored answer (TM-T19); the answer of a
// confirmation is the host with its ETag.
func (h *Hosts) ConfirmHost(ctx context.Context, req rest.ConfirmHostRequestObject) (rest.ConfirmHostResponseObject, error) {
	cmd := terminal.ConfirmHost{HostID: ids.ID(req.HostId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		cmd.FingerprintPrefix, cmd.LLMCredentials, cmd.Reason = b.FingerprintPrefix, string(b.LlmCredentials), b.Reason
		cmd.DeclaredServerHost = b.DeclaredServerHost
		if b.CredentialHolderId != nil {
			cmd.CredentialHolderID = ids.ID(*b.CredentialHolderId)
		}
		if b.SessionUsers != nil {
			for _, u := range *b.SessionUsers {
				cmd.SessionUsers = append(cmd.SessionUsers, terminal.SessionUserAssign{UserID: ids.ID(u.UserId), OSUser: u.OsUser, OwnerAccount: isTrue(u.OwnerAccount)})
			}
		}
	}
	v, err := h.hosts.Confirm(ctx, principal(ctx), cmd)
	if e, ok := errs.As(err); ok && e.Code == errs.CodeFingerprintMismatch {
		idempotency.KeepEffectOnFinal(ctx)
	}
	if err != nil {
		return nil, err
	}
	return rest.ConfirmHost200JSONResponse{Body: toREST(v), Headers: rest.ConfirmHost200ResponseHeaders{ETag: etag(v)}}, nil
}

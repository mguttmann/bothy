// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CeremonyUser is the person of a WebAuthn ceremony as the verifier sees it: the principal ID
// (whose 16 bytes are the user handle), the name and display name of the creation options and the
// active passkeys.
type CeremonyUser struct {
	ID                ids.ID
	Name, DisplayName string
	Credentials       []StoredPasskey
}

// NewPasskey is a verified creation response: the credential to store. It carries no secret.
type NewPasskey struct {
	CredentialID, PublicKeyCOSE []byte
	SignCount                   uint32
	// AAGUID is the authenticator model; zero for none.
	AAGUID                      ids.ID
	Transports                  []string
	BackupEligible, BackupState bool
}

// AssertionResult is a verified assertion: the person and the passkey the lookup returned, and the
// signature counter and backup state of the authenticator data.
type AssertionResult struct {
	User    CeremonyUser
	Passkey StoredPasskey
	// SignCount is the counter the authenticator reported; domain.SignCount decides what is kept.
	SignCount   uint32
	BackupState bool
}

// ErrPasskeyRejected is the error class of a response the verifier refused (wrong origin, RP ID,
// flags, signature, algorithm, challenge or format). The use cases answer it like a wrong
// confirmation; every other error of the verifier is a server failure.
var ErrPasskeyRejected = errors.New("identity: passkey response rejected")

// PasskeyVerifier is the outgoing port of the WebAuthn ceremonies (brief M1-40, names fixed;
// implemented by adapters/webauthn with go-webauthn). JSON goes in and out, so the application
// layer stays free of the library. The relying party (RP ID, origins), userVerification =
// required, residentKey = preferred, attestation = none and the algorithms ES256, EdDSA and RS256
// are fixed by the adapter's configuration. challenge is the raw challenge value; the use case
// has checked its HMAC against the stored row before it calls a Verify method.
type PasskeyVerifier interface {
	// CreationOptions returns PublicKeyCredentialCreationOptions in JSON form for user with
	// challenge; excludeCredentials are user.Credentials.
	CreationOptions(user CeremonyUser, challenge []byte) (optionsJSON []byte, err error)
	// VerifyCreation verifies responseJSON, a PublicKeyCredential with an attestation response,
	// against challenge and user. Errors: ErrPasskeyRejected for a refused response.
	VerifyCreation(user CeremonyUser, challenge []byte, responseJSON []byte) (NewPasskey, error)
	// RequestOptions returns PublicKeyCredentialRequestOptions in JSON form with challenge; allow
	// becomes allowCredentials (none for a discoverable login).
	RequestOptions(allow []StoredPasskey, challenge []byte) (optionsJSON []byte, err error)
	// VerifyAssertion resolves the credential through lookup (discoverable or allow-listed).
	// lookup gets the credential ID and the user handle of the response (nil without one) and
	// returns the person with the passkey among its Credentials; an error of lookup is returned
	// unchanged. Errors: ErrPasskeyRejected for a refused response.
	VerifyAssertion(challenge []byte, responseJSON []byte, lookup func(credentialID, userHandle []byte) (CeremonyUser, error)) (AssertionResult, error)
}

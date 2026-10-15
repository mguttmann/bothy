// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The field error codes of pairing (24 section 3.2, Design note 4 of M2-41).
const (
	// FieldInvalidKey marks a public key that is malformed or can never be used.
	FieldInvalidKey = "invalid_key"
	// FieldUnknownMember marks a JWK member other than kty, crv and x (the private d above all).
	FieldUnknownMember = "unknown_member"
	// FieldEmpty marks a list that must hold at least one entry.
	FieldEmpty = "empty"
	// FieldDuplicate marks a second entry with the same name.
	FieldDuplicate = "duplicate"
	// FieldLimitReached marks a list above terminal.report_max_entries.
	FieldLimitReached = "limit_reached"
	// FieldInvalidFormat marks a value outside its format (host_version, fingerprint_prefix).
	FieldInvalidFormat = "invalid_format"
)

// The JWK of the login key of a host (24 section 4.1): exactly these members.
const (
	jwkKty     = "kty"
	jwkCrv     = "crv"
	jwkX       = "x"
	jwkKtyOKP  = "OKP"
	jwkCrvEd   = "Ed25519"
	ptrNoise   = "/noise_public_key"
	ptrJWK     = "/auth_public_jwk/"
	ptrPrefix  = "/fingerprint_prefix"
	ptrUsers   = "/session_users"
	ptrProfile = "/profiles"
)

// HostKeys are the checked public keys of a host: the raw X25519 key, the raw Ed25519 key, the JWK
// as the server stores it ({"crv","kty","x"}, built by the server, never the client's object) and
// its RFC 7638 thumbprint (hosts.auth_jkt).
type HostKeys struct {
	Noise   []byte
	AuthRaw []byte
	JWK     []byte
	JKT     string
}

// ParseHostKeys checks the two public keys of host.pair (Design note 4a and 4b of M2-41).
// noiseKey is strict unpadded base64url of exactly 32 bytes and a point that can complete an
// X25519 exchange (a low-order point is refused); jwk has exactly the members kty (OKP), crv
// (Ed25519) and x (strict unpadded base64url of 32 bytes), parsed by kernel/jose. Every finding
// is a field error at /noise_public_key or /auth_public_jwk/<member> with invalid_key or
// unknown_member; the details never repeat a value. The result is valid only without findings.
func ParseHostKeys(noiseKey string, jwk map[string]any) (HostKeys, []errs.FieldError) {
	var out HostKeys
	var findings []errs.FieldError
	if n, ok := x25519Key(noiseKey); ok {
		out.Noise = n
	} else {
		findings = append(findings, errs.FieldError{Pointer: ptrNoise, Code: FieldInvalidKey, Detail: "the Noise key is no X25519 public key"})
	}
	var extra []string
	for name := range jwk {
		if name != jwkKty && name != jwkCrv && name != jwkX {
			extra = append(extra, name)
		}
	}
	slices.Sort(extra)
	for _, name := range extra {
		findings = append(findings, errs.FieldError{Pointer: ptrJWK + escapePointer(name), Code: FieldUnknownMember, Detail: "the JWK carries only kty, crv and x"})
	}
	kty, _ := jwk[jwkKty].(string)
	crv, _ := jwk[jwkCrv].(string)
	x, _ := jwk[jwkX].(string)
	if kty != jwkKtyOKP {
		findings = append(findings, errs.FieldError{Pointer: ptrJWK + jwkKty, Code: FieldInvalidKey, Detail: "kty must be OKP"})
	}
	if crv != jwkCrvEd {
		findings = append(findings, errs.FieldError{Pointer: ptrJWK + jwkCrv, Code: FieldInvalidKey, Detail: "crv must be Ed25519"})
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(x)
	if err != nil || len(raw) != hostKeyBytes {
		findings = append(findings, errs.FieldError{Pointer: ptrJWK + jwkX, Code: FieldInvalidKey, Detail: "x must be 32 bytes base64url without padding"})
	}
	if len(findings) > 0 {
		return HostKeys{}, findings
	}
	canonical, _ := json.Marshal(map[string]string{jwkKty: kty, jwkCrv: crv, jwkX: x}) // strings only: never fails
	k, err := jose.ParseJWK(canonical)
	if err != nil {
		return HostKeys{}, []errs.FieldError{{Pointer: ptrJWK + jwkX, Code: FieldInvalidKey, Detail: "the JWK is no Ed25519 public key"}}
	}
	out.AuthRaw, out.JWK, out.JKT = raw, k.Canonical(), k.Thumbprint()
	return out, nil
}

// x25519Key decodes and checks the Noise key: a low-order point makes the exchange with a fresh
// private key fail, so such a key could never complete a Noise handshake.
func x25519Key(s string) ([]byte, bool) {
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil || len(b) != hostKeyBytes {
		return nil, false
	}
	pub, err := ecdh.X25519().NewPublicKey(b)
	if err != nil {
		return nil, false
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, false
	}
	if _, err := priv.ECDH(pub); err != nil {
		return nil, false
	}
	return b, true
}

// escapePointer escapes a member name for a JSON Pointer (RFC 6901).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// PairReport is the report of host.pair besides the keys (schema HostPairRequest).
type PairReport struct {
	Platform, OSVersion, HostVersion string
	Profiles                         []Profile
	SessionUsers                     []ReportedSessionUser
}

// ReportLimits are the configured bounds of a report: host.os_user_max_chars,
// terminal.report_label_max_chars and terminal.report_max_entries.
type ReportLimits struct {
	OSUserMaxChars, LabelMaxChars, MaxEntries int
}

// profileNamePattern is the pattern of HostProfile.name.
var profileNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// semverPattern is SemVer 2.0.0 without a leading v (semver.org, the published expression).
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// The values of hosts.platform and of HostProfile.kind.
var (
	platforms    = []string{"macos", "linux", "windows"}
	profileKinds = []string{"shell", "claude", "command"}
)

// ValidatePairReport checks the report of host.pair (Design note 4c to 4f of M2-41; HD1 on the
// server): at least one session user, each a session user name (NewOSUser) distinct without
// regard to case, its working directories non-empty display texts of at most
// terminal.report_label_max_chars characters; profile names of the schema pattern, distinct and
// of at most that length, display names alike; each list at most terminal.report_max_entries
// entries; host_version SemVer without v; os_version, when present, a display text of at most
// the label length. It returns every finding in body order, never with the value.
func ValidatePairReport(r PairReport, lim ReportLimits) []errs.FieldError {
	var out []errs.FieldError
	if !slices.Contains(platforms, r.Platform) {
		out = append(out, errs.FieldError{Pointer: "/platform", Code: FieldInvalid, Detail: "the platform must be macos, linux or windows"})
	}
	if r.OSVersion != "" {
		out = appendText(out, "/os_version", "os_version", r.OSVersion, lim.LabelMaxChars)
	}
	if !semverPattern.MatchString(r.HostVersion) {
		out = append(out, errs.FieldError{Pointer: "/host_version", Code: FieldInvalidFormat, Detail: "host_version must be SemVer 2.0.0 without v"})
	}
	out = append(out, checkProfiles(r.Profiles, lim)...)
	return append(out, checkSessionUsers(r.SessionUsers, lim)...)
}

// checkProfiles checks the profiles of a report.
func checkProfiles(list []Profile, lim ReportLimits) []errs.FieldError {
	var out []errs.FieldError
	if len(list) > lim.MaxEntries {
		out = append(out, errs.FieldError{Pointer: ptrProfile, Code: FieldLimitReached, Detail: "too many profiles"})
	}
	seen := map[string]bool{}
	for i, p := range list {
		prefix := ptrProfile + "/" + strconv.Itoa(i)
		switch {
		case !profileNamePattern.MatchString(p.Name):
			out = append(out, errs.FieldError{Pointer: prefix + "/name", Code: FieldPatternMismatch, Detail: "the profile name is outside its pattern"})
		case utf8.RuneCountInString(p.Name) > lim.LabelMaxChars:
			out = append(out, errs.FieldError{Pointer: prefix + "/name", Code: FieldTooLong, Detail: "the profile name is too long"})
		case seen[p.Name]:
			out = append(out, errs.FieldError{Pointer: prefix + "/name", Code: FieldDuplicate, Detail: "the profile name is listed twice"})
		}
		seen[p.Name] = true
		out = appendText(out, prefix+"/display_name", "display name", p.DisplayName, lim.LabelMaxChars)
		if !slices.Contains(profileKinds, p.Kind) {
			out = append(out, errs.FieldError{Pointer: prefix + "/kind", Code: FieldInvalid, Detail: "the profile kind must be shell, claude or command"})
		}
	}
	return out
}

// checkSessionUsers checks the session users of a report.
func checkSessionUsers(list []ReportedSessionUser, lim ReportLimits) []errs.FieldError {
	var out []errs.FieldError
	switch {
	case len(list) == 0:
		return []errs.FieldError{{Pointer: ptrUsers, Code: FieldEmpty, Detail: "a host without session users can never give a terminal"}}
	case len(list) > lim.MaxEntries:
		out = append(out, errs.FieldError{Pointer: ptrUsers, Code: FieldLimitReached, Detail: "too many session users"})
	}
	seen := map[string]bool{}
	for i, u := range list {
		prefix := ptrUsers + "/" + strconv.Itoa(i)
		if _, err := NewOSUser(u.OSUser, lim.OSUserMaxChars, prefix+"/os_user"); err != nil {
			out = append(out, fieldsOf(err)...)
		} else if key := strings.ToLower(u.OSUser); seen[key] {
			out = append(out, errs.FieldError{Pointer: prefix + "/os_user", Code: FieldDuplicate, Detail: "the session user is listed twice"})
		} else {
			seen[key] = true
		}
		if len(u.Workdirs) > lim.MaxEntries {
			out = append(out, errs.FieldError{Pointer: prefix + "/workdirs", Code: FieldLimitReached, Detail: "too many working directories"})
		}
		for j, d := range u.Workdirs {
			out = appendText(out, prefix+"/workdirs/"+strconv.Itoa(j), "working directory", d, lim.LabelMaxChars)
		}
	}
	return out
}

// appendText appends the finding of a display text that must not be blank, must pass SEC-IN-4
// and must have at most maxChars characters; nothing for a good text. The text is not trimmed:
// the host service sends the label it shows.
func appendText(out []errs.FieldError, pointer, what, s string, maxChars int) []errs.FieldError {
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		if errors.As(err, &te) {
			return append(out, errs.FieldError{Pointer: pointer, Code: string(te.Rule), Detail: "the " + what + " contains a forbidden character"})
		}
		return append(out, errs.FieldError{Pointer: pointer, Code: FieldInvalid, Detail: "the " + what + " is not valid"})
	}
	switch {
	case strings.TrimSpace(s) == "":
		return append(out, errs.FieldError{Pointer: pointer, Code: FieldRequired, Detail: "the " + what + " is required"})
	case utf8.RuneCountInString(s) > maxChars:
		return append(out, errs.FieldError{Pointer: pointer, Code: FieldTooLong, Detail: "the " + what + " is too long"})
	}
	return out
}

// fieldsOf returns the field errors of a validation error.
func fieldsOf(err error) []errs.FieldError {
	if e, ok := errs.As(err); ok {
		return e.Fields
	}
	return []errs.FieldError{{Code: FieldInvalid, Detail: "invalid value"}}
}

// NormalizeFingerprintPrefix turns the typed prefix of confirmHost into its compare form (Design
// note 6 of M2-41): upper case; exactly n characters (terminal.fingerprint_confirm_chars), all
// from the Crockford alphabet. Letters outside it (I, L, O, U) and separators are not mapped; the
// client may normalise before sending. Errors: validation_failed at /fingerprint_prefix with
// invalid_format; such an attempt does not count.
func NormalizeFingerprintPrefix(s string, n int) (string, error) {
	up := strings.ToUpper(s)
	if len(up) != n || utf8.RuneCountInString(s) != n {
		return "", FieldError(ptrPrefix, FieldInvalidFormat, "the prefix has the wrong length")
	}
	for i := 0; i < len(up); i++ {
		if !strings.ContainsRune(crockfordAlphabet, rune(up[i])) {
			return "", FieldError(ptrPrefix, FieldInvalidFormat, "the prefix holds a character outside the fingerprint alphabet")
		}
	}
	return up, nil
}

// PrefixMatches compares a normalised prefix with the first characters of the compact form of
// fingerprint in constant time; an empty prefix or one longer than the fingerprint never matches.
func PrefixMatches(fingerprint, prefix string) bool {
	compact := CompactFingerprint(fingerprint)
	if prefix == "" || len(prefix) > len(compact) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(compact[:len(prefix)]), []byte(prefix)) == 1
}

// AttemptOutcome is the effect of one well-formed confirmation attempt on hosts.fingerprint_failures.
type AttemptOutcome struct {
	// Match reports the right prefix.
	Match bool
	// Failures is the value to store: 0 after a match or a fallback, else the raised counter.
	Failures int
	// Remaining is attempts_remaining of a mismatch: 0 means the host falls back to pairing.
	Remaining int
	// Fallback reports the mismatch that reached terminal.fingerprint_max_attempts.
	Fallback bool
}

// NextAttempt counts one attempt (TM-T19): a match resets the counter; a mismatch raises it, and
// the mismatch that reaches maxAttempts (terminal.fingerprint_max_attempts) is the fallback with
// the counter back at 0.
func NextAttempt(failuresBefore, maxAttempts int, match bool) AttemptOutcome {
	if match {
		return AttemptOutcome{Match: true}
	}
	f := failuresBefore + 1
	if f >= maxAttempts {
		return AttemptOutcome{Fallback: true}
	}
	return AttemptOutcome{Failures: f, Remaining: maxAttempts - f}
}

// RepairPlan is the effect of a new pairing code on a host (24 section 3.3, Design note 11 of
// M2-41): the transition into pairing, and whether the stored keys are dropped (the one condition
// of key_changed), the open sessions end with host_rekeyed and the disabled reason is cleared.
type RepairPlan struct {
	Result                               Result
	DropKeys, EndSessions, ClearDisabled bool
}

// RepairEffects returns the plan of issueHostPairingCode on a host in state with disabledReason:
// pairing only replaces the code; confirming, active and disabled also drop the keys; active also
// ends the sessions; disabled (also duplicate_connection, the only way out of it, 24 section 3.4)
// clears its reason. Errors: ErrInvalidTransition for retired.
func RepairEffects(state State, disabledReason string) (RepairPlan, error) {
	r, err := Transition(state, disabledReason, CauseNewPairingCode)
	if err != nil {
		return RepairPlan{}, err
	}
	return RepairPlan{
		Result: r, DropKeys: state != StatePairing, EndSessions: state == StateActive, ClearDisabled: state == StateDisabled,
	}, nil
}

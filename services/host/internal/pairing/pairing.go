// SPDX-License-Identifier: Apache-2.0

// Package pairing is "bothy-host pair" (docs/spec/24-terminal.md 3.2, docs/DECISIONS.md
// D-M2-42-16): it generates the X25519 key pair (Noise) and the Ed25519 key pair (login) locally,
// redeems the pairing code through pairHost with both public keys, the platform, the versions and
// the lists after HD7, checks the fingerprint the server returns against its own computation, and
// stores the keys and the pairing record only after the server accepted them. The pairing code,
// the private keys and the request body appear in no output, log or error.
package pairing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/fingerprint"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Errors of Pair; their texts are the lines the command prints.
var (
	ErrNotHTTPS            = errors.New("pair: the server URL must use https")
	ErrFingerprintMismatch = errors.New("pair: fingerprint mismatch")
	ErrCAFile              = errors.New("pair: the CA file holds no certificate")
)

// FieldError is one errors[] item of a validation_failed answer.
type FieldError struct {
	Code    string `json:"code"`
	Pointer string `json:"pointer"`
}

// RefusedError is a problem answer of the server.
type RefusedError struct {
	Status int
	Code   string
	Reason string
	Errors []FieldError
}

func (e *RefusedError) Error() string {
	var b strings.Builder
	b.WriteString("pair: server refused: " + e.Code)
	if e.Reason != "" {
		b.WriteString(" (" + e.Reason + ")")
	}
	for _, f := range e.Errors {
		b.WriteString("\n  " + f.Code + " at " + f.Pointer)
	}
	if e.Status == http.StatusUnauthorized {
		b.WriteString("\npair: ask an owner for a new pairing code")
	}
	return b.String()
}

// Lists are the profiles and session users after HD7 that pair reports.
type Lists struct {
	Profiles     []doctor.Profile
	SessionUsers []doctor.ReportedSessionUser
}

// Options configure Pair.
type Options struct {
	ServerURL, Code string
	// CAPEM is the content of --ca-file, trusted in addition to the system roots.
	CAPEM []byte
	State *state.Dir
	Lists Lists
	// Platform, OSVersion and HostVersion are HostPairRequest members.
	Platform, OSVersion, HostVersion string
	// Random is the source of the key pairs.
	Random io.Reader
	// Transport overrides the HTTP transport (tests); nil uses the TLS roots above.
	Transport http.RoundTripper
}

// Result is a successful pairing.
type Result struct {
	HostID, InstanceID, Fingerprint string
}

// maxAnswer bounds the answer body read from the server (protocol constant of this client).
const maxAnswer = 1 << 20

type pairRequest struct {
	PairingCode    string                       `json:"pairing_code"`
	NoisePublicKey string                       `json:"noise_public_key"`
	AuthPublicJWK  map[string]string            `json:"auth_public_jwk"`
	Platform       string                       `json:"platform"`
	OSVersion      string                       `json:"os_version,omitempty"`
	HostVersion    string                       `json:"host_version"`
	Profiles       []doctor.Profile             `json:"profiles"`
	SessionUsers   []doctor.ReportedSessionUser `json:"session_users"`
}

// Body returns the request body of pairHost for the public keys; it holds no private member.
func body(o Options, noisePub, authPub []byte) ([]byte, error) {
	profiles, users := o.Lists.Profiles, o.Lists.SessionUsers
	if profiles == nil {
		profiles = []doctor.Profile{}
	}
	if users == nil {
		users = []doctor.ReportedSessionUser{}
	}
	enc := base64.RawURLEncoding
	return json.Marshal(pairRequest{
		PairingCode: o.Code, NoisePublicKey: enc.EncodeToString(noisePub),
		AuthPublicJWK: map[string]string{"kty": "OKP", "crv": "Ed25519", "x": enc.EncodeToString(authPub)},
		Platform:      o.Platform, OSVersion: o.OSVersion, HostVersion: o.HostVersion, Profiles: profiles, SessionUsers: users,
	})
}

func (o Options) client() (*http.Client, error) {
	if o.Transport != nil {
		return &http.Client{Transport: o.Transport, CheckRedirect: noRedirect}, nil // nosemgrep: bothy-go-egress-only -- the host module has no egress package; pair calls only the https URL the operator gives (24 3.2, D-M2-42-25)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if len(o.CAPEM) > 0 && !roots.AppendCertsFromPEM(o.CAPEM) {
		return nil, ErrCAFile
	}
	return &http.Client{CheckRedirect: noRedirect, Transport: &http.Transport{ // nosemgrep: bothy-go-egress-only -- the host module has no egress package; pair calls only the https URL the operator gives (24 3.2, D-M2-42-25)
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		Proxy:           nil, // never from the environment
	}}, nil
}

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// Pair runs the pairing. It writes nothing to the state directory unless the server answered 201
// with the fingerprint of the generated keys.
func Pair(ctx context.Context, o Options) (Result, error) {
	u, err := url.Parse(o.ServerURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return Result{}, ErrNotHTTPS
	}
	kp, err := noise.GenerateKeyPair(o.Random)
	if err != nil {
		return Result{}, fmt.Errorf("pair: %w", err)
	}
	authPub, authPriv, err := ed25519.GenerateKey(o.Random)
	if err != nil {
		return Result{}, fmt.Errorf("pair: %w", err)
	}
	fp, err := fingerprint.Compute(kp.Public[:], authPub)
	if err != nil {
		return Result{}, fmt.Errorf("pair: %w", err)
	}
	b, err := body(o, kp.Public[:], authPub)
	if err != nil {
		return Result{}, fmt.Errorf("pair: %w", err)
	}
	status, answer, err := o.post(ctx, strings.TrimSuffix(u.String(), "/")+"/api/v1/hosts/pair", b)
	if err != nil {
		return Result{}, err
	}
	if status != http.StatusCreated {
		return Result{}, refused(status, answer)
	}
	var res struct {
		HostID         string `json:"host_id"`
		KeyFingerprint string `json:"key_fingerprint"`
		InstanceID     string `json:"instance_id"`
	}
	if err := json.Unmarshal(answer, &res); err != nil || res.HostID == "" {
		return Result{}, errors.New("pair: unreadable answer of the server")
	}
	if fingerprint.Compact(res.KeyFingerprint) != fingerprint.Compact(fp) {
		return Result{}, ErrFingerprintMismatch
	}
	keys := state.Keys{NoisePrivate: kp.Private, AuthPrivate: authPriv}
	if err := o.State.WriteKeys(keys); err != nil {
		return Result{}, err
	}
	if err := o.State.WritePairing(state.Pairing{ServerURL: u.String(), HostID: res.HostID, InstanceID: res.InstanceID, Fingerprint: fp}); err != nil {
		return Result{}, err
	}
	// The control connection of run must trust the same server (M2-43).
	if err := o.State.WriteServerCA(o.CAPEM); err != nil {
		return Result{}, err
	}
	return Result{HostID: res.HostID, InstanceID: res.InstanceID, Fingerprint: fp}, nil
}

// post sends body to endpoint and returns status and answer; errors carry no URL.
func (o Options) post(ctx context.Context, endpoint string, body []byte) (int, []byte, error) {
	client, err := o.client()
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, ErrNotHTTPS
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, application/problem+json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("pair: server unreachable: %w", scrub(err))
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer))
	if err != nil {
		return 0, nil, fmt.Errorf("pair: %w", scrub(err))
	}
	return resp.StatusCode, answer, nil
}

// scrub drops the URL from a transport error; the error type and its last cause stay.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func refused(status int, answer []byte) error {
	var p struct {
		Code   string       `json:"code"`
		Reason string       `json:"reason"`
		Errors []FieldError `json:"errors"`
	}
	if json.Unmarshal(answer, &p) != nil || p.Code == "" {
		p.Code = "http_" + fmt.Sprint(status)
	}
	return &RefusedError{Status: status, Code: p.Code, Reason: p.Reason, Errors: p.Errors}
}

// ReadCA reads the file of --ca-file; an empty path gives no extra roots.
func ReadCA(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path) //nolint:gosec // the file the operator names
	if err != nil {
		return nil, fmt.Errorf("pair: CA file: %w", err)
	}
	return b, nil
}

// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// The configuration keys of the relying party (docs/spec/08-identity.md section 18); errors
// name them, never their values.
const (
	keyWebAuthnRPID    = "auth.webauthn_rp_id"
	keyWebAuthnOrigins = "auth.webauthn_origins"
)

// RelyingParty is the WebAuthn relying party of the instance (08 "2.5", brief M1-40 "Relying party
// configuration"): the RP ID and the allowed origins. Enabled is false when auth.webauthn_rp_id is
// empty and the host of server.public_url is an IP address, which can never be an RP ID: such an
// instance has no passkeys (docs/DECISIONS.md M1-40).
type RelyingParty struct {
	// ID is the RP ID, a domain in lower case.
	ID string
	// Origins are the allowed origins: http(s) origins and opaque app origins such as
	// android:apk-key-hash:...
	Origins []string
	// Enabled reports whether passkeys are available.
	Enabled bool
}

// ResolveRelyingParty returns the relying party of server.public_url publicURL,
// auth.webauthn_rp_id rpID and auth.webauthn_origins origins and checks it (start validation):
// an empty rpID means the host of publicURL, empty origins mean the origin of publicURL. The RP ID
// must be a domain; every http(s) origin must be an origin whose host equals the RP ID or ends in
// "." plus the RP ID (a registrable suffix), and http only for localhost; any other entry is an
// opaque app origin. The error names the key, never a value.
//
// Example:
//
//	rp, err := domain.ResolveRelyingParty("https://chat.example.com", "", nil)
//	// rp.ID == "chat.example.com", rp.Origins == []string{"https://chat.example.com"}
func ResolveRelyingParty(publicURL, rpID string, origins []string) (RelyingParty, error) {
	pub, err := url.Parse(publicURL)
	if err != nil || pub.Hostname() == "" {
		return RelyingParty{}, errors.New("server.public_url must be an origin for the WebAuthn relying party")
	}
	if rpID == "" {
		host := strings.ToLower(pub.Hostname())
		if _, err := netip.ParseAddr(host); err == nil {
			return RelyingParty{}, nil
		}
		if len(origins) == 0 && pub.Scheme == "http" && host != localhost || !validRPID(host) {
			// No secure context or a host that is no valid RP ID (a single label such as an
			// internal host name): browsers offer no passkeys there, and an operator who set
			// nothing keeps a working start.
			return RelyingParty{}, nil
		}
		rpID = host
	}
	if !validRPID(rpID) {
		return RelyingParty{}, errors.New(keyWebAuthnRPID + " must be a domain name in lower case, for example chat.example.com")
	}
	if len(origins) == 0 {
		origins = []string{strings.ToLower(pub.Scheme + "://" + pub.Host)}
	}
	for i, o := range origins {
		if err := checkOrigin(rpID, o); err != nil {
			return RelyingParty{}, fmt.Errorf("%s entry %d %w", keyWebAuthnOrigins, i+1, err)
		}
	}
	return RelyingParty{ID: rpID, Origins: append([]string(nil), origins...), Enabled: true}, nil
}

// localhost is the one host an http origin may name (a secure context without TLS).
const localhost = "localhost"

// validRPID reports a domain: dot-separated labels of lower case letters, digits and hyphens, no
// label empty or with a hyphen at either end, not an IP address, and either localhost or at least
// two labels (a single label such as "com" is no registrable domain).
func validRPID(id string) bool {
	if id == localhost {
		return true
	}
	if _, err := netip.ParseAddr(id); err == nil || len(id) > 253 {
		return false
	}
	labels := strings.Split(id, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if !validLabel(l) {
			return false
		}
	}
	return true
}

// validLabel reports a DNS label of 1 to 63 lower case letters, digits and hyphens, without a
// hyphen at either end.
func validLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for _, c := range l {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// checkOrigin checks one entry of auth.webauthn_origins against rpID.
func checkOrigin(rpID, o string) error {
	if !strings.HasPrefix(o, "http://") && !strings.HasPrefix(o, "https://") {
		// An opaque app origin (for example android:apk-key-hash:...): the WebAuthn adapter checks
		// its prefix against the forms clients convey.
		if !strings.Contains(o, ":") || strings.ContainsAny(o, " \t\r\n") {
			return errors.New("must be an http(s) origin or an app origin such as android:apk-key-hash:<hash>")
		}
		return nil
	}
	u, err := url.Parse(o)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		o != u.Scheme+"://"+u.Host {
		return errors.New("must be an origin scheme://host[:port]")
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "http" && host != localhost {
		return errors.New("may use http only for localhost")
	}
	if host != rpID && !strings.HasSuffix(host, "."+rpID) {
		return errors.New("must have the RP ID " + keyWebAuthnRPID + " as its host or as a suffix of its host")
	}
	return nil
}

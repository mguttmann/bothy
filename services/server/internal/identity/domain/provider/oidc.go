// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// The client authentication methods of identity_provider.oidc.v1 (08 section 2.6).
const (
	// ClientAuthPrivateKeyJWT signs a client assertion with the key of client_key_ref (preferred).
	ClientAuthPrivateKeyJWT = "private_key_jwt"
	// ClientAuthSecretBasic sends the secret of client_secret_ref with HTTP Basic.
	ClientAuthSecretBasic = "client_secret_basic"
)

// schemeHTTPS is the only scheme of an issuer and of a claimed redirect.
const schemeHTTPS = "https"

// TenantPlaceholder is the placeholder of a multi-tenant issuer (08 section 2.3).
const TenantPlaceholder = "{tenantid}"

// DefaultClientKeyAlg is the algorithm of the client assertion when client_key_alg is absent.
const DefaultClientKeyAlg = "PS256"

// The members of identity_provider.oidc.v1.
const (
	oidcIssuer          = "issuer"
	oidcClientID        = "client_id"
	oidcClientAuth      = "client_auth"
	oidcClientSecretRef = "client_secret_ref"
	oidcClientKeyRef    = "client_key_ref"
	oidcClientKeyAlg    = "client_key_alg"
	oidcClientKeyID     = "client_key_id"
	oidcClientCertX5t   = "client_cert_x5t_s256"
	oidcAllowedTenants  = "allowed_tenant_ids"
	oidcAppRedirectURIs = "app_redirect_uris"
	oidcGroupsClaim     = "groups_claim"
)

// oidcMembers is the closed member list of the schema.
var oidcMembers = []string{
	oidcIssuer, oidcClientID, oidcClientAuth, oidcClientSecretRef, oidcClientKeyRef, oidcClientKeyAlg, oidcClientKeyID,
	oidcClientCertX5t, oidcAllowedTenants, oidcAppRedirectURIs, oidcGroupsClaim,
}

// OIDCConfig is identity_provider.oidc.v1 after normalization; the field order is the stored member
// order. Secrets appear only as references.
type OIDCConfig struct {
	Issuer            string   `json:"issuer"`
	ClientID          string   `json:"client_id"`
	ClientAuth        string   `json:"client_auth"`
	ClientSecretRef   string   `json:"client_secret_ref,omitempty"`
	ClientKeyRef      string   `json:"client_key_ref,omitempty"`
	ClientKeyAlg      string   `json:"client_key_alg,omitempty"`
	ClientKeyID       string   `json:"client_key_id,omitempty"`
	ClientCertX5tS256 string   `json:"client_cert_x5t_s256,omitempty"`
	AllowedTenantIDs  []string `json:"allowed_tenant_ids,omitempty"`
	AppRedirectURIs   []string `json:"app_redirect_uris,omitempty"`
	GroupsClaim       string   `json:"groups_claim,omitempty"`
}

// tenantIDPattern is the form of an allowed tenant id: it replaces the placeholder in a URL path,
// so it holds no character that would change the path.
var tenantIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// privateSchemePattern is a private-use URI scheme (RFC 8252 section 7.1: a reverse domain name,
// so it contains a dot).
var privateSchemePattern = regexp.MustCompile(`^[a-z][a-z0-9+-]*(\.[a-z0-9+-]+)+$`)

// multiTenantAliases are the Entra path segments that no token carries as iss; a multi-tenant
// provider names {tenantid} with allowed_tenant_ids instead (08 section 2.3).
var multiTenantAliases = []string{"common", "organizations", "consumers"}

// ValidateOIDC validates identity_provider.oidc.v1 (08 section 2.6) and returns the normalized
// configuration: client_key_alg gets its default PS256 with private_key_jwt. Secret references are
// checked with refs for their syntax only.
func ValidateOIDC(raw json.RawMessage, refs SecretRefs) (json.RawMessage, error) {
	m, err := members(raw, oidcMembers)
	if err != nil {
		return nil, err
	}
	var s fieldSet
	var c OIDCConfig
	multi := s.oidcIssuer(m, &c)
	if s.str(m, oidcClientID, &c.ClientID) {
		s.text(memberPointer(oidcClientID), c.ClientID)
	} else if _, ok := m[oidcClientID]; !ok {
		s.add(memberPointer(oidcClientID), CodeRequired, "The member is required.")
	}
	s.oidcClientAuth(m, &c, refs)
	s.oidcTenants(m, &c, multi)
	s.oidcRedirects(m, &c)
	if s.str(m, oidcGroupsClaim, &c.GroupsClaim) {
		s.text(memberPointer(oidcGroupsClaim), c.GroupsClaim)
	}
	if err := s.err(); err != nil {
		return nil, err
	}
	return canonical(c)
}

// oidcIssuer checks issuer: https, a host, no user info, query or fragment, at most one
// placeholder and only in the path, no multi-tenant alias segment. It reports whether the issuer
// names the placeholder.
func (s *fieldSet) oidcIssuer(m map[string]json.RawMessage, c *OIDCConfig) bool {
	p := memberPointer(oidcIssuer)
	if !s.str(m, oidcIssuer, &c.Issuer) {
		if _, ok := m[oidcIssuer]; !ok {
			s.add(p, CodeRequired, "The member is required.")
		}
		return false
	}
	multi := strings.Contains(c.Issuer, TenantPlaceholder)
	const stand = "tenant-placeholder"
	parsed, err := url.Parse(strings.Replace(c.Issuer, TenantPlaceholder, stand, 1))
	switch {
	case c.Issuer == "":
		s.add(p, CodeRequired, "The member is required.")
	case err != nil || parsed.Scheme != schemeHTTPS || parsed.Host == "" || parsed.Opaque != "":
		s.add(p, CodeInvalid, "The issuer must be an absolute https URL.")
	case parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.Contains(c.Issuer, "#"):
		s.add(p, CodeInvalid, "The issuer must not have user information, a query or a fragment.")
	case strings.Count(c.Issuer, TenantPlaceholder) > 1 || (multi && !strings.Contains(parsed.Path, stand)):
		s.add(p, CodeInvalid, "The issuer may name "+TenantPlaceholder+" once, in its path.")
	case hasAliasSegment(parsed.Path):
		s.add(p, CodeInvalid, "A multi-tenant issuer names "+TenantPlaceholder+" with allowed_tenant_ids instead of common, organizations or consumers.")
	}
	return multi
}

// hasAliasSegment reports a path segment that is a multi-tenant alias.
func hasAliasSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		for _, a := range multiTenantAliases {
			if strings.EqualFold(seg, a) {
				return true
			}
		}
	}
	return false
}

// oidcClientAuth checks client_auth and the members that belong to exactly one method.
func (s *fieldSet) oidcClientAuth(m map[string]json.RawMessage, c *OIDCConfig, refs SecretRefs) {
	p := memberPointer(oidcClientAuth)
	if !s.str(m, oidcClientAuth, &c.ClientAuth) {
		if _, ok := m[oidcClientAuth]; !ok {
			s.add(p, CodeRequired, "The member is required.")
		}
		return
	}
	switch c.ClientAuth {
	case ClientAuthSecretBasic:
		s.oidcSecretBasic(m, c, refs)
	case ClientAuthPrivateKeyJWT:
		s.oidcPrivateKeyJWT(m, c, refs)
	default:
		s.add(p, CodeInvalidEnum, "The value must be private_key_jwt or client_secret_basic.")
	}
}

// oidcSecretBasic checks the members of client_secret_basic: the secret reference, and none of
// the members of private_key_jwt.
func (s *fieldSet) oidcSecretBasic(m map[string]json.RawMessage, c *OIDCConfig, refs SecretRefs) {
	s.ref(m, oidcClientSecretRef, &c.ClientSecretRef, refs)
	for _, name := range []string{oidcClientKeyRef, oidcClientKeyAlg, oidcClientKeyID, oidcClientCertX5t} {
		if _, ok := m[name]; ok {
			s.add(memberPointer(name), CodeNotAllowed, "The member belongs to private_key_jwt.")
		}
	}
}

// oidcPrivateKeyJWT checks the members of private_key_jwt: the key reference, the algorithm
// (default PS256), the key id and the certificate thumbprint, and no secret reference.
func (s *fieldSet) oidcPrivateKeyJWT(m map[string]json.RawMessage, c *OIDCConfig, refs SecretRefs) {
	s.ref(m, oidcClientKeyRef, &c.ClientKeyRef, refs)
	if _, ok := m[oidcClientSecretRef]; ok {
		s.add(memberPointer(oidcClientSecretRef), CodeNotAllowed, "The member belongs to client_secret_basic.")
	}
	c.ClientKeyAlg = DefaultClientKeyAlg
	if s.str(m, oidcClientKeyAlg, &c.ClientKeyAlg) && c.ClientKeyAlg != "PS256" && c.ClientKeyAlg != "RS256" && c.ClientKeyAlg != "ES256" {
		s.add(memberPointer(oidcClientKeyAlg), CodeInvalidEnum, "The algorithm must be PS256, RS256 or ES256.")
	}
	if s.str(m, oidcClientKeyID, &c.ClientKeyID) {
		s.text(memberPointer(oidcClientKeyID), c.ClientKeyID)
	}
	if s.str(m, oidcClientCertX5t, &c.ClientCertX5tS256) {
		if b, err := base64.RawURLEncoding.DecodeString(c.ClientCertX5tS256); err != nil || len(b) != 32 {
			s.add(memberPointer(oidcClientCertX5t), CodeInvalid, "The value must be a SHA-256 thumbprint in base64url without padding.")
		}
	}
}

// ref checks the required secret reference name of m with refs.
func (s *fieldSet) ref(m map[string]json.RawMessage, name string, dst *string, refs SecretRefs) {
	p := memberPointer(name)
	if !s.str(m, name, dst) {
		if _, ok := m[name]; !ok {
			s.add(p, CodeRequired, "The member is required.")
		}
		return
	}
	if refs == nil || refs.Valid(*dst) != nil {
		s.add(p, CodeInvalid, "The value must be a secret reference such as env:NAME or file:/path.")
	}
}

// oidcTenants checks allowed_tenant_ids: required and not empty with the placeholder, not allowed
// without it, each a plain unique token.
func (s *fieldSet) oidcTenants(m map[string]json.RawMessage, c *OIDCConfig, multi bool) {
	p := memberPointer(oidcAllowedTenants)
	_, present := m[oidcAllowedTenants]
	if !multi {
		if present {
			s.add(p, CodeNotAllowed, "The member needs "+TenantPlaceholder+" in the issuer.")
		}
		return
	}
	if !s.strings(m, oidcAllowedTenants, &c.AllowedTenantIDs) {
		if !present {
			s.add(p, CodeRequired, "An issuer with "+TenantPlaceholder+" needs allowed_tenant_ids.")
		}
		return
	}
	if len(c.AllowedTenantIDs) == 0 {
		s.add(p, CodeRequired, "An issuer with "+TenantPlaceholder+" needs allowed_tenant_ids.")
	}
	seen := map[string]bool{}
	for i, v := range c.AllowedTenantIDs {
		ip := p + "/" + strconv.Itoa(i)
		switch {
		case !tenantIDPattern.MatchString(v):
			s.add(ip, CodeInvalid, "A tenant id holds letters, digits, dot, underscore and hyphen only.")
		case seen[strings.ToLower(v)]:
			s.add(ip, CodeDuplicateKey, "The tenant id appears more than once.")
		}
		seen[strings.ToLower(v)] = true
	}
}

// oidcRedirects checks app_redirect_uris: each a claimed https URL or a private-use scheme
// (RFC 8252 sections 7.1 and 7.2), without fragment, unique. Loopback redirects are not listed;
// the fixed rule of 08 section 2.2 allows them.
func (s *fieldSet) oidcRedirects(m map[string]json.RawMessage, c *OIDCConfig) {
	p := memberPointer(oidcAppRedirectURIs)
	if !s.strings(m, oidcAppRedirectURIs, &c.AppRedirectURIs) {
		return
	}
	seen := map[string]bool{}
	for i, v := range c.AppRedirectURIs {
		ip := p + "/" + strconv.Itoa(i)
		u, err := url.Parse(v)
		switch {
		case err != nil || u.Fragment != "" || strings.Contains(v, "#") || u.User != nil:
			s.add(ip, CodeInvalid, "The redirect must be a URI without user information or fragment.")
		case u.Scheme == schemeHTTPS && u.Host == "":
			s.add(ip, CodeInvalid, "An https redirect needs a host.")
		case u.Scheme != schemeHTTPS && !privateSchemePattern.MatchString(u.Scheme):
			s.add(ip, CodeInvalid, "The redirect must use https or a private-use scheme in reverse domain form.")
		case seen[v]:
			s.add(ip, CodeDuplicateKey, "The redirect appears more than once.")
		}
		seen[v] = true
	}
}

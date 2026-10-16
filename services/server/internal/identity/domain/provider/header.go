// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"encoding/json"
	"net/textproto"
	"strings"
)

// The members of identity_provider.trusted_header.v1.
const (
	headerLogin = "login_header"
	headerName  = "name_header"
)

// TrustedHeaderConfig is identity_provider.trusted_header.v1 after normalization: the header names
// in canonical form, because HTTP field names are case-insensitive (RFC 9110 section 5.1).
type TrustedHeaderConfig struct {
	LoginHeader string `json:"login_header"`
	NameHeader  string `json:"name_header,omitempty"`
}

// ValidateTrustedHeader validates identity_provider.trusted_header.v1 (08 section 2.6):
// login_header is required and name_header optional, both an HTTP field name token (RFC 9110
// section 5.1), and the two differ. It returns the configuration with canonical header names.
// The loopback rule of 08 section 2.4 belongs to the driver (M1-43), not to the schema.
func ValidateTrustedHeader(raw json.RawMessage) (json.RawMessage, error) {
	m, err := members(raw, []string{headerLogin, headerName})
	if err != nil {
		return nil, err
	}
	var s fieldSet
	var c TrustedHeaderConfig
	if !s.str(m, headerLogin, &c.LoginHeader) {
		if _, ok := m[headerLogin]; !ok {
			s.add(memberPointer(headerLogin), CodeRequired, "The member is required.")
		}
	} else if s.token(memberPointer(headerLogin), c.LoginHeader) {
		c.LoginHeader = textproto.CanonicalMIMEHeaderKey(c.LoginHeader)
	}
	if s.str(m, headerName, &c.NameHeader) && s.token(memberPointer(headerName), c.NameHeader) {
		if strings.EqualFold(c.NameHeader, c.LoginHeader) {
			s.add(memberPointer(headerName), CodeInvalid, "The name header must differ from the login header.")
		}
		c.NameHeader = textproto.CanonicalMIMEHeaderKey(c.NameHeader)
	}
	if err := s.err(); err != nil {
		return nil, err
	}
	return canonical(c)
}

// token checks an HTTP field name: one or more tchar (RFC 9110 section 5.6.2).
func (s *fieldSet) token(pointer, v string) bool {
	if v == "" {
		s.add(pointer, CodeRequired, "The member must not be empty.")
		return false
	}
	for i := 0; i < len(v); i++ {
		if !isTchar(v[i]) {
			s.add(pointer, CodeInvalid, "The value must be an HTTP field name (RFC 9110 token).")
			return false
		}
	}
	return true
}

// isTchar reports a tchar of RFC 9110 section 5.6.2.
func isTchar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

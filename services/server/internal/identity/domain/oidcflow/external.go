// SPDX-License-Identifier: Apache-2.0

package oidcflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

// SchemaExternalIdentity is the schema name of auth_login_flows.external_identity (08 section 2.2
// step 6).
const SchemaExternalIdentity = "external_identity.v1"

// ExternalIdentity is the verified identity the callback stores in the flow until complete
// (schema external_identity.v1). EmailHint and the names are for display and the handle of a
// provisioned person only, never for mapping or rights (R-ID-02).
type ExternalIdentity struct {
	// Issuer is the iss the token was matched against.
	Issuer string
	// Subject is sub.
	Subject string
	// DisplayName is the claim name, else preferred_username, else empty.
	DisplayName string
	// PreferredUsername is the claim preferred_username; the handle of a provisioned person
	// starts from its local part.
	PreferredUsername string
	// EmailHint is the claim email.
	EmailHint string
	// AuthTime is the claim auth_time; zero when the provider sent none. iat never stands in for
	// it: the time a token was issued says nothing about when the person signed in, so a re-login
	// without auth_time proves no new sign-in (08 section 6.2).
	AuthTime time.Time
	// AMR are the authentication methods the provider names.
	AMR []string
	// GroupsState is GroupsPresent, GroupsAbsent or GroupsOverage for a provider with
	// groups_claim, empty for one without (no claim sync, M1-42).
	GroupsState string
	// Groups are the values of a present group claim, sorted and without duplicates. They can be
	// names, so they are never logged and never written to audit details.
	Groups []string
}

// externalJSON is the stored form; empty members stay out.
type externalJSON struct {
	Schema            string   `json:"schema"`
	Issuer            string   `json:"issuer"`
	Subject           string   `json:"subject"`
	DisplayName       string   `json:"display_name,omitempty"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	EmailHint         string   `json:"email_hint,omitempty"`
	AuthTime          int64    `json:"auth_time,omitempty"`
	AMR               []string `json:"amr,omitempty"`
	GroupsState       string   `json:"groups_state,omitempty"`
	Groups            []string `json:"groups,omitempty"`
}

// MarshalJSON writes the schema external_identity.v1 with auth_time in Unix seconds; without an
// AuthTime the member stays out.
func (x ExternalIdentity) MarshalJSON() ([]byte, error) {
	j := externalJSON{
		Schema: SchemaExternalIdentity, Issuer: x.Issuer, Subject: x.Subject, DisplayName: x.DisplayName,
		PreferredUsername: x.PreferredUsername, EmailHint: x.EmailHint, AMR: x.AMR,
		GroupsState: x.GroupsState, Groups: x.Groups,
	}
	if !x.AuthTime.IsZero() {
		j.AuthTime = x.AuthTime.Unix()
	}
	return json.Marshal(j)
}

// errExternal reports a stored external identity that does not follow the schema.
var errExternal = errors.New("oidcflow: stored external identity does not follow external_identity.v1")

// ParseExternalIdentity reads a stored external identity; unknown members, another schema, a
// missing issuer or subject, an unknown groups_state and groups without groups_state "present"
// are errors. A missing auth_time gives the zero AuthTime.
func ParseExternalIdentity(b []byte) (ExternalIdentity, error) {
	var j externalJSON
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&j); err != nil || j.Schema != SchemaExternalIdentity || j.Issuer == "" || j.Subject == "" {
		return ExternalIdentity{}, errExternal
	}
	switch j.GroupsState {
	case "", GroupsAbsent, GroupsOverage:
		if j.Groups != nil {
			return ExternalIdentity{}, errExternal
		}
	case GroupsPresent:
	default:
		return ExternalIdentity{}, errExternal
	}
	x := ExternalIdentity{
		Issuer: j.Issuer, Subject: j.Subject, DisplayName: j.DisplayName, PreferredUsername: j.PreferredUsername,
		EmailHint: j.EmailHint, AMR: j.AMR, GroupsState: j.GroupsState, Groups: j.Groups,
	}
	if j.AuthTime != 0 {
		x.AuthTime = time.Unix(j.AuthTime, 0).UTC()
	}
	return x, nil
}

// ExternalFromClaims builds the external identity of checked claims c matched against issuer
// (brief M1-41 "handleOidcCallback" item 6).
func ExternalFromClaims(c Claims, issuer string) ExternalIdentity {
	x := ExternalIdentity{
		Issuer: issuer, Subject: c.Subject, DisplayName: c.Name, PreferredUsername: c.PreferredUsername,
		EmailHint: c.Email, AuthTime: c.AuthTime, AMR: c.AMR,
	}
	if x.DisplayName == "" {
		x.DisplayName = c.PreferredUsername
	}
	return x
}

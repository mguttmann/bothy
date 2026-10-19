// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// SchemaVersion is the only value of schema_version (app_manifest.v1, 11 section 3.1).
const SchemaVersion = 1

// The delivery modes of delivery.mode.
const (
	DeliveryGateway = "gateway"
	DeliveryWebhook = "webhook"
)

// Manifest is the typed form of app_manifest.v1 (11 section 3.1, schema AppManifest of
// docs/api/openapi.yaml). Optional members that are absent are zero values; Delivery is nil when
// the block is absent.
type Manifest struct {
	SchemaVersion    int               `json:"schema_version"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Publisher        string            `json:"publisher"`
	Version          string            `json:"version"`
	HomepageURL      string            `json:"homepage_url,omitempty"`
	MinAPIVersion    string            `json:"min_api_version"`
	Scopes           ManifestScopes    `json:"scopes"`
	Events           []string          `json:"events,omitempty"`
	Delivery         *Delivery         `json:"delivery,omitempty"`
	Commands         []Command         `json:"commands,omitempty"`
	Components       []string          `json:"components,omitempty"`
	RequestedGrants  []RequestedGrant  `json:"requested_grants,omitempty"`
	IncomingWebhooks []IncomingWebhook `json:"incoming_webhooks,omitempty"`
	DataEgress       string            `json:"data_egress,omitempty"`
}

// ManifestScopes are scopes.required and scopes.optional.
type ManifestScopes struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

// Delivery is the delivery block: how the app receives events and interactions.
type Delivery struct {
	Mode            string `json:"mode"`
	EventsURL       string `json:"events_url,omitempty"`
	InteractionsURL string `json:"interactions_url,omitempty"`
}

// Command is one command of the manifest (schema CommandRegister, 09 section 10.2).
type Command struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	ScopeKind     string          `json:"scope_kind,omitempty"`
	ScopeID       string          `json:"scope_id,omitempty"`
	Options       []CommandOption `json:"options,omitempty"`
	RequiredLevel string          `json:"required_level,omitempty"`
	Control       *bool           `json:"control,omitempty"`
}

// CommandOption is one option of a command (schema CommandOption).
type CommandOption struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Required     bool            `json:"required"`
	Choices      []CommandChoice `json:"choices,omitempty"`
	Autocomplete *bool           `json:"autocomplete,omitempty"`
}

// CommandChoice is one fixed choice of an option (schema CommandChoice).
type CommandChoice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RequestedGrant is one entry of requested_grants: what the app asks for; the owner chooses the
// targets with grant.create (11 section 4.1).
type RequestedGrant struct {
	TargetKind   string   `json:"target_kind"`
	Level        string   `json:"level"`
	Capabilities []string `json:"capabilities,omitempty"`
	Purpose      string   `json:"purpose"`
}

// IncomingWebhook is one expected incoming webhook of the manifest.
type IncomingWebhook struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

// AllScopes returns scopes.required and scopes.optional, sorted and without duplicates.
func (m Manifest) AllScopes() []string {
	out := append(slices.Clone(m.Scopes.Required), m.Scopes.Optional...)
	slices.Sort(out)
	return slices.Compact(out)
}

// DeliveryHosts returns the lower-case hosts of delivery.events_url and
// delivery.interactions_url, sorted and without duplicates; an address that does not parse is
// left out (the rules report it).
func (m Manifest) DeliveryHosts() []string {
	if m.Delivery == nil {
		return nil
	}
	var out []string
	for _, raw := range []string{m.Delivery.EventsURL, m.Delivery.InteractionsURL} {
		if raw == "" {
			continue
		}
		if u, ok := parseHTTPS(raw); ok {
			out = append(out, strings.ToLower(u.Hostname()))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// CommandNames returns the lower-case names of the commands, sorted and without duplicates.
func (m Manifest) CommandNames() []string {
	out := make([]string, 0, len(m.Commands))
	for _, c := range m.Commands {
		out = append(out, strings.ToLower(c.Name))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Hash returns the SHA-256 of the manifest bytes as received (11 section 3.1, M2-24 design note
// 14): the hash names exactly what the owner uploaded.
func Hash(raw []byte) [sha256.Size]byte { return sha256.Sum256(raw) }

// ParseManifest decodes raw into a Manifest. Unknown members are an error (11 section 3.1); the
// request validation already refused them, so an error here is a body that bypassed it.
//
// Errors: validation_failed with the pointer "" and code invalid.
func ParseManifest(raw []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, invalidManifest()
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Manifest{}, invalidManifest()
	}
	return m, nil
}

// invalidManifest is the error of a manifest that is no app_manifest.v1 document.
func invalidManifest() error {
	return errs.New(errs.CodeValidationFailed, "The manifest is not an app_manifest.v1 document.").
		WithFields(errs.FieldError{Pointer: "", Code: CodeInvalid, Detail: "The manifest is not an app_manifest.v1 document."})
}

// SameContent reports whether a and b are the same JSON document the way jsonb compares them:
// member order and white space do not count. Two documents that do not decode are never equal.
func SameContent(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

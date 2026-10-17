// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"regexp"
	"slices"
)

// Capability is a capability the owner allows in agent_operator_config.v1 (09 section 2.2).
type Capability string

// The capabilities of 09 section 2.2, each the key of its permission.
const (
	CapabilityFollowups  Capability = "supports_followups"
	CapabilityComponents Capability = "supports_components"
	CapabilityVoice      Capability = "supports_voice"
	CapabilityTitle      Capability = "supports_title"
	CapabilityFilesOut   Capability = "supports_files_out"
)

// keyDefaultModel is the only text key of the schema.
const keyDefaultModel = "default_model"

// modelPattern is the pattern of default_model (09 section 2.2).
var modelPattern = regexp.MustCompile(`^[A-Za-z0-9._:/@-]+$`)

// pointerConfig is the JSON pointer of operator_config in AgentCreate and AgentUpdate.
const pointerConfig = "/operator_config"

// operatorConfigV1 is the stored form of agent_operator_config.v1 in the key order of 09 section
// 2.2; absent keys are left out.
type operatorConfigV1 struct {
	DefaultModel       *string `json:"default_model,omitempty"`
	SupportsFollowups  *bool   `json:"supports_followups,omitempty"`
	SupportsComponents *bool   `json:"supports_components,omitempty"`
	SupportsVoice      *bool   `json:"supports_voice,omitempty"`
	SupportsTitle      *bool   `json:"supports_title,omitempty"`
	SupportsFilesOut   *bool   `json:"supports_files_out,omitempty"`
}

// flag returns the field of c.
func (c *operatorConfigV1) flag(k Capability) **bool {
	switch k {
	case CapabilityFollowups:
		return &c.SupportsFollowups
	case CapabilityComponents:
		return &c.SupportsComponents
	case CapabilityVoice:
		return &c.SupportsVoice
	case CapabilityTitle:
		return &c.SupportsTitle
	case CapabilityFilesOut:
		return &c.SupportsFilesOut
	}
	return nil
}

// OperatorConfig is a validated agents.operator_config (schema agent_operator_config.v1): the
// default model the adapter uses without a choice in the session and the capabilities the owner
// allows. It never holds secrets: the schema has no key that could carry one. The zero value is
// the empty object, which allows every capability the adapter reports.
type OperatorConfig struct {
	c operatorConfigV1
}

// ParseOperatorConfig validates m against agent_operator_config.v1: an object without other keys
// (additionalProperties false), every key optional, default_model a text matching
// ^[A-Za-z0-9._:/@-]+$, the supports_* keys booleans. nil is the empty object.
//
// Errors: validation_failed with the pointer /operator_config/<key> of the first invalid key in
// key order; the error never repeats a value.
func ParseOperatorConfig(m map[string]any) (OperatorConfig, error) {
	var out OperatorConfig
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := out.set(k, m[k]); err != nil {
			return OperatorConfig{}, err
		}
	}
	return out, nil
}

// set validates and stores one key.
func (o *OperatorConfig) set(key string, v any) error {
	pointer := pointerConfig + "/" + key
	if key == keyDefaultModel {
		s, ok := v.(string)
		if !ok || !modelPattern.MatchString(s) {
			return fieldError(pointer, FieldPattern, "default_model is not a model name")
		}
		o.c.DefaultModel = &s
		return nil
	}
	field := o.c.flag(Capability(key))
	if field == nil {
		return fieldError(pointer, FieldUnknown, "agent_operator_config.v1 has no such key")
	}
	b, ok := v.(bool)
	if !ok {
		return fieldError(pointer, FieldInvalid, "the value must be a boolean")
	}
	*field = &b
	return nil
}

// OperatorConfigFromStore reads the stored column. The value was validated before it was
// written; a value that is not the schema anymore is refused all the same.
//
// Errors: those of ParseOperatorConfig, or validation_failed on /operator_config for a value that
// is not a JSON object.
func OperatorConfigFromStore(raw []byte) (OperatorConfig, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return OperatorConfig{}, fieldError(pointerConfig, FieldInvalid, "operator_config is not a JSON object")
	}
	return ParseOperatorConfig(m)
}

// JSON returns the canonical JSON object: the keys of 09 section 2.2 in that order, absent keys
// left out, "{}" for the empty configuration.
func (o OperatorConfig) JSON() []byte {
	b, err := json.Marshal(o.c)
	if err != nil {
		// Marshalling a struct of strings and booleans cannot fail.
		return []byte(`{}`)
	}
	return b
}

// Map returns the configuration as a JSON object value (schema JsonObject of the API).
func (o OperatorConfig) Map() map[string]any {
	m := map[string]any{}
	if o.c.DefaultModel != nil {
		m[keyDefaultModel] = *o.c.DefaultModel
	}
	for _, k := range []Capability{CapabilityFollowups, CapabilityComponents, CapabilityVoice, CapabilityTitle, CapabilityFilesOut} {
		if f := *o.c.flag(k); f != nil {
			m[string(k)] = *f
		}
	}
	return m
}

// Allows reports whether the owner allows capability k: a missing key allows (09 section 2.2).
func (o OperatorConfig) Allows(k Capability) bool {
	c := o.c
	f := c.flag(k)
	return f == nil || *f == nil || **f
}

// Equal reports whether o and other hold the same keys and values.
func (o OperatorConfig) Equal(other OperatorConfig) bool {
	return string(o.JSON()) == string(other.JSON())
}

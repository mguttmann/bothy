// SPDX-License-Identifier: Apache-2.0

package specschema

import (
	"fmt"
	"strings"
)

// kwAllOf is the composition keyword the strictness rule treats specially.
const kwAllOf = "allOf"

// Keywords whose value is one subschema, a map of subschemas or a list of subschemas
// (JSON Schema 2020-12 core and applicator vocabularies). Every other keyword holds data or an
// annotation and is copied as it is.
var (
	schemaKeywords = map[string]bool{
		"items": true, "additionalProperties": true, "unevaluatedProperties": true, "unevaluatedItems": true,
		"not": true, "if": true, "then": true, "else": true, "contains": true, "propertyNames": true,
	}
	schemaMapKeywords  = map[string]bool{"properties": true, "patternProperties": true, "$defs": true, "dependentSchemas": true}
	schemaListKeywords = map[string]bool{kwAllOf: true, "anyOf": true, "oneOf": true, "prefixItems": true}
)

// dropped reports whether k is a keyword outside JSON Schema 2020-12 that the bundle removes
// (design note 4): OpenAPI's discriminator, xml and externalDocs and every extension x-*.
func dropped(k string) bool {
	return k == "discriminator" || k == "xml" || k == "externalDocs" || strings.HasPrefix(k, "x-")
}

// visitor is called for every schema object; inAllOf says whether it is a direct allOf member.
type visitor func(m map[string]any, inAllOf bool) error

// walk calls f for every schema object below and including s, keyword-aware.
func walk(s any, inAllOf bool, f visitor) error {
	m, ok := s.(map[string]any)
	if !ok {
		return nil
	}
	if err := f(m, inAllOf); err != nil {
		return err
	}
	for k, v := range m {
		if err := walkKeyword(k, v, f); err != nil {
			return err
		}
	}
	return nil
}

// walkKeyword walks the subschemas the keyword k holds in v.
func walkKeyword(k string, v any, f visitor) error {
	switch {
	case schemaKeywords[k]:
		return walk(v, false, f)
	case schemaMapKeywords[k]:
		sub, _ := v.(map[string]any)
		for _, e := range sub {
			if err := walk(e, false, f); err != nil {
				return err
			}
		}
	case schemaListKeywords[k]:
		list, _ := v.([]any)
		for _, e := range list {
			if err := walk(e, k == kwAllOf, f); err != nil {
				return err
			}
		}
	}
	return nil
}

// rewriteRefs turns the two known $ref forms into #/$defs/X (design note 3) and fails on any
// other form or on a reference to a schema that does not exist.
func rewriteRefs(defs map[string]any) error {
	for name, s := range defs {
		if err := walk(s, false, func(m map[string]any, _ bool) error { return rewriteRef(name, defs, m) }); err != nil {
			return err
		}
	}
	return nil
}

// rewriteRef rewrites the $ref of the schema object m inside the definition name.
func rewriteRef(name string, defs, m map[string]any) error {
	raw, present := m["$ref"]
	if !present {
		return nil
	}
	ref, ok := raw.(string)
	if !ok {
		return fmt.Errorf("specschema: %s: $ref is not a string", name)
	}
	target, ok := strings.CutPrefix(ref, openAPIRefPrefix)
	if !ok {
		target, ok = strings.CutPrefix(ref, asyncAPIRefPrefix)
	}
	if !ok || target == "" || strings.Contains(target, "/") {
		return fmt.Errorf("specschema: %s: unknown $ref form %q", name, ref)
	}
	if _, exists := defs[target]; !exists {
		return fmt.Errorf("specschema: %s: $ref %q names no schema", name, ref)
	}
	m["$ref"] = defsRefPrefix + target
	return nil
}

// allOfRefTargets returns the definitions that some allOf array references directly (base schemas
// such as EventEnvelope); the strictness rule skips them (design note 5 (b)).
func allOfRefTargets(defs map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, s := range defs {
		_ = walk(s, false, func(m map[string]any, inAllOf bool) error {
			if ref, ok := m["$ref"].(string); ok && inAllOf {
				out[strings.TrimPrefix(ref, defsRefPrefix)] = true
			}
			return nil
		})
	}
	return out
}

// normalize returns a copy of the schema s without the dropped keywords and with
// unevaluatedProperties: false on every object schema that declares properties or allOf and
// neither additionalProperties nor unevaluatedProperties (design note 5), except direct inline
// members of an allOf (inAllOf) and, for a definition root, a base schema (strict false). In
// 2020-12, unevaluatedProperties on the composing schema sees the properties of every allOf member,
// while the same keyword on a member would reject the fields of its siblings.
func normalize(s any, strict, inAllOf bool) any {
	m, ok := s.(map[string]any)
	if !ok {
		return s
	}
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		if !dropped(k) {
			out[k] = normalizeKeyword(k, v)
		}
	}
	if strict && !inAllOf && needsStrictness(m) {
		out["unevaluatedProperties"] = false
	}
	return out
}

// normalizeKeyword normalizes the subschemas the keyword k holds in v and copies other values.
func normalizeKeyword(k string, v any) any {
	switch {
	case schemaKeywords[k]:
		return normalize(v, true, false)
	case schemaMapKeywords[k]:
		sub, ok := v.(map[string]any)
		if !ok {
			return v
		}
		cp := make(map[string]any, len(sub))
		for name, e := range sub {
			cp[name] = normalize(e, true, false)
		}
		return cp
	case schemaListKeywords[k]:
		list, ok := v.([]any)
		if !ok {
			return v
		}
		cp := make([]any, len(list))
		for i, e := range list {
			cp[i] = normalize(e, true, k == kwAllOf)
		}
		return cp
	default:
		return v
	}
}

// needsStrictness reports whether m declares properties or allOf and leaves unknown members open.
func needsStrictness(m map[string]any) bool {
	_, hasProps := m["properties"]
	_, hasAllOf := m[kwAllOf]
	_, hasAP := m["additionalProperties"]
	_, hasUP := m["unevaluatedProperties"]
	return (hasProps || hasAllOf) && !hasAP && !hasUP
}

// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// Placeholder replaces a requested ID in the instance member of a problem body and in the raw
// body whose length is compared. It is as long as a UUID, so lengths stay comparable.
const Placeholder = "<requested-id-xxxxxxxxxxxxxxxxxxxxx>"

// droppedHeaders differ per request by design (AZ-P4 design note 4).
var droppedHeaders = []string{"Correlation-Id", "Date", "Dpop-Nonce"}

// Normalized is a response with everything removed that differs per request by design: what
// remains must be equal for a foreign and an unknown ID.
type Normalized struct {
	// Status is the HTTP status.
	Status int
	// Header are the response headers without Correlation-Id, Date and DPoP-Nonce, RateLimit
	// reduced to its policy name.
	Header http.Header
	// Body is the decoded JSON body without correlation_id and with the requested IDs in instance
	// replaced, or the raw body as a string when it is no JSON.
	Body any
	// Length is the byte length of the raw body after the requested IDs were replaced.
	Length int
}

// Normalize returns r normalized for the requested IDs (AZ-P4 design note 4): drop
// Correlation-Id, Date and DPoP-Nonce; keep the policy name of RateLimit and drop r and t; in a
// JSON object body drop correlation_id and replace the requested IDs in instance with
// Placeholder; Length is the raw byte length after the same replacement.
func Normalize(r harness.Response, requested []string) Normalized {
	h := r.Header.Clone()
	if h == nil {
		h = http.Header{}
	}
	for _, name := range droppedHeaders {
		h.Del(name)
	}
	if v := h.Values("Ratelimit"); len(v) > 0 {
		policies := make([]string, len(v))
		for i, x := range v {
			policies[i] = ratePolicy(x)
		}
		h["Ratelimit"] = policies
	}
	raw := r.Body
	for _, id := range requested {
		if id != "" {
			raw = bytes.ReplaceAll(raw, []byte(id), []byte(Placeholder))
		}
	}
	n := Normalized{Status: r.Status, Header: h, Length: len(raw)}
	var body any
	if err := json.Unmarshal(r.Body, &body); err != nil {
		n.Body = string(r.Body)
		return n
	}
	if obj, ok := body.(map[string]any); ok {
		delete(obj, "correlation_id")
		if inst, ok := obj["instance"].(string); ok {
			for _, id := range requested {
				if id != "" {
					inst = strings.ReplaceAll(inst, id, Placeholder)
				}
			}
			obj["instance"] = inst
		}
	}
	n.Body = body
	return n
}

// ratePolicy keeps the policy name of one RateLimit field value (draft-ietf-httpapi-ratelimit
// headers): the item before the first parameter, `"rl.read";r=9;t=1` becomes `"rl.read"`.
func ratePolicy(v string) string {
	parts := strings.Split(v, ",")
	for i, p := range parts {
		name, _, _ := strings.Cut(p, ";")
		parts[i] = strings.TrimSpace(name)
	}
	return strings.Join(parts, ", ")
}

// Diff returns a description of every difference between a and b, empty when they are equal.
func Diff(a, b Normalized) []string {
	var out []string
	if a.Status != b.Status {
		out = append(out, fmt.Sprintf("status %d != %d", a.Status, b.Status))
	}
	names := map[string]bool{}
	for k := range a.Header {
		names[k] = true
	}
	for k := range b.Header {
		names[k] = true
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Equal(a.Header.Values(k), b.Header.Values(k)) {
			out = append(out, fmt.Sprintf("header %s %q != %q", k, a.Header.Values(k), b.Header.Values(k)))
		}
	}
	if !reflect.DeepEqual(a.Body, b.Body) {
		out = append(out, fmt.Sprintf("body %v != %v", a.Body, b.Body))
	}
	if a.Length != b.Length {
		out = append(out, fmt.Sprintf("length %d != %d", a.Length, b.Length))
	}
	return out
}

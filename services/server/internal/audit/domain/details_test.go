// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
)

const (
	uuidA = "01928f3a-7b1c-7000-8a00-0000000000a1"
	uuidB = "01928f3a-7b1c-7000-8a00-0000000000b1"
	timeA = "2026-10-12T08:15:30.120Z"
	timeB = "2026-10-12T08:16:10.005Z"
)

// validDetails returns a details object with every reserved member and every attribute kind.
func validDetails() domain.DetailsV1 {
	return domain.DetailsV1{
		"ip_hmac":   strings.Repeat("ab", 32),
		"ip_key_id": uuidB,
		"aggregate": map[string]any{"count": int64(2), "first_at": timeA, "last_at": timeB},
		"violations": []any{map[string]any{
			"code": "i1_violation", "agent_id": uuidA, "credential_holder_id": uuidB,
			"principal_ids": []any{uuidA, uuidB},
		}},
		"reason":   "Révision by Alex",
		"os_user":  "BOTHY-S-SHARED",
		"level":    "read",
		"grant_id": uuidA,
		"expires":  timeA,
		"count":    json.Number("12"),
		"step":     -3,
		"inherit":  true,
		"targets":  []any{uuidA, uuidB},
		"empty":    []any{},
	}
}

func TestDetailsV1_ClosedSchema(t *testing.T) {
	t.Parallel()
	if err := validDetails().Validate(); err != nil {
		t.Fatalf("valid details rejected: %v", err)
	}
	if err := (domain.DetailsV1{}).Validate(); err != nil {
		t.Fatalf("empty details rejected: %v", err)
	}
	cases := map[string]func(d domain.DetailsV1){
		"ip_hmac short":             func(d domain.DetailsV1) { d["ip_hmac"] = "ab" },
		"ip_hmac upper case":        func(d domain.DetailsV1) { d["ip_hmac"] = strings.Repeat("AB", 32) },
		"ip_hmac a number":          func(d domain.DetailsV1) { d["ip_hmac"] = int64(1) },
		"ip_key_id not a UUID":      func(d domain.DetailsV1) { d["ip_key_id"] = "key-1" },
		"ip_hmac without ip_key_id": func(d domain.DetailsV1) { delete(d, "ip_key_id") },
		"aggregate count 0": func(d domain.DetailsV1) {
			d["aggregate"] = map[string]any{"count": int64(0), "first_at": timeA, "last_at": timeA}
		},
		"aggregate extra member":    func(d domain.DetailsV1) { d["aggregate"].(map[string]any)["x"] = int64(1) },
		"aggregate missing last_at": func(d domain.DetailsV1) { delete(d["aggregate"].(map[string]any), "last_at") },
		"aggregate last before first": func(d domain.DetailsV1) {
			d["aggregate"] = map[string]any{"count": int64(2), "first_at": timeB, "last_at": timeA}
		},
		"aggregate a string":       func(d domain.DetailsV1) { d["aggregate"] = "3" },
		"violations an object":     func(d domain.DetailsV1) { d["violations"] = map[string]any{} },
		"violation missing agent":  func(d domain.DetailsV1) { delete(d["violations"].([]any)[0].(map[string]any), "agent_id") },
		"violation free text code": func(d domain.DetailsV1) { d["violations"].([]any)[0].(map[string]any)["code"] = "two words" },
		"violation extra member":   func(d domain.DetailsV1) { d["violations"].([]any)[0].(map[string]any)["name"] = "x" },
		"reason empty":             func(d domain.DetailsV1) { d["reason"] = "" },
		"reason with control":      func(d domain.DetailsV1) { d["reason"] = "a\nb" },
		"reason with bidi":         func(d domain.DetailsV1) { d["reason"] = "a\u202eb" },
		"reason a number":          func(d domain.DetailsV1) { d["reason"] = int64(1) },
		"os_user a path":           func(d domain.DetailsV1) { d["os_user"] = "../x" },
		"os_user empty":            func(d domain.DetailsV1) { d["os_user"] = "" },
		"os_user a number":         func(d domain.DetailsV1) { d["os_user"] = int64(1) },
		"free text attribute":      func(d domain.DetailsV1) { d["note"] = "hello world" },
		"e-mail attribute":         func(d domain.DetailsV1) { d["mail"] = "sam@example.com" },
		"float attribute":          func(d domain.DetailsV1) { d["ratio"] = 0.5 },
		"float number attribute":   func(d domain.DetailsV1) { d["ratio"] = json.Number("0.5") },
		"null attribute":           func(d domain.DetailsV1) { d["x"] = nil },
		"object attribute":         func(d domain.DetailsV1) { d["x"] = map[string]any{} },
		"nested list":              func(d domain.DetailsV1) { d["x"] = []any{[]any{}} },
		"bad key upper case":       func(d domain.DetailsV1) { d["Level"] = "read" },
		"bad key leading digit":    func(d domain.DetailsV1) { d["1level"] = "read" },
		"bad key too long":         func(d domain.DetailsV1) { d["k"+strings.Repeat("x", domain.MaxCodeLen)] = "read" },
		"bad code":                 func(d domain.DetailsV1) { d["level"] = "Read" },
		"code too long":            func(d domain.DetailsV1) { d["level"] = strings.Repeat("x", domain.MaxCodeLen+1) },
		"integer too large":        func(d domain.DetailsV1) { d["n"] = int64(1) << 53 },
		"NUL in code":              func(d domain.DetailsV1) { d["level"] = "a\x00" },
		"IPv4 address as code":     func(d domain.DetailsV1) { d["peer"] = "192.0.2.44" },
		"IPv6 address as code":     func(d domain.DetailsV1) { d["peer"] = "2001:db8::44" },
		"IPv4 with port as code":   func(d domain.DetailsV1) { d["peer"] = "192.0.2.44:8080" },
		"IPv6 with port as code":   func(d domain.DetailsV1) { d["peer"] = "[2001:db8::44]:8080" },
		"IP address in a list":     func(d domain.DetailsV1) { d["peers"] = []any{"192.0.2.44"} },
	}
	for name, mutate := range cases {
		d := validDetails()
		mutate(d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestValidCode_LengthBound(t *testing.T) {
	t.Parallel()
	if domain.MaxCodeLen != 64 {
		t.Fatalf("MaxCodeLen = %d; changing it widens what a code may carry (SEC-AUD-5)", domain.MaxCodeLen)
	}
	if !domain.ValidCode(strings.Repeat("a", domain.MaxCodeLen)) || domain.ValidCode(strings.Repeat("a", domain.MaxCodeLen+1)) {
		t.Fatal("the length bound of codes is not MaxCodeLen")
	}
	for _, ok := range []string{"no_grant", "grant.create", "a:b-c", "0", uuidA} {
		if !domain.ValidCode(ok) {
			t.Errorf("ValidCode(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "_x", "A", "a b", "é", "a/b"} {
		if domain.ValidCode(bad) {
			t.Errorf("ValidCode(%q) = true", bad)
		}
	}
}

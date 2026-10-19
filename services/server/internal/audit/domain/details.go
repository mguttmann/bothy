// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The reserved members of audit_details.v1 (19 SEC-AUD-5).
const (
	// DetailIPHMAC is the IP pseudonym, 64 lower-case hex characters.
	DetailIPHMAC = "ip_hmac"
	// DetailIPKeyID is the audit_ip_keys.id of the key behind ip_hmac.
	DetailIPKeyID = "ip_key_id"
	// DetailAggregate is {count, first_at, last_at} of an aggregated denial.
	DetailAggregate = "aggregate"
	// DetailViolations is the list {code, agent_id or host_id, credential_holder_id, principal_ids} of
	// i1_violation and i6_violation (M2-40).
	DetailViolations = "violations"
	// DetailReason is the reason a management action requires, the only free text.
	DetailReason = "reason"
	// DetailOSUser is the name of a session user of a terminal host (M2-39), in the form of
	// host_session_users.os_user; never a path.
	DetailOSUser = "os_user"
)

// MaxCodeLen is the longest code (and attribute key) in bytes. Codes are short machine words
// such as reason codes and action ids; the bound keeps a code from carrying text.
const MaxCodeLen = 64

// ErrDetails is wrapped by every error of DetailsV1.Validate.
var ErrDetails = errors.New("audit: details outside audit_details.v1")

var (
	codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)
	keyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	hmacPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// osUserPattern is the check host_session_users_os_user_check of 03 section 4.14.
	osUserPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
)

// ValidCode reports whether s is a code of SEC-AUD-5: ^[a-z0-9][a-z0-9_.:-]*$ and at most
// MaxCodeLen bytes. UUIDs in canonical form are codes too.
func ValidCode(s string) bool { return len(s) <= MaxCodeLen && codePattern.MatchString(s) }

// ValidKey reports whether s is an attribute key: ^[a-z][a-z0-9_]*$ and at most MaxCodeLen bytes.
func ValidKey(s string) bool { return len(s) <= MaxCodeLen && keyPattern.MatchString(s) }

// IsReserved reports whether key is one of the reserved members of audit_details.v1.
func IsReserved(key string) bool {
	switch key {
	case DetailIPHMAC, DetailIPKeyID, DetailAggregate, DetailViolations, DetailReason, DetailOSUser:
		return true
	}
	return false
}

// ValidReasonText checks the free-text reason of a management action: not empty and free of
// invalid UTF-8, control and bidi characters (SEC-IN-4 through kernel/textcheck). Its length
// bound (audit.reason_max_chars) belongs to the request validation of the action, which answers
// 400 too_long.
func ValidReasonText(s string) error {
	if s == "" {
		return fmt.Errorf("%w: reason is empty", ErrDetails)
	}
	if err := textcheck.DisplayText(s); err != nil {
		return fmt.Errorf("%w: reason: %w", ErrDetails, err)
	}
	return nil
}

// ValidOSUser reports whether s is the name of a session user in the form of
// host_session_users.os_user: ^[A-Za-z0-9_][A-Za-z0-9_.-]*$, never a path.
func ValidOSUser(s string) bool { return osUserPattern.MatchString(s) }

// DetailsV1 is a details object of schema audit_details.v1 (19 SEC-AUD-5), decoded: strings,
// booleans, integers (int, int64, json.Number), []any and, inside the reserved members,
// map[string]any.
type DetailsV1 map[string]any

// Validate checks d against audit_details.v1: the reserved members in their shapes (ip_hmac and
// ip_key_id only together), every other member under a key matching ^[a-z][a-z0-9_]*$ with a
// value that is a UUID, a code (never an IP address), an integer within 2^53 - 1, a boolean, a canonical time or a
// list of these. Free text outside reason, floats, null, objects and nested lists are refused.
// The error wraps ErrDetails and names the member, never its value.
func (d DetailsV1) Validate() error {
	_, hasHMAC := d[DetailIPHMAC]
	_, hasKey := d[DetailIPKeyID]
	if hasHMAC != hasKey {
		return fmt.Errorf("%w: ip_hmac and ip_key_id come together", ErrDetails)
	}
	for k, v := range d {
		if err := validateMember(k, v); err != nil {
			return err
		}
	}
	return nil
}

// validateMember checks one member of a details object.
func validateMember(k string, v any) error {
	var ok bool
	switch k {
	case DetailIPHMAC:
		s, isStr := v.(string)
		ok = isStr && hmacPattern.MatchString(s)
	case DetailIPKeyID:
		ok = isUUID(v)
	case DetailAggregate:
		ok = validAggregate(v)
	case DetailViolations:
		ok = validViolations(v)
	case DetailReason:
		s, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("%w: member reason is not a string", ErrDetails)
		}
		return ValidReasonText(s)
	case DetailOSUser:
		s, isStr := v.(string)
		ok = isStr && ValidOSUser(s)
	default:
		if !ValidKey(k) {
			return fmt.Errorf("%w: attribute key does not match ^[a-z][a-z0-9_]*$", ErrDetails)
		}
		ok = validAttribute(v)
	}
	if !ok {
		return fmt.Errorf("%w: member %s has a value outside its form", ErrDetails, k)
	}
	return nil
}

// validAttribute reports a scalar attribute value or a flat list of them.
func validAttribute(v any) bool {
	if list, isList := v.([]any); isList {
		for _, e := range list {
			if !validScalar(e) {
				return false
			}
		}
		return true
	}
	return validScalar(v)
}

// validScalar reports a UUID, code, canonical time, safe integer or boolean.
func validScalar(v any) bool {
	switch x := v.(type) {
	case string:
		return (ValidCode(x) && !isIPAddress(x)) || isTime(x)
	case bool:
		return true
	case int, int64, json.Number:
		_, ok := integer(x)
		return ok
	default:
		return false
	}
}

// integer returns v as int64 when it is an integer within 2^53 - 1.
func integer(v any) (int64, bool) {
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	case json.Number:
		var err error
		if n, err = parseInteger(string(x)); err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, n <= MaxSafeInteger && n >= -MaxSafeInteger
}

// isIPAddress reports a string that parses as an IP address, with or without a port: the code
// pattern admits "192.0.2.44", "192.0.2.44:8080" and "2001:db8::44", but an address may reach the
// log only as ip_hmac (20 "4.5").
func isIPAddress(s string) bool {
	if _, err := netip.ParseAddr(s); err == nil {
		return true
	}
	_, err := netip.ParseAddrPort(s)
	return err == nil
}

// isUUID reports a UUID string in canonical lower-case form.
func isUUID(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	_, err := ParseUUID(s)
	return err == nil
}

// isTime reports a time string in the canonical form of FormatTime.
func isTime(s string) bool {
	_, err := ParseTime(s)
	return err == nil
}

// timeOf returns the canonical time v, or false.
func timeOf(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := ParseTime(s)
	return t, err == nil
}

// exactKeys reports whether m has exactly the keys want.
func exactKeys(m map[string]any, want ...string) bool {
	if len(m) != len(want) {
		return false
	}
	for k := range m {
		if !slices.Contains(want, k) {
			return false
		}
	}
	return true
}

// validAggregate checks {count >= 1, first_at, last_at >= first_at}.
func validAggregate(v any) bool {
	m, ok := v.(map[string]any)
	if !ok || !exactKeys(m, "count", "first_at", "last_at") {
		return false
	}
	n, okN := integer(m["count"])
	first, okF := timeOf(m["first_at"])
	last, okL := timeOf(m["last_at"])
	return okN && okF && okL && n >= 1 && !last.Before(first)
}

// validViolations checks a list of {code, agent_id, credential_holder_id, principal_ids} (I1)
// or {code, host_id, credential_holder_id, principal_ids} (I6, M2-40).
func validViolations(v any) bool {
	list, ok := v.([]any)
	if !ok {
		return false
	}
	for _, e := range list {
		if m, isMap := e.(map[string]any); !isMap || !validViolation(m) {
			return false
		}
	}
	return true
}

// validViolation checks one entry of validViolations: the subject is host_id for I6, else
// agent_id.
func validViolation(m map[string]any) bool {
	subject := "agent_id"
	if _, host := m["host_id"]; host {
		subject = "host_id"
	}
	if !exactKeys(m, "code", subject, "credential_holder_id", "principal_ids") {
		return false
	}
	code, isStr := m["code"].(string)
	ids, isList := m["principal_ids"].([]any)
	if !isStr || !ValidCode(code) || !isUUID(m[subject]) || !isUUID(m["credential_holder_id"]) || !isList {
		return false
	}
	for _, id := range ids {
		if !isUUID(id) {
			return false
		}
	}
	return true
}

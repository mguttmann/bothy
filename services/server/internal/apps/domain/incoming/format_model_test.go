// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// foreignMembers are top-level names neither format defines: the members Alertmanager v0.34.1
// sends beyond the documented payload (routeLabels, notification_reason), a typo and names a
// later sender version might add, one with the characters a JSON Pointer escapes.
var foreignMembers = []string{"routeLabels", "notification_reason", "colour", "futureMember", "a/b~c"}

// classifyModel is the rule of 11 section 8.2 as a reference: a body with an Alertmanager member
// is alertmanager and ignores foreign members, a native member there is format_mixed; every other
// body is native and refuses every foreign member with unknown_member, sorted by name.
func classifyModel(present []string) (incoming.Format, []errs.FieldError) {
	if slices.ContainsFunc(present, func(m string) bool { return slices.Contains(incoming.AlertmanagerMembers(), m) }) {
		for _, m := range incoming.NativeMembers() {
			if slices.Contains(present, m) {
				return "", []errs.FieldError{{Pointer: "/" + m, Code: incoming.CodeFormatMixed}}
			}
		}
		return incoming.FormatAlertmanager, nil
	}
	var unknown []string
	for _, m := range present {
		if !slices.Contains(incoming.NativeMembers(), m) {
			unknown = append(unknown, m)
		}
	}
	if len(unknown) == 0 {
		return incoming.FormatNative, nil
	}
	slices.Sort(unknown)
	out := make([]errs.FieldError, 0, len(unknown))
	for _, m := range unknown {
		out = append(out, errs.FieldError{Pointer: "/" + pointerEscape(m), Code: incoming.CodeUnknownMember})
	}
	return "", out
}

// pointerEscape escapes a member name for a JSON Pointer (RFC 6901 section 3).
func pointerEscape(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// checkClassify compares Classify with the model for one set of present members.
func checkClassify(t *testing.T, present []string) {
	t.Helper()
	wantFormat, wantFields := classifyModel(present)
	got, err := incoming.Classify(present)
	if wantFields == nil {
		if err != nil || got != wantFormat {
			t.Fatalf("Classify(%q) = %q, %v; want %q", present, got, err, wantFormat)
		}
		return
	}
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("Classify(%q) = %q, %v; want validation_failed %+v", present, got, err, wantFields)
	}
	if len(e.Fields) != len(wantFields) {
		t.Fatalf("Classify(%q) fields = %+v, want %+v", present, e.Fields, wantFields)
	}
	for i, f := range e.Fields {
		if f.Pointer != wantFields[i].Pointer || f.Code != wantFields[i].Code {
			t.Fatalf("Classify(%q) fields = %+v, want %+v", present, e.Fields, wantFields)
		}
	}
}

// TestClassify_ForeignMembersModel covers the whole class of member sets: every subset of the
// native members, the Alertmanager members and the foreign names, in random order (seeded),
// against the model. A foreign sender's payload with a member the contract does not name must
// not be refused, because Alertmanager drops a notification answered with 400.
func TestClassify_ForeignMembersModel(t *testing.T) {
	t.Parallel()
	// The known repros first: the real v0.34.1 payload and the native typo.
	checkClassify(t, append(incoming.AlertmanagerMembers(), "routeLabels", "notification_reason"))
	checkClassify(t, []string{"content_md", "colour"})
	all := slices.Concat(incoming.NativeMembers(), incoming.AlertmanagerMembers(), foreignMembers)
	r := rand.New(rand.NewPCG(20, 10)) //nolint:gosec // G404: reproducible test data from a fixed seed, not security
	for range 5000 {
		var present []string
		for _, m := range all {
			if r.IntN(3) == 0 {
				present = append(present, m)
			}
		}
		r.Shuffle(len(present), func(i, j int) { present[i], present[j] = present[j], present[i] })
		checkClassify(t, present)
	}
}

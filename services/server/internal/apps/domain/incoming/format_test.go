// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// wantField asserts that err is a validation_failed with exactly one field error at pointer and code.
func wantField(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("error = %v, want validation_failed", err)
	}
	if len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("fields = %+v, want one at %q with %q", e.Fields, pointer, code)
	}
}

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name    string
		present []string
		want    incoming.Format
	}{
		{"native", []string{"content_md", "title", "thread_id", "client_msg_id"}, incoming.FormatNative},
		{"content only", []string{"content_md"}, incoming.FormatNative},
		{"alertmanager", incoming.AlertmanagerMembers(), incoming.FormatAlertmanager},
		{"one alertmanager member", []string{"alerts"}, incoming.FormatAlertmanager},
		{"empty object is native", nil, incoming.FormatNative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := incoming.Classify(tc.present)
			if err != nil || got != tc.want {
				t.Fatalf("Classify = %q, %v; want %q", got, err, tc.want)
			}
		})
	}

	t.Run("mixed body names the first native member", func(t *testing.T) {
		_, err := incoming.Classify([]string{"title", "client_msg_id", "version", "alerts"})
		wantField(t, err, "/title", incoming.CodeFormatMixed)
		_, err = incoming.Classify([]string{"thread_id", "status"})
		wantField(t, err, "/thread_id", incoming.CodeFormatMixed)
	})

	t.Run("member lists in schema order", func(t *testing.T) {
		if got := incoming.NativeMembers(); len(got) != 4 || got[0] != "content_md" || got[3] != "client_msg_id" {
			t.Fatalf("NativeMembers = %v", got)
		}
		if got := incoming.AlertmanagerMembers(); len(got) != 10 || got[0] != "version" || got[9] != "alerts" {
			t.Fatalf("AlertmanagerMembers = %v", got)
		}
	})
}

func TestCheckEnabled(t *testing.T) {
	if err := incoming.CheckEnabled(incoming.FormatNative, []string{"native", "alertmanager"}); err != nil {
		t.Fatalf("enabled native: %v", err)
	}
	wantField(t, incoming.CheckEnabled(incoming.FormatAlertmanager, []string{"native"}), "", incoming.CodeFormatDisabled)
	wantField(t, incoming.CheckEnabled(incoming.FormatNative, nil), "", incoming.CodeFormatDisabled)
}

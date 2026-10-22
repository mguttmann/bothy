// SPDX-License-Identifier: Apache-2.0

package previewtext_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/authz/domain/previewtext"
)

// exampleSwitch is the guided flow of 10 section 11 (M2-21 design note 9): the owner previews Sam
// with trigger on Research/Notizen and resolve_i1 for Assistent, whose adapter reports
// organization, and Helfer, whose adapter does not.
func exampleSwitch(locale string) previewtext.Input {
	return previewtext.Input{
		Locale: locale, TimeZone: "UTC",
		Subject:       previewtext.Subject{Kind: domain.SubjectUser, Name: "Sam"},
		Level:         domain.LevelTrigger,
		Target:        previewtext.Target{Kind: domain.TargetChannel, Space: "Research", Channel: "Notizen"},
		TriggerAgents: []string{"Assistent"},
		Switches:      []previewtext.Switch{{Agent: "Helfer"}, {Agent: "Assistent", OrganizationReady: true}},
		ExpiryMissing: true,
	}
}

// TestPreviewTextSwitchLines: one switch line per switched agent in name order before the
// summary, and the not-ready line for an adapter without organization, in German and English.
func TestPreviewTextSwitchLines(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"de", "en"} {
		b, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", "example_switch_"+lang+".golden")))
		if err != nil {
			t.Fatal(err)
		}
		text, got := previewtext.Render(exampleSwitch(lang))
		if want := strings.TrimSuffix(string(b), "\n"); text != want || got != lang {
			t.Fatalf("text (%s):\n%s\nwant:\n%s", got, text, want)
		}
	}
}

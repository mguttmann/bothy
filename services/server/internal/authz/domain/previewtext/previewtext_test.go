// SPDX-License-Identifier: Apache-2.0

package previewtext_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/authz/domain/previewtext"
)

// example07 is the world of 07 section 9.3: the owner previews Sam with trigger on
// Marketing/Kampagnen until 2026-12-31T12:00:00Z without capabilities; Texter is the default
// operator of Marketing and also reads Marketing/Intern.
func example07(locale string) previewtext.Input {
	return previewtext.Input{
		Locale: locale, TimeZone: "UTC",
		Subject:       previewtext.Subject{Kind: domain.SubjectUser, Name: "Sam"},
		Level:         domain.LevelTrigger,
		Target:        previewtext.Target{Kind: domain.TargetChannel, Space: "Marketing", Channel: "Kampagnen"},
		ExpiresAt:     time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC),
		TriggerAgents: []string{"Texter"},
		ReadHorizon:   []previewtext.Horizon{{Agent: "Texter", Space: "Marketing", Channel: "Intern"}},
	}
}

// golden returns the golden file name as text without its final line feed.
func golden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", name)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "\n") || strings.Count(string(b), "\n") != 4 {
		t.Fatalf("%s must hold exactly four lines", name)
	}
	return strings.TrimSuffix(string(b), "\n")
}

func TestPreviewTextExample07German(t *testing.T) {
	t.Parallel()
	text, lang := previewtext.Render(example07("de"))
	if want := golden(t, "example_07_de.golden"); text != want || lang != "de" {
		t.Fatalf("text (%s):\n%s\nwant:\n%s", lang, text, want)
	}
}

func TestPreviewTextExample07English(t *testing.T) {
	t.Parallel()
	text, lang := previewtext.Render(example07("en"))
	if want := golden(t, "example_07_en.golden"); text != want || lang != "en" {
		t.Fatalf("text (%s):\n%s\nwant:\n%s", lang, text, want)
	}
}

// TestPreviewTextLanguageFallback: the primary subtag of users.locale decides; anything but de
// and en is German, the product default (01 section 9).
func TestPreviewTextLanguageFallback(t *testing.T) {
	t.Parallel()
	for locale, want := range map[string]string{"fr-FR": "de", "en-GB": "en", "": "de", "EN": "en", "de_AT": "de", "eng": "de"} {
		if _, lang := previewtext.Render(example07(locale)); lang != want {
			t.Errorf("locale %q: language %s, want %s", locale, lang, want)
		}
	}
	text, _ := previewtext.Render(example07("fr-FR"))
	if text != golden(t, "example_07_de.golden") {
		t.Fatalf("fr-FR renders German:\n%s", text)
	}
}

// TestPreviewTextDateInTimeZone: the instant is shown in the caller's time zone; an invalid zone
// falls back to UTC.
func TestPreviewTextDateInTimeZone(t *testing.T) {
	t.Parallel()
	in := example07("en")
	in.ExpiresAt = time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC)
	in.TimeZone = "Europe/Berlin"
	if text, _ := previewtext.Render(in); !strings.Contains(text, "until 2027-01-01.") {
		t.Fatalf("Berlin date: %s", text)
	}
	in.TimeZone = "Not/AZone"
	if text, _ := previewtext.Render(in); !strings.Contains(text, "until 2026-12-31.") {
		t.Fatalf("invalid zone renders UTC: %s", text)
	}
}

// TestPreviewTextHintsAndOrder: every hint line in the fixed order denial, I1, tool class, read
// horizon, group reach, missing expiry; group wording with counts; caps lines.
func TestPreviewTextHintsAndOrder(t *testing.T) {
	t.Parallel()
	in := previewtext.Input{
		Locale: "en", Subject: previewtext.Subject{Kind: domain.SubjectGroup, Name: "Leser"}, Level: domain.LevelWrite,
		Target: previewtext.Target{Kind: domain.TargetSpace, Space: "Support"}, Caps: domain.CapsOf(domain.CapUpload, domain.CapExport, domain.CapManage),
		Denial:      domain.ReasonDelegationExceedsOwnLevel,
		I1:          []previewtext.I1{{Agent: "Zed", Holder: "Owner"}, {Agent: "Assistent", Holder: "Owner"}},
		ToolClass:   []string{"Runner"},
		ReadHorizon: []previewtext.Horizon{{Agent: "Lotse", Space: "Support", Channel: "Intern", Count: 1}, {Agent: "Lotse", Space: "Marketing", Channel: "A", Count: 2}},
		GroupReach:  previewtext.Count{Set: true, N: 1},
	}
	text, _ := previewtext.Render(in)
	want := strings.Join([]string{
		`Group Leser gets the level "Write" in space Support.`,
		`Members of Leser can then: read and write messages, create threads.`,
		`Upload files: yes. Confirm approval cards: no. Export: yes. Manage: yes.`,
		`Rejected: you can grant at most your own level.`,
		`Rejected: Assistent runs on the personal subscription of Owner. Switch it to organization first.`,
		`Rejected: Zed runs on the personal subscription of Owner. Switch it to organization first.`,
		`Note: Runner has the tool class host; Group Leser cannot trigger Runner (owners only).`,
		`Note: Lotse also reads Marketing/A. 2 members of Leser may not read Marketing/A but can learn its contents through Lotse.`,
		`Note: Lotse also reads Support/Intern. 1 member of Leser may not read Support/Intern but can learn its contents through Lotse.`,
		`Note: the group Leser reaches one person.`,
	}, "\n")
	if text != want {
		t.Fatalf("text:\n%s\nwant:\n%s", text, want)
	}
	in.Locale, in.GroupReach, in.Denial, in.I1, in.ToolClass, in.ReadHorizon = "de", previewtext.Count{Set: true, N: 3}, "", nil, nil, nil
	in.Subject, in.Level, in.Target, in.Caps = previewtext.Subject{Kind: domain.SubjectUser, Name: "Alex"}, domain.LevelRead,
		previewtext.Target{Kind: domain.TargetInstance}, 0
	in.GroupReach, in.ExpiryMissing = previewtext.Count{}, true
	text, _ = previewtext.Render(in)
	want = strings.Join([]string{
		// language-check: off
		`Alex erhält die Stufe "Lesen" in der ganzen Instanz.`,
		`Alex kann damit: Nachrichten lesen.`,
		`Dateien hochladen: nein. Freigabe-Karten bestätigen: nein.`,
		`Hinweis: Die Freigabe für Alex hat kein Ablaufdatum.`,
		// language-check: on
	}, "\n")
	if text != want {
		t.Fatalf("text:\n%s\nwant:\n%s", text, want)
	}
}

// TestPreviewTextAgentsNeverTrigger: an agent subject never gets the trigger part, and the
// trigger agents are sorted by name.
func TestPreviewTextAgentsNeverTrigger(t *testing.T) {
	t.Parallel()
	in := example07("en")
	in.TriggerAgents = []string{"Zed", "Anna"}
	if text, _ := previewtext.Render(in); !strings.Contains(text, "create threads, trigger agent Anna, trigger agent Zed.") {
		t.Fatalf("sorted agents: %s", text)
	}
	in.Subject = previewtext.Subject{Kind: domain.SubjectAgent, Name: "Lotse"}
	if text, _ := previewtext.Render(in); strings.Contains(text, "trigger agent") {
		t.Fatalf("an agent subject triggers nothing: %s", text)
	}
}

// TestPreviewTextNamesAreNotExpanded: a name holding a placeholder stays as written.
func TestPreviewTextNamesAreNotExpanded(t *testing.T) {
	t.Parallel()
	in := example07("en")
	in.Subject.Name = "Sam {where}"
	if text, _ := previewtext.Render(in); !strings.HasPrefix(text, `Sam {where} gets the level "Trigger" in channel Marketing/Kampagnen`) {
		t.Fatalf("text:\n%s", text)
	}
}

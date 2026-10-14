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

// exampleHost is the world of the golden host texts (Design note 13 of M2-40): the owner previews
// a grant terminal to Sam on R1 (assigned to Alex, organization, not a server host) without expiry.
func exampleHost(locale string) previewtext.Input {
	return previewtext.Input{
		Locale: locale, TimeZone: "UTC",
		Subject:       previewtext.Subject{Kind: domain.SubjectUser, Name: "Sam"},
		Target:        previewtext.Target{Kind: domain.TargetHost, Host: "R1"},
		Caps:          domain.CapsOf(domain.CapTerminal),
		NotEffective:  previewtext.NotEffective{Set: true, Cause: previewtext.CauseNotAssignee},
		ExpiryMissing: true,
	}
}

// goldenHost returns the golden host text, which has exactly three lines.
func goldenHost(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", name)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "\n") || strings.Count(string(b), "\n") != 3 {
		t.Fatalf("%s must hold exactly three lines", name)
	}
	return strings.TrimSuffix(string(b), "\n")
}

func TestPreviewTextExampleHostGerman(t *testing.T) {
	t.Parallel()
	text, lang := previewtext.Render(exampleHost("de"))
	if want := goldenHost(t, "example_host_de.golden"); text != want || lang != "de" {
		t.Fatalf("text (%s):\n%s\nwant:\n%s", lang, text, want)
	}
}

func TestPreviewTextExampleHostEnglish(t *testing.T) {
	t.Parallel()
	text, lang := previewtext.Render(exampleHost("en"))
	if want := goldenHost(t, "example_host_en.golden"); text != want || lang != "en" {
		t.Fatalf("text (%s):\n%s\nwant:\n%s", lang, text, want)
	}
}

// TestPreviewTextHostVariants (Design note 13): every host template in both languages, the order
// of the lines, no abilities or capabilities line, and hint.i6 with the holder's display name.
func TestPreviewTextHostVariants(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name, locale string
		edit         func(*previewtext.Input)
		want         []string
	}{
		{"server host de", "de", func(in *previewtext.Input) {
			in.NotEffective.Cause = previewtext.CauseServerHost
			in.ExpiryMissing = false
		},
			[]string{`Sam erhält das Zusatzrecht "Terminal" auf dem Rechner R1.`,
				"Hinweis: Die Freigabe wirkt für Sam nicht. R1 betreibt den Server; dort öffnen nur Owner ein Terminal."}},
		{"server host en", "en", func(in *previewtext.Input) {
			in.NotEffective.Cause = previewtext.CauseServerHost
			in.ExpiryMissing = false
		},
			[]string{`Sam gets the additional right "Terminal" on host R1.`,
				"Note: the grant does not take effect for Sam. R1 runs the server; only owners open a terminal there."}},
		{"no session user de with expiry", "de", func(in *previewtext.Input) {
			in.NotEffective.Cause, in.ExpiryMissing, in.ExpiresAt = previewtext.CauseNoSessionUser, false, exp
		}, []string{`Sam erhält bis 31.12.2026 das Zusatzrecht "Terminal" auf dem Rechner R1.`,
			"Hinweis: Die Freigabe wirkt für Sam erst, wenn Sam auf R1 einen eigenen Sitzungsbenutzer hat."}},
		{"no session user en with expiry", "en", func(in *previewtext.Input) {
			in.NotEffective.Cause, in.ExpiryMissing, in.ExpiresAt = previewtext.CauseNoSessionUser, false, exp
		}, []string{`Sam gets the additional right "Terminal" on host R1 until 2026-12-31.`,
			"Note: the grant takes effect for Sam only once Sam has an own session user on R1."}},
		{"i6 de", "de", func(in *previewtext.Input) {
			in.Target.Host, in.NotEffective = "R2", previewtext.NotEffective{}
			in.HostI6 = []previewtext.I6{{Host: "R2", Holder: "Robin"}}
		}, []string{`Sam erhält das Zusatzrecht "Terminal" auf dem Rechner R2.`,
			"Abgelehnt: Auf R2 gilt die persönliche LLM-Anmeldung von Robin. Dort darf nur Robin eine Freigabe haben.",
			"Hinweis: Die Freigabe für Sam hat kein Ablaufdatum."}},
		{"i6 en", "en", func(in *previewtext.Input) {
			in.Target.Host, in.NotEffective = "R2", previewtext.NotEffective{}
			in.HostI6 = []previewtext.I6{{Host: "R2", Holder: "Robin"}}
		}, []string{`Sam gets the additional right "Terminal" on host R2.`,
			"Rejected: R2 uses the personal LLM login of Robin. Only Robin may hold a grant there.",
			"Note: the grant for Sam has no expiry date."}},
		{"group de", "de", func(in *previewtext.Input) {
			in.Subject = previewtext.Subject{Kind: domain.SubjectGroup, Name: "Leser"}
			in.NotEffective, in.ExpiryMissing = previewtext.NotEffective{Set: true, Count: 2}, false
			in.GroupReach = previewtext.Count{Set: true, N: 2}
		}, []string{`Gruppe Leser erhält das Zusatzrecht "Terminal" auf dem Rechner R1.`,
			"Hinweis: Die Freigabe wirkt für 2 Mitglieder von Leser auf R1 nicht.",
			"Hinweis: Die Gruppe Leser erreicht 2 Menschen."}},
		{"group one en", "en", func(in *previewtext.Input) {
			in.Subject = previewtext.Subject{Kind: domain.SubjectGroup, Name: "Leser"}
			in.NotEffective, in.ExpiryMissing = previewtext.NotEffective{Set: true, Count: 1}, false
			in.GroupReach = previewtext.Count{Set: true, N: 1}
		}, []string{`Group Leser gets the additional right "Terminal" on host R1.`,
			"Note: the grant does not take effect on R1 for 1 member of Leser.",
			"Note: the group Leser reaches one person."}},
		{"group one de", "de", func(in *previewtext.Input) {
			in.Subject = previewtext.Subject{Kind: domain.SubjectGroup, Name: "Leser"}
			in.NotEffective, in.ExpiryMissing = previewtext.NotEffective{Set: true, Count: 1}, false
		}, []string{`Gruppe Leser erhält das Zusatzrecht "Terminal" auf dem Rechner R1.`,
			"Hinweis: Die Freigabe wirkt für 1 Mitglied von Leser auf R1 nicht."}},
		{"group en", "en", func(in *previewtext.Input) {
			in.Subject = previewtext.Subject{Kind: domain.SubjectGroup, Name: "Leser"}
			in.NotEffective, in.ExpiryMissing = previewtext.NotEffective{Set: true, Count: 3}, false
		}, []string{`Group Leser gets the additional right "Terminal" on host R1.`,
			"Note: the grant does not take effect on R1 for 3 members of Leser."}},
		{"denial first", "de", func(in *previewtext.Input) {
			in.Denial = domain.ReasonOwnerOnly
			in.HostI6 = []previewtext.I6{{Host: "R1", Holder: "Alex"}}
		}, []string{`Sam erhält das Zusatzrecht "Terminal" auf dem Rechner R1.`,
			"Abgelehnt: Freigaben auf der ganzen Instanz legt nur ein Owner an.",
			"Abgelehnt: Auf R1 gilt die persönliche LLM-Anmeldung von Alex. Dort darf nur Alex eine Freigabe haben.",
			"Hinweis: Die Freigabe wirkt für Sam nicht. Auf R1 öffnen nur Owner und die Person, der der Rechner zugewiesen ist, ein Terminal.",
			"Hinweis: Die Freigabe für Sam hat kein Ablaufdatum."}},
	}
	for _, c := range cases {
		in := exampleHost(c.locale)
		c.edit(&in)
		text, _ := previewtext.Render(in)
		if want := strings.Join(c.want, "\n"); text != want {
			t.Errorf("%s:\n%s\nwant:\n%s", c.name, text, want)
		}
		for _, banned := range []string{"kann damit", "can then", "Dateien hochladen", "Upload files"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s: a line of the tree (%q)", c.name, banned)
			}
		}
	}
}

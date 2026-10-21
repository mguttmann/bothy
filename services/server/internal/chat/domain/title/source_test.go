// SPDX-License-Identifier: Apache-2.0

package title_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestSourceAtCreation (09 section 12 points 1 and 2, brief note 5): a person with a real title
// gives manual, with an absent, empty or placeholder title auto; agents and apps always auto.
func TestSourceAtCreation(t *testing.T) {
	t.Parallel()
	p := title.NewPlaceholders(title.DefaultEntries())
	for _, c := range []struct {
		by    title.By
		title string
		want  title.Source
	}{
		{title.ByPerson, "Invoice export fails", title.Manual},
		{title.ByPerson, "AI", title.Manual},
		{title.ByPerson, "", title.Auto},
		{title.ByPerson, "New chat", title.Auto},
		{title.ByPerson, "Neuer Chat 3", title.Auto},
		{title.ByPerson, "  ohne   titel  ", title.Auto},
		{title.ByOperatorAgent, "Invoice export fails", title.Auto},
		{title.ByOperatorAgent, "", title.Auto},
		{title.ByOther, "Invoice export fails", title.Auto},
		{title.ByOther, "New chat", title.Auto},
	} {
		if got := p.SourceAtCreation(c.by, c.title); got != c.want {
			t.Errorf("%v %q: %s, want %s", c.by, c.title, got, c.want)
		}
	}
}

// TestSourceOnUpdate (09 section 12 point 3, 05 thread.title.update, brief note 6): a person and
// every other principal give manual; the operator agent keeps auto and gets title_manual on
// manual.
func TestSourceOnUpdate(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		by      title.By
		current title.Source
		want    title.Source
	}{
		{title.ByPerson, title.Auto, title.Manual},
		{title.ByPerson, title.Manual, title.Manual},
		{title.ByOperatorAgent, title.Auto, title.Auto},
		{title.ByOther, title.Auto, title.Manual},
		{title.ByOther, title.Manual, title.Manual},
	} {
		got, err := title.SourceOnUpdate(c.by, c.current)
		if err != nil || got != c.want {
			t.Errorf("%v on %s: %s, %v; want %s", c.by, c.current, got, err, c.want)
		}
	}
	got, err := title.SourceOnUpdate(title.ByOperatorAgent, title.Manual)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeTitleManual || got != "" {
		t.Fatalf("operator agent on manual: %q, %v", got, err)
	}
}

// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
)

// localeSettings is a fake of instance.Settings that answers locale.default.
type localeSettings struct {
	instance.Settings
	mu     sync.Mutex
	locale string
}

func (s *localeSettings) LocaleDefault(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locale, nil
}

func (s *localeSettings) set(locale string) {
	s.mu.Lock()
	s.locale = locale
	s.mu.Unlock()
}

func mustLangs(t *testing.T, locales map[string]string) searchlang.Map {
	t.Helper()
	m, err := searchlang.New([]string{"german", "english"}, locales)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestSearchLanguage holds 14 3.1 for the directory (docs/DECISIONS.md D-M3-13-1): a person gets
// the mapping of users.locale, an agent and an app the mapping of locale.default, a locale without
// entry simple, an unknown id not_found; InstanceSearchLanguage follows locale.default.
func TestSearchLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	sam := f.add(domain.KindUser, "sam")
	agent := f.add(domain.KindAgent, "helper")
	tracker := f.add(domain.KindApp, "tracker")
	f.sql("UPDATE users SET locale = 'de' WHERE principal_id = $1", alex.ID.String())
	f.sql("UPDATE users SET locale = 'en-US' WHERE principal_id = $1", sam.ID.String())
	settings := &localeSettings{locale: "en"}
	dir := app.NewDirectory(f.repo).WithSearch(mustLangs(t, map[string]string{"de": "german", "en": "english"}), settings)
	for _, c := range []struct {
		id   ids.ID
		want string
	}{{alex.ID, "german"}, {sam.ID, "english"}, {agent.ID, "english"}, {tracker.ID, "english"}} {
		got, err := dir.SearchLanguage(ctx, c.id)
		if err != nil || got != c.want {
			t.Errorf("SearchLanguage(%s) = %q, %v, want %q", c.id, got, err, c.want)
		}
	}
	if _, err := dir.SearchLanguage(ctx, ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("SearchLanguage(unknown) = %v, want not_found", err)
	}
	if got, err := dir.InstanceSearchLanguage(ctx); err != nil || got != "english" {
		t.Errorf("InstanceSearchLanguage = %q, %v, want english", got, err)
	}
	settings.set("de-CH")
	if got, err := dir.InstanceSearchLanguage(ctx); err != nil || got != "german" {
		t.Errorf("InstanceSearchLanguage after locale.default de-CH = %q, %v, want german", got, err)
	}
	if got, err := dir.SearchLanguage(ctx, agent.ID); err != nil || got != "german" {
		t.Errorf("SearchLanguage(agent) after locale.default de-CH = %q, %v, want german", got, err)
	}

	// A map without en: the English-speaking person gets simple.
	noEnglish := app.NewDirectory(f.repo).WithSearch(mustLangs(t, map[string]string{"de": "german"}), settings)
	if got, err := noEnglish.SearchLanguage(ctx, sam.ID); err != nil || got != searchlang.Simple {
		t.Errorf("SearchLanguage(sam) without en = %q, %v, want simple", got, err)
	}

	// A directory without the search configuration fails closed instead of writing simple.
	plain := app.NewDirectory(f.repo)
	if _, err := plain.SearchLanguage(ctx, alex.ID); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("SearchLanguage without WithSearch = %v, want internal", err)
	}
	if _, err := plain.InstanceSearchLanguage(ctx); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("InstanceSearchLanguage without WithSearch = %v, want internal", err)
	}
}

// TestSearchLanguageJoinsTheUnitOfWork holds the M1-26 convention: the read joins the ambient
// transaction, so a person created in the same unit already has a language.
func TestSearchLanguageJoinsTheUnitOfWork(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := app.NewDirectory(f.repo).WithSearch(mustLangs(t, map[string]string{"de": "german"}), &localeSettings{locale: "en"})
	f.do(func(ctx context.Context) error {
		h, _ := domain.NewHandle("robin", 32)
		d, _ := domain.NewDisplayName("Robin", 64)
		p, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
		if err != nil {
			return err
		}
		if err := f.repo.InsertPrincipal(ctx, &p); err != nil {
			return err
		}
		u, err := domain.NewUser(p, "robin@example.com", "de", "Europe/Berlin")
		if err != nil {
			return err
		}
		if err := f.repo.InsertUser(ctx, u); err != nil {
			return err
		}
		got, err := dir.SearchLanguage(ctx, p.ID)
		if err != nil || got != "german" {
			t.Errorf("SearchLanguage inside the unit = %q, %v, want german", got, err)
		}
		return nil
	})
}

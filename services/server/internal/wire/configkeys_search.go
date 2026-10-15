// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// The keys of the language configuration of full-text search (docs/spec/14-suche.md 3.1).
const (
	keySearchLanguages = "search.languages"
	keySearchLocaleMap = "search.locale_map"
)

// searchConfigKeys returns the keys of the search from docs/spec/14-suche.md "Konfigurationswerte
// dieses Kapitels" that the server reads so far: the language configuration of the search
// vectors and of the query function (M3-13). The search coordinator (M3-14) adds the keys of the
// query side.
func searchConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keySearchLanguages, Kind: config.KindStringList, Default: `["german","english"]`,
			Doc:     "Text search configurations every search query is built with, besides simple.",
			Chapter: chapter14,
		},
		{
			Name: keySearchLocaleMap, Kind: config.KindStringMap, Default: `{"de":"german","en":"english"}`,
			Doc:     "Text search configuration of new messages and threads per first language subtag of the author's locale.",
			Chapter: chapter14,
		},
	}
}

// SearchLanguages returns the validated language configuration of search from v
// (search.languages and search.locale_map). The checks search_languages and search_locale_map
// have refused an invalid pair at load already, so an error means a schema without these keys
// or a hand-built Values; it names keys and positions, never values.
func SearchLanguages(v *config.Values) (searchlang.Map, error) {
	return searchlang.New(v.Strings(keySearchLanguages), v.StringMap(keySearchLocaleMap))
}

// searchLanguagesCheck is the check search_languages (docs/DECISIONS.md D-M3-13-5): the entries
// of search.languages are unique, unqualified lower-case PostgreSQL identifiers and never simple.
func searchLanguagesCheck(v *config.Values) error {
	_, err := searchlang.New(v.Strings(keySearchLanguages), nil)
	return err
}

// searchLocaleMapCheck is the check search_locale_map (docs/DECISIONS.md D-M3-13-5, 14 3.1):
// every key of search.locale_map is a first language subtag and every value is listed in
// search.languages or is simple. This is the start error of 14 3.1: a language missing from
// search.languages would never be found. An invalid search.languages is left to
// search_languages, so it is reported once.
func searchLocaleMapCheck(v *config.Values) error {
	languages := v.Strings(keySearchLanguages)
	if _, err := searchlang.New(languages, nil); err != nil {
		return nil
	}
	_, err := searchlang.New(languages, v.StringMap(keySearchLocaleMap))
	return err
}

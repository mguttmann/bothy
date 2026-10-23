// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// The keys of the language configuration of full-text search (docs/spec/14-search.md 3.1).
const (
	keySearchLanguages = "search.languages"
	keySearchLocaleMap = "search.locale_map"
)

// The keys of the query side that the start checks of M3-14 read (docs/DECISIONS.md D-M3-14-14).
const (
	keySearchPageSizeDefault  = "search.page_size_default"
	keySearchPageSizeMax      = "search.page_size_max"
	keySearchHeadlineMaxWords = "search.headline_max_words"
	keySearchHeadlineMinWords = "search.headline_min_words"
	keySearchPrefixMinChars   = "search.prefix_min_chars"
)

// maxSearchRows bounds the row counts of a search: the statements take them as int32 and read
// one row more than the limit to tell whether more follow.
const maxSearchRows = "2147483646"

// searchRecencyScaleDefault is the default of search.recency_scale (14 section 15).
const searchRecencyScaleDefault = "30d"

// searchConfigKeys returns the keys of the search from docs/spec/14-search.md "Configuration values
// of this chapter" that the server reads so far: the language configuration of the search
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
		{
			Name: "search.query_max_chars", Kind: config.KindInt, Default: "256", Min: "1",
			Doc:     "Longest search query in characters; a longer one is refused with too_long.",
			Chapter: chapter14,
		},
		{
			Name: "search.max_candidates", Kind: config.KindInt, Default: "2000", Min: "1", Max: maxSearchRows,
			Doc:     "Newest hits after filters and rights that a search ranks; reaching it sets candidates_capped.",
			Chapter: chapter14,
		},
		{
			Name: "search.rank_normalization", Kind: config.KindInt, Default: "1", Min: "0", Max: "63",
			Doc:     "Normalization bit mask of ts_rank in the relevance score.",
			Chapter: chapter14,
		},
		{
			Name: "search.recency_scale", Kind: config.KindDuration, Default: searchRecencyScaleDefault, Min: "1s",
			Doc:     "Age at which the relevance score of a hit falls to half.",
			Chapter: chapter14,
		},
		{
			Name: keySearchPageSizeDefault, Kind: config.KindInt, Default: "20", Min: "1",
			Doc:     "Hits per page of a search without a limit.",
			Chapter: chapter14,
		},
		{
			Name: keySearchPageSizeMax, Kind: config.KindInt, Default: "50", Min: "1", Max: maxSearchRows,
			Doc:     "Largest page of a search; a larger limit is capped.",
			Chapter: chapter14,
		},
		{
			Name: "search.headline_input_max_chars", Kind: config.KindInt, Default: "20000", Min: "1", Max: "2147483647",
			Doc:     "Characters of a hit's text that the highlighting reads.",
			Chapter: chapter14,
		},
		{
			Name: "search.headline_max_fragments", Kind: config.KindInt, Default: "2", Min: "0",
			Doc:     "Excerpts per hit (MaxFragments of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchHeadlineMaxWords, Kind: config.KindInt, Default: "30", Min: "1",
			Doc:     "Most words per excerpt (MaxWords of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchHeadlineMinWords, Kind: config.KindInt, Default: "10", Min: "1",
			Doc:     "Fewest words per excerpt (MinWords of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchPrefixMinChars, Kind: config.KindInt, Default: "2", Min: "1",
			Doc:     "Shortest word of the prefix search over thread titles; shorter words are dropped.",
			Chapter: chapter14,
		},
		{
			Name: "search.statement_timeout", Kind: config.KindDuration, Default: "3s", Min: minDeadline, Max: maxPGTimeout,
			Doc:     "Time limit of the statements of one search; reaching it answers 503 search_timeout.",
			Chapter: chapter14,
		},
	}
}

// searchPageSizeCheck is the check search_page_size (docs/DECISIONS.md D-M3-14-14):
// search.page_size_default is not above search.page_size_max.
func searchPageSizeCheck(v *config.Values) error {
	if v.Int(keySearchPageSizeDefault) > v.Int(keySearchPageSizeMax) {
		return fmt.Errorf("%s must not be above %s", keySearchPageSizeDefault, keySearchPageSizeMax)
	}
	return nil
}

// searchHeadlineWordsCheck is the check search_headline_words (D-M3-14-14): ts_headline refuses
// MinWords at or above MaxWords ("MinWords should be less than MaxWords"), so every search would
// fail; the start refuses the pair instead.
func searchHeadlineWordsCheck(v *config.Values) error {
	if v.Int(keySearchHeadlineMinWords) >= v.Int(keySearchHeadlineMaxWords) {
		return fmt.Errorf("%s must be below %s", keySearchHeadlineMinWords, keySearchHeadlineMaxWords)
	}
	return nil
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

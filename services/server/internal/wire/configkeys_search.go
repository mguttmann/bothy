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

// maxSearchRows bounds the row counts and the headline input of a search. The statements take them
// as int32 and read one row more than the limit, and the messaging adapter refuses a limit at or
// above 1<<20 (internal/messaging/adapters/postgres/search.go maxSearchRows), so every value the
// start accepts is one every search type runs with.
const maxSearchRows = "1048575"

// maxSearchInt4 bounds the search keys PostgreSQL reads as int4 (the prefix length parameter and
// the ts_headline options).
const maxSearchInt4 = "2147483647"

// searchRecencyScaleDefault is the default of search.recency_scale (14 section 15).
const searchRecencyScaleDefault = "30d"

// searchConfigKeys returns the keys of the search from docs/spec/14-search.md "Configuration values
// of this chapter" that the server reads so far: the language configuration of the search
// vectors and of the query function (M3-13). The search coordinator (M3-14) adds the keys of the
// query side, the text extraction (M3-15) the keys search.extract.*.
func searchConfigKeys() []config.Key {
	return append([]config.Key{
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
			Name: "search.headline_input_max_chars", Kind: config.KindInt, Default: "20000", Min: "1", Max: maxSearchRows,
			Doc:     "Characters of a hit's text that the highlighting reads.",
			Chapter: chapter14,
		},
		{
			Name: "search.headline_max_fragments", Kind: config.KindInt, Default: "2", Min: "0", Max: maxSearchInt4,
			Doc:     "Excerpts per hit (MaxFragments of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchHeadlineMaxWords, Kind: config.KindInt, Default: "30", Min: "1", Max: maxSearchInt4,
			Doc:     "Most words per excerpt (MaxWords of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchHeadlineMinWords, Kind: config.KindInt, Default: "10", Min: "1", Max: maxSearchInt4,
			Doc:     "Fewest words per excerpt (MinWords of ts_headline).",
			Chapter: chapter14,
		},
		{
			Name: keySearchPrefixMinChars, Kind: config.KindInt, Default: "2", Min: "1", Max: maxSearchInt4,
			Doc:     "Shortest word of the prefix search over thread titles; shorter words are dropped.",
			Chapter: chapter14,
		},
		{
			Name: "search.statement_timeout", Kind: config.KindDuration, Default: "3s", Min: minDeadline, Max: maxPGTimeout,
			Doc:     "Time limit of the statements of one search; reaching it answers 503 search_timeout.",
			Chapter: chapter14,
		},
	}, append(searchExtractConfigKeys(), searchAssistConfigKeys()...)...)
}

// The keys of the search assistant that the cross-checks name (14 4.10).
const (
	keySearchAssistAgentID = "search.assist.agent_id"
	keySearchAssistTimeout = "search.assist.timeout"
)

// searchAssistConfigKeys returns the keys of the search assistant (docs/spec/14-search.md 4.10 and
// section 15). The bounds of max_hits and max_terms are the maxItems of SearchAssistResult.hits and
// SearchAssistAnswer.terms in docs/api/openapi.yaml, so every configured value can be answered.
func searchAssistConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keySearchAssistAgentID, Kind: config.KindString, Default: "",
			Doc:     "Agent that answers the search assistant (its id); empty answers every question with full-text hits.",
			Chapter: chapter14,
		},
		{
			Name: keySearchAssistTimeout, Kind: config.KindDuration, Default: defaultTenSeconds, Min: minPositiveDuration,
			Doc:     "Longest wait of assistSearch for the assistant agent's answer; below http.request_timeout.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.max_candidates", Kind: config.KindInt, Default: "20", Min: "1", Max: "50",
			Doc:     "Hits each search of a question reads and most candidates sent to the assistant agent.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.max_hits", Kind: config.KindInt, Default: "8", Min: "1", Max: "20",
			Doc:     "Most hits of an answer of the search assistant, and most picks of the assistant agent.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.max_terms", Kind: config.KindInt, Default: "12", Min: "1", Max: "16",
			Doc:     "Most search terms of a question, and most further terms of an answer of the assistant agent.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.max_turns", Kind: config.KindInt, Default: "6", Min: "0", Max: "10",
			Doc:     "Most earlier questions of the popup a question may carry.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.excerpt_max_chars", Kind: config.KindInt, Default: "300", Min: "20", Max: "2000",
			Doc:     "Longest excerpt of a candidate sent to the assistant agent, in characters.",
			Chapter: chapter14,
		},
		{
			Name: "search.assist.reason_max_chars", Kind: config.KindInt, Default: "200", Min: "20", Max: "1000",
			Doc:     "Longest reason of a pick of the assistant agent, in characters.",
			Chapter: chapter14,
		},
	}
}

// searchExtractTimeoutDefault is the default of search.extract.timeout (14 section 15).
const searchExtractTimeoutDefault = "60s"

// searchExtractConfigKeys returns the keys of the text extraction of files (14 5, M3-15).
// search.extract.tika_url stays out of the schema: the Tika adapter is planned for V3
// (tools/configdoc/planned.yaml), so a configuration that sets it is refused as an unknown key.
func searchExtractConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "search.extract.max_input_bytes", Kind: config.KindInt, Default: "52428800", Min: "1",
			Doc:     "Largest file whose text is extracted; larger files end failed with input_too_large.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.max_text_bytes", Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Largest extracted text stored per file; the rest is cut and marked truncated.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.max_uncompressed_bytes", Kind: config.KindInt, Default: "209715200", Min: "1",
			Doc:     "Most bytes the read parts of one Office or OpenDocument archive may inflate to.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.max_archive_entries", Kind: config.KindInt, Default: "10000", Min: "1",
			Doc:     "Most entries an Office or OpenDocument archive may list.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.max_xml_depth", Kind: config.KindInt, Default: "256", Min: "1",
			Doc:     "Deepest element nesting an XML part of an Office or OpenDocument file may have.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.timeout", Kind: config.KindDuration, Default: searchExtractTimeoutDefault, Min: minDeadline,
			Doc:     "Time limit of reading and parsing one file for the text extraction.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.refresh_interval", Kind: config.KindDuration, Default: defaultDay, Min: "1s",
			Doc:     "Interval of the maintenance job that re-extracts rows of older extractor versions.",
			Chapter: chapter14,
		},
		{
			Name: "search.extract.refresh_batch_size", Kind: config.KindInt, Default: "500", Min: "1",
			Doc:     "Files per run of the maintenance job that re-extracts rows of older extractor versions.",
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

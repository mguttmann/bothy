// SPDX-License-Identifier: Apache-2.0

// Package searchkit is the stateless library of the full-text search (docs/spec/14-search.md
// section 10, "internal library without state"; docs/DECISIONS.md D-M3-14-1): the query and page
// types the search coordinator hands to the read operations of messaging, chat and files, the
// keyset position of a page (14 4.7), the random highlight markers and the split of a headline
// into segments (14 4.6). It is part of the kernel core (standard library and kernel/ids only), so
// the ports of search, messaging, chat and files can share it without importing each other.
//
// Who does what: the coordinator builds the tsquery text once per request (Query.TSQuery), takes
// the visible channels from Authorizer.Scope (Query.ChannelIDs) and draws the markers; a read
// operation runs one statement with "channel_id = ANY(...)", the candidate limit
// Limits.MaxCandidates, the keyset after Query.From and ts_headline over the returned rows only,
// and splits each headline with Split.
//
// Example:
//
//	m, err := searchkit.NewMarkers(nil) // crypto/rand
//	if err != nil {
//		return err
//	}
//	segs := searchkit.Split(headline, m) // [{Text: "the "}, {Text: "invoice", Match: true}]
package searchkit

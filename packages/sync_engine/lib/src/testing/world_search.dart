// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// The limits of the search handlers of the fake (brief M4-23 Scope): the configuration values
/// of 14 15 and the list page sizes of 06 6 with their defaults, so a test can lower them.
///
/// Example:
/// ```dart
/// server.world.searchSettings
///   ..queryMaxChars = 5
///   ..maxCandidates = 2;
/// ```
final class FakeSearchSettings {
  /// `search.query_max_chars` (14 4.1): a longer `q` answers `400 validation_failed` with
  /// `too_long`.
  int queryMaxChars = 256;

  /// `search.max_candidates` (14 4.5): only the newest this many matches are ranked, and the
  /// page reports `candidates_capped: true` when more matched.
  int maxCandidates = 2000;

  /// `search.page_size_default` (14 4.7): hits per page of `search` and `searchThreadTitles`
  /// without `limit`.
  int pageSizeDefault = 20;

  /// `search.page_size_max` (14 4.7): a larger `limit` is capped, not rejected.
  int pageSizeMax = 50;

  /// `search.prefix_min_chars` (14 4.9): shorter words of a prefix query are dropped.
  int prefixMinChars = 2;

  /// `api.page_size_default` (06 6): items per page of `listMyMentions` and `listActiveRuns`
  /// without `limit`.
  int listPageSizeDefault = 50;

  /// `api.page_size_max` (06 6): a larger `limit` of those lists is capped, not rejected.
  int listPageSizeMax = 200;
}

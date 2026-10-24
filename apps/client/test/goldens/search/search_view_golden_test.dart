// SPDX-License-Identifier: Apache-2.0

// The goldens of the search view D7 (brief M4-23 Tests): the filter column with a space, a
// time window and the sort, the count line, the capped note and a message, a thread and a file
// hit with highlighted segments; and the offline view with local hits and the note "Messages
// stored only on this device". Light and dark, two densities, text scaling 1 and 2, German and
// English, through provider overrides (no database).

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/search/providers.dart';
import 'package:client/search/search_controller.dart';
import 'package:client/search/search_port.dart';
import 'package:client/search/search_view.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;

import '../../support/fakes.dart';
import '../../support/structure.dart';

final DateTime now = DateTime.utc(2026, 10, 17, 12);

/// A controller that shows [fixed] and ignores input, so the golden needs no ports.
class _Fixed extends SearchQueryController {
  _Fixed(this.fixed);

  final SearchState fixed;

  @override
  SearchState build({String? channelId, String? threadId}) => fixed;

  @override
  void setInput(String input) {}

  @override
  Future<void> setFilters(SearchFilters filters) async {}

  @override
  Future<void> submit() async {}

  @override
  Future<void> loadMore() async {}
}

const List<SearchSegment> resumeSegments = [
  SearchSegment(text: '... window ', match: false),
  SearchSegment(text: 'resume_window', match: true),
  SearchSegment(text: ' is measured from the last heartbeat ...', match: false),
];

final SearchResultItem messageHit = SearchResultItem(
  id: uuid(0x701),
  type: SearchHitType.message,
  path: const ['Engineering', 'server', 'Gateway: resume after network change'],
  segments: resumeSegments,
  threadId: threadId(1),
  messageId: MessageId.parse(uuid(0x701)),
  channelId: channelId(1),
  authorId: robin,
  authorName: 'dev-runner',
  at: now.subtract(const Duration(hours: 1, minutes: 46)),
);

final SearchResultItem threadHit = SearchResultItem(
  id: threadId(2).value,
  type: SearchHitType.thread,
  path: const ['Engineering', 'releases', 'Version 0.4'],
  segments: const [
    SearchSegment(text: 'new ', match: false),
    SearchSegment(text: 'resume_window', match: true),
    SearchSegment(text: ' default', match: false),
  ],
  threadId: threadId(2),
  channelId: channelId(2),
  authorId: alex,
  at: now.subtract(const Duration(days: 1, hours: 3)),
);

final SearchResultItem fileHit = SearchResultItem(
  id: uuid(0x901),
  type: SearchHitType.file,
  path: const ['Engineering', 'server'],
  segments: const [
    SearchSegment(text: 'tuning ', match: false),
    SearchSegment(text: 'resume_window', match: true),
    SearchSegment(text: ' <b>per host</b>', match: false),
  ],
  channelId: channelId(1),
  authorId: sam,
  title: 'notes-sync.md',
  at: now.subtract(const Duration(days: 3)),
);

final SearchResultItem localMessage = SearchResultItem(
  id: uuid(0x702),
  type: SearchHitType.message,
  path: const ['Engineering', 'server', 'Gateway: resume after network change'],
  segments: resumeSegments,
  threadId: threadId(1),
  messageId: MessageId.parse(uuid(0x702)),
  channelId: channelId(1),
  authorId: sam,
  at: now.subtract(const Duration(minutes: 20)),
  local: true,
);

final SearchResultItem localTranscript = SearchResultItem(
  id: uuid(0x902),
  type: SearchHitType.transcript,
  path: const ['Support', 'general', 'Printer jam'],
  segments: const [
    SearchSegment(text: 'the ', match: false),
    SearchSegment(text: 'resume', match: true),
    SearchSegment(text: ' button is back', match: false),
  ],
  threadId: threadId(3),
  channelId: channelId(3),
  authorId: alex,
  title: 'voice-message.m4a',
  at: now.subtract(const Duration(days: 2)),
  local: true,
);

final SearchFilters filters = const SearchFilters().copyWith(
  spaceId: spaceId(1),
  time: SearchTime.week,
  sort: SearchSort.relevance,
);

final SearchState serverState = SearchState(
  input: 'resume_window',
  filters: filters,
  submitted: true,
  items: [messageHit, threadHit, fileHit],
  nextCursor: 'opaque',
  candidatesCapped: true,
);

final SearchState localState = SearchState(
  input: 'resume',
  submitted: true,
  items: [localMessage, localTranscript],
  localOnly: true,
  offline: true,
);

List<Override> overrides(SearchState state) => [
  appPortsProvider.overrideWithValue(
    testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
  ),
  personTimeZoneProvider.overrideWithValue('UTC'),
  searchQueryControllerProvider.overrideWith2((_) => _Fixed(state)),
  searchSpacesProvider.overrideWith(
    (ref) => Stream.value([
      space(1, name: 'Engineering'),
      space(2, name: 'Support'),
    ]),
  ),
  searchChannelsProvider.overrideWithValue([
    channel(1, name: 'server'),
    channel(2, name: 'releases'),
    channel(3, spaceN: 2, name: 'general'),
  ]),
  searchPrincipalsProvider.overrideWith(
    (ref) => Stream.value([
      Principal(
        id: alex,
        kind: PrincipalKind.user,
        handle: 'alex',
        displayName: 'Alex',
      ),
      Principal(
        id: robin,
        kind: PrincipalKind.agent,
        handle: 'dev-runner',
        displayName: 'dev-runner',
      ),
      Principal(
        id: sam,
        kind: PrincipalKind.user,
        handle: 'sam',
        displayName: 'Sam',
      ),
    ]),
  ),
];

Widget framed(SearchState state, String query) => ProviderScope(
  overrides: overrides(state),
  child: SizedBox(
    width: 880,
    height: 560,
    child: SearchView(initialQuery: query),
  ),
);

final Map<String, Widget Function()> views = {
  'search_results': () => framed(serverState, 'resume_window'),
  'search_local': () => framed(localState, 'resume'),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: name,
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}

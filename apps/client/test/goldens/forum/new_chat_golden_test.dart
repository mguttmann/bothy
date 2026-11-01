// SPDX-License-Identifier: Apache-2.0

// The goldens of the new chat view (brief M4-41 Tests; components.md 6; 01 6.6, 6.8 item 8;
// 09 12): a project forum with an operator agent (optional title with the agent hint, a chosen
// tag, the stored draft in the composer); a forum without an operator (required title and the
// hint to enter one); the view of a sent post with "Sending"; and of a refused post with
// "Try again". Light and dark, densities comfortable and compact, text scaling 1 and 2, German
// and English. One file per theme and form keeps each file below repo.max_file_kb.
//
// The view's providers get fixed values (the forum, its tags, the stored draft, the sent posts),
// so no database stream runs in the fake time of a golden test; the controller of the view
// needs an outbox, which one in-memory database and an idle engine (nothing connects) give.

import 'dart:async';
import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/forum/failed_post_notice.dart';
import 'package:client/forum/new_chat_view.dart';
import 'package:client/forum/providers.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/thread/providers.dart' show instanceLimitsProvider;
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/engine.dart';
import '../../support/fakes.dart';
import '../../support/new_chat_fixtures.dart';
import '../../support/structure.dart';

/// The size of the view in a cell.
const Size view = Size(440, 720);

/// The width of a cell: wider than the longest scenario label of alchemist, so the matrix has
/// a whole number of pixels on every host (docs/DECISIONS.md D-M4-07-10).
const double cellWidth = 460;

/// The outbox entry of the post that waits.
final String sending = uuid(0x801);

/// The outbox entry of the refused post.
final String refused = uuid(0x802);

/// Every cell starts with the bug tag chosen and no title (the preset of the view), so the
/// chosen state of a tag shows; [NewChatPreset.take] keeps it for the next cell.
class _Preset extends NewChatPreset {
  @override
  (String, List<ForumTagId>)? take(ChannelId forum) => ('', [tagId(1)]);
}

late LocalDatabase db;
late SyncEngine engine;
late Future<void> Function() closeAuth;
late Directory root;

/// The view of [forum] (with the sent [entry]), online, with fixed provider values.
Widget newChat(Channel forum, {String? entry}) => ProviderScope(
  overrides: [
    appPortsProvider.overrideWithValue(
      testPorts(dataRoot: root, clock: TestClock(t0)),
    ),
    localDatabaseProvider.overrideWithValue(db),
    syncEngineProvider.overrideWithValue(engine),
    connectionStateProvider.overrideWith(
      (ref) => Stream.value(const ConnectionState(ConnectionPhase.live)),
    ),
    personalSettingsProvider.overrideWithValue(
      PersonalSettings.resolve(const {}, desktop: true, hasCrashTarget: false),
    ),
    newChatPresetProvider.overrideWith(_Preset.new),
    principalNamesProvider.overrideWith(
      (ref) => Stream.value({robin: 'Robin'}),
    ),
    instanceLimitsProvider.overrideWith((ref) => Stream.value(null)),
    storedChannelProvider(forum.id.value).overrideWith((ref) async => forum),
    forumTagsProvider(forum.id.value)
        .overrideWith((ref) => Stream.value(forum == server ? serverTags : [])),
    channelDraftProvider(forum.id.value).overrideWith(
      (ref) => Stream.value(
        storedDraft(
          forum.id,
          forum == server
              ? 'The login page shows a blank screen.'
              : 'Dark mode for the docs',
        ),
      ),
    ),
    pendingPostsProvider(forum.id.value).overrideWith(
      (ref) => Stream.value([
        PendingPost(entryId: sending, title: '', failed: false, createdAt: t0),
        PendingPost(
          entryId: refused,
          title: 'Upload limit',
          failed: true,
          createdAt: t0,
        ),
      ]),
    ),
    failedPostsProvider(forum.id.value).overrideWith(
      (ref) => Stream.value([
        FailedPost(
          entryId: refused,
          title: 'Upload limit',
          contentMd: 'Restart the gateway',
          code: 'forbidden',
          reason: 'restricted tag',
        ),
      ]),
    ),
  ],
  child: SizedBox(
    width: cellWidth,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox.fromSize(
          size: view,
          child: NewChatView(forum: forum.id, sentEntry: entry),
        ),
      ],
    ),
  ),
);

/// Lets the database streams of the cells deliver and the frames settle.
Future<void> settle(WidgetTester tester) async {
  for (var i = 0; i < 20; i++) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump();
  }
}

/// The forms of the view by file name.
final Map<String, Widget Function()> forms = {
  'new_chat': () => newChat(server),
  'new_chat_title_required': () => newChat(ideas),
  'new_chat_sending': () => newChat(server, entry: sending),
  'new_chat_refused': () => newChat(server, entry: refused),
};

void main() {
  setUpAll(() async {
    root = Directory.systemTemp.createTempSync('new_chat_golden');
    // A stream that ends in the fake time of a test stops at once: drift's timer for it would
    // never fire, and the close in tearDownAll would wait for it.
    db = openInMemoryForTests(clock: () => t0, closeStreamsSynchronously: true);
    final idle = idleEngine(db);
    engine = idle.engine;
    closeAuth = idle.auth.dispose;
  });

  tearDownAll(() async {
    await engine.dispose();
    await closeAuth();
    await db.close();
    root.deleteSync(recursive: true);
  });

  // The goldens share one database, closed in tearDownAll. A query stream of it that ends in
  // the fake time of a test must not leave a timer behind: drift's close waits for that timer,
  // which never fires once the fake time of its test is gone, so the close of tearDownAll hung
  // for the 12 minutes of the test timeout (issue #238, run 40975481761).
  testWidgets(
    'a database stream that ends in fake time leaves no timer for the close',
    (tester) async {
      await tester.pumpWidget(
        StreamBuilder<List<OutboxEntryView>>(
          stream: engine.outbox.watchThread(null),
          builder: (context, snapshot) => const SizedBox(),
        ),
      );
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 5)),
      );
      await tester.pumpWidget(const SizedBox());
    },
  );

  for (final MapEntry(key: name, value: form) in forms.entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'the view $name in ${theme.name}, two densities, text scale 1 and 2, de and en',
        fileName: '${name}_${theme.name}',
        pumpBeforeTest: settle,
        builder: () => dsGoldenMatrix(
          name: name,
          themes: [theme],
          densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: () =>
              Material(type: MaterialType.transparency, child: form()),
        ),
      );
    }
  }
}

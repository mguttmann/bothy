// SPDX-License-Identifier: Apache-2.0

// The level menus (brief M4-33 Tests and Design note 2; 13 3.1, 3.2): setting a level, muting
// until a time, inheriting again, each through the outbox with the local row changed at once;
// the inherited level with its source; "Following" for a thread the server follows; the fixed
// times of "Mute until".

import 'dart:convert';

import 'package:client/menus/channel_menu.dart';
import 'package:client/menus/level_menu.dart';
import 'package:client/menus/space_menu.dart';
import 'package:client/menus/thread_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sync/notify_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

const String rowText = 'the row';

/// Opens the menu of the row by a right click.
Future<void> openMenu(WidgetTester tester) async {
  await tester.tap(find.text(rowText), buttons: kSecondaryMouseButton);
  await tester.pumpAndSettle();
}

Future<void> tapKey(WidgetTester tester, String key) async {
  await tester.tap(find.byKey(ValueKey(key)));
  await tester.pumpAndSettle();
}

void main() {
  late ColumnHost host;
  var ids = 0;

  Future<void> seed(WidgetTester tester) => tester.runAsync(() async {
    final db = host.db;
    await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
    await db.structureDao.upsertChannelIfNewer(channel(1, name: 'general'));
    await db.threadsDao.upsertIfNewer(thread(1, title: 'Release notes'));
  });

  Future<void> pumpMenu(WidgetTester tester, Widget Function() menu) async {
    host = await ColumnHost.open(tester);
    await seed(tester);
    await host.pump(
      tester,
      Builder(builder: (_) => menu()),
      overrides: [
        notifyWriterProvider.overrideWithValue(
          NotifyWriter(
            host.engine,
            host.db,
            () =>
                '0192f4a0-7b1c-7000-8000-${(++ids).toRadixString(16).padLeft(12, '0')}',
          ),
        ),
      ],
    );
  }

  Future<List<OutboxEntry>> outbox(WidgetTester tester) async =>
      (await tester.runAsync(host.db.outboxDao.allEntries))!;

  Future<List<NotificationPreference>> rows(WidgetTester tester) async =>
      (await tester.runAsync(host.db.personalDao.notificationPreferences))!;

  /// Pumps until the outbox holds [count] entries (the writes run in the real zone).
  Future<List<OutboxEntry>> waitOutbox(WidgetTester tester, int count) async {
    final watch = Stopwatch()..start();
    while (true) {
      final all = await outbox(tester);
      if (all.length >= count) return all;
      if (watch.elapsed > const Duration(minutes: 3)) {
        fail('the outbox did not reach $count entries');
      }
      await tester.pump();
    }
  }

  final channel1 = channel(1, name: 'general');

  testWidgets('setting a level writes the row at once and one outbox entry', (
    tester,
  ) async {
    await pumpMenu(
      tester,
      () => ChannelMenu(channel: channel1, child: const Text(rowText)),
    );
    await openMenu(tester);
    await tapKey(tester, 'menu.notifications');
    expect(
      find.text('Inherit (Mentions from the default of the instance)'),
      findsOneWidget,
    );
    await tapKey(tester, 'menu.level.mute');
    final entries = await waitOutbox(tester, 1);
    expect(entries.single.kind, NotifyOps.putNotificationPreference);
    expect(jsonDecode(entries.single.payload), {
      'target_kind': 'channel',
      'target_id': channelId(1).value,
      'level': 'mute',
    });
    final stored = (await rows(tester)).single;
    expect(stored.level, NotificationLevel.mute);
    expect(stored.version, const ResourceVersion(1));
    await host.dispose(tester);
  });

  testWidgets('mute for an hour keeps the level that applies afterwards', (
    tester,
  ) async {
    await pumpMenu(
      tester,
      () => ChannelMenu(channel: channel1, child: const Text(rowText)),
    );
    await openMenu(tester);
    await tapKey(tester, 'menu.muteUntil');
    await tapKey(tester, 'menu.mute.hour');
    final entries = await waitOutbox(tester, 1);
    final until = t0.add(const Duration(hours: 1));
    expect(jsonDecode(entries.single.payload), {
      'target_kind': 'channel',
      'target_id': channelId(1).value,
      'level': 'mentions',
      'muted_until': until.toIso8601String(),
    });
    // The menu now names the end of the mute.
    await openMenu(tester);
    await tapKey(tester, 'menu.notifications');
    expect(find.textContaining('muted until'), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('inherit deletes the own row through the outbox', (tester) async {
    await pumpMenu(
      tester,
      () => ChannelMenu(channel: channel1, child: const Text(rowText)),
    );
    await tester.runAsync(
      () => host.db.personalDao.upsertNotificationPreferenceIfNewer(
        NotificationPreference(
          targetKind: NotificationTargetKind.channel,
          targetId: channelId(1).value,
          level: NotificationLevel.all,
          version: const ResourceVersion(4),
        ),
      ),
    );
    await host.settle(tester);
    await openMenu(tester);
    await tapKey(tester, 'menu.notifications');
    await tapKey(tester, 'menu.inherit');
    final entries = await waitOutbox(tester, 1);
    expect(entries.single.kind, NotifyOps.deleteNotificationPreference);
    expect(await rows(tester), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('a channel names what it inherits from its space', (
    tester,
  ) async {
    await pumpMenu(
      tester,
      () => ChannelMenu(channel: channel1, child: const Text(rowText)),
    );
    await tester.runAsync(
      () => host.db.personalDao.upsertNotificationPreferenceIfNewer(
        NotificationPreference(
          targetKind: NotificationTargetKind.space,
          targetId: spaceId(1).value,
          level: NotificationLevel.mute,
          version: const ResourceVersion(1),
        ),
      ),
    );
    await host.settle(tester);
    await openMenu(tester);
    await tapKey(tester, 'menu.notifications');
    expect(find.text('inherits: Mute'), findsOneWidget);
    expect(find.text('Inherit (Mute from the space)'), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a followed thread shows Following', (tester) async {
    await pumpMenu(
      tester,
      () => ThreadMenu(
        thread: threadId(1),
        title: 'Release notes',
        channel: channel1,
        child: const Text(rowText),
      ),
    );
    await tester.runAsync(
      () => host.db.personalDao.upsertNotificationPreferenceIfNewer(
        NotificationPreference(
          targetKind: NotificationTargetKind.thread,
          targetId: threadId(1).value,
          level: NotificationLevel.all,
          version: const ResourceVersion(1),
        ),
      ),
    );
    await host.settle(tester);
    await openMenu(tester);
    await tapKey(tester, 'menu.notifications');
    expect(find.text('Following'), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('the space menu opens with Shift+F10 and sets the space', (
    tester,
  ) async {
    await pumpMenu(
      tester,
      () => SpaceMenu(
        space: space(1, name: 'Team'),
        child: const Focus(autofocus: true, child: Text(rowText)),
      ),
    );
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.f10);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pumpAndSettle();
    await tapKey(tester, 'menu.notifications');
    await tapKey(tester, 'menu.level.all');
    final entries = await waitOutbox(tester, 1);
    expect(jsonDecode(entries.single.payload), {
      'target_kind': 'space',
      'target_id': spaceId(1).value,
      'level': 'all',
    });
    await host.dispose(tester);
  });

  testWidgets('choose date opens the date picker after the menu closed', (
    tester,
  ) async {
    await pumpMenu(
      tester,
      () => ChannelMenu(channel: channel1, child: const Text(rowText)),
    );
    await openMenu(tester);
    await tapKey(tester, 'menu.muteUntil');
    await tapKey(tester, 'menu.mute.date');
    expect(tester.takeException(), isNull);
    expect(find.byType(DatePickerDialog), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    await host.dispose(tester);
  });

  group('mute choices', () {
    test('one hour later, and tomorrow 08:00 in the person\'s zone', () {
      final c = muteChoices(DateTime.utc(2026, 10, 20, 21), 'Europe/Berlin');
      expect(c.oneHour, DateTime.utc(2026, 10, 20, 22));
      // 2026-10-20 23:00 in Berlin; tomorrow 08:00 CEST is 06:00 UTC.
      expect(c.tomorrow, DateTime.utc(2026, 10, 21, 6));
    });

    test('tomorrow across the change to winter time', () {
      // Berlin leaves summer time on 2026-10-25 at 03:00.
      final c = muteChoices(DateTime.utc(2026, 10, 24, 12), 'Europe/Berlin');
      expect(c.tomorrow, DateTime.utc(2026, 10, 25, 7));
    });

    test('an unknown zone counts as UTC', () {
      final c = muteChoices(DateTime.utc(2026, 10, 20, 21), 'Mars/Base');
      expect(c.tomorrow, DateTime.utc(2026, 10, 21, 8));
    });
  });
}

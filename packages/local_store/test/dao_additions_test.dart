// SPDX-License-Identifier: Apache-2.0

// The DAO methods M4-14 added: deleting a setting and a notification preference by the
// version rule (04 14, `deleted: true`) and reading the reactions of a message inside a
// transaction (the `me` flag of `reaction.*`); M4-33 the watched quiet hours rules; M4-41 the
// thread of a message by its client_msg_id.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test('a newer deletion removes the setting, an older one keeps it', () async {
    final dao = db.personalDao;
    await dao.upsertSettingIfNewer(
      const UserSetting(
        key: 'composer.enter_sends',
        value: true,
        version: ResourceVersion(3),
      ),
    );
    expect(
      await dao.deleteSettingIfNewer(
        'composer.enter_sends',
        const ResourceVersion(3),
      ),
      isFalse,
    );
    expect(await dao.watchSettings().first, hasLength(1));
    expect(
      await dao.deleteSettingIfNewer(
        'composer.enter_sends',
        const ResourceVersion(4),
      ),
      isTrue,
    );
    expect(await dao.watchSettings().first, isEmpty);
  });

  test('a newer deletion removes the notification preference', () async {
    final dao = db.personalDao;
    final target = uuid(0x101);
    await dao.upsertNotificationPreferenceIfNewer(
      NotificationPreference(
        targetKind: NotificationTargetKind.fromCode('channel'),
        targetId: target,
        level: NotificationLevel.fromCode('all'),
        version: const ResourceVersion(2),
      ),
    );
    final kind = NotificationTargetKind.fromCode('channel');
    expect(
      await dao.deleteNotificationPreferenceIfNewer(
        kind,
        target,
        const ResourceVersion(1),
      ),
      isFalse,
    );
    expect(await dao.notificationPreferences(), hasLength(1));
    expect(
      await dao.deleteNotificationPreferenceIfNewer(
        kind,
        target,
        const ResourceVersion(5),
      ),
      isTrue,
    );
    expect(await dao.notificationPreferences(), isEmpty);
  });

  test(
    'watchQuietHours lists the rules by ID and follows changes (M4-33)',
    () async {
      final dao = db.personalDao;
      QuietHours rule(int n, String start) => QuietHours(
        id: QuietHoursId.parse(uuid(0x200 + n)),
        weekdays: 31,
        startLocal: start,
        endLocal: '07:00',
        enabled: true,
        version: const ResourceVersion(1),
      );
      List<String> starts(List<QuietHours> rules) => [
        for (final r in rules) r.startLocal,
      ];
      await dao.upsertQuietHoursIfNewer(rule(2, '20:00'));
      await dao.upsertQuietHoursIfNewer(rule(1, '19:00'));
      expect(starts(await dao.watchQuietHours().first), ['19:00', '20:00']);
      final next = dao.watchQuietHours().skip(1).first;
      await pumpEventQueue();
      await dao.deleteQuietHours(rule(2, '20:00').id);
      expect(starts(await next), ['19:00']);
    },
  );

  test('reactionsOf reads the summaries inside a transaction', () async {
    final dao = db.messagesDao;
    final id = messageId(1, 1);
    await dao.applyReactionSummary(
      ReactionSummary(messageId: id, emoji: 'b', count: 1, me: false),
    );
    await dao.applyReactionSummary(
      ReactionSummary(messageId: id, emoji: 'a', count: 2, me: true),
    );
    final inside = await db.transaction(() => dao.reactionsOf(id));
    expect([for (final r in inside) r.emoji], ['a', 'b']);
    expect(inside.first.me, isTrue);
  });

  test('watchPinned lists the pinned messages of a thread in order', () async {
    final dao = db.messagesDao;
    for (final seq in [1, 2, 3]) {
      await dao.upsertIfNewer(message(1, seq));
    }
    await dao.upsertIfNewer(message(2, 1));
    await dao.applyPin(messageId(1, 3), pinned: true);
    await dao.applyPin(messageId(1, 1), pinned: true);
    await dao.applyPin(messageId(2, 1), pinned: true);
    final pinned = await dao.watchPinned(threadId(1)).first;
    expect([for (final m in pinned) m.threadSeq.value], [1, 3]);
  });

  test(
    'threadOfClientMsg names the thread of a sent message (M4-41)',
    () async {
      final dao = db.messagesDao;
      await dao.upsertIfNewer(message(2, 1));
      // The first message of a new post carries the outbox entry as its client_msg_id.
      expect(await dao.threadOfClientMsg(messageId(2, 1)), threadId(2));
      expect(await dao.threadOfClientMsg(messageId(3, 1)), isNull);
    },
  );
}

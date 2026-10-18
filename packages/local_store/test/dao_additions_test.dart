// SPDX-License-Identifier: Apache-2.0

// The DAO methods M4-14 added: deleting a setting and a notification preference by the
// version rule (04 14, `deleted: true`) and reading the reactions of a message inside a
// transaction (the `me` flag of `reaction.*`).

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
}

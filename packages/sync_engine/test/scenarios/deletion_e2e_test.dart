// SPDX-License-Identifier: Apache-2.0

// Deletion and undo end to end (01 6.18 item 8, D-DEL-5): the fake deletes a chat, a channel
// and a space of the standard world; SyncEngine on its local database removes each with its
// content and a removal notice, and brings it back with the undo, a channel with its access
// and a space with its channels through REST.

import 'package:domain/domain.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('a chat, a channel and a space: deleted and restored', () async {
    await withScenario((server, w, client) async {
      final world = server.world;
      final db = client.db;
      await client.start();
      await client.focusThread(w.printer);
      w.post(w.sam, w.printer, 'The printer is out of paper');
      await waitFor(
        'the printer message',
        () async => (await client.messages(w.printer)).isNotEmpty,
      );
      final printer = ThreadId.parse(w.printer);

      world.deleteThread(w.sam.id, w.printer);
      await waitFor(
        'the chat removal',
        () => client.notices.contains('removed:thread:${w.printer}'),
      );
      expect(await db.threadsDao.byId(printer), isNull);
      expect(await client.messages(w.printer), isEmpty);
      world.restoreThread(w.sam.id, w.printer);
      await waitFor(
        'the restored chat',
        () async => await db.threadsDao.byId(printer) != null,
      );

      world.grantCapability(w.alex.id, w.support, 'manage');
      world.deleteChannel(w.alex.id, w.support);
      await waitFor(
        'the channel removal',
        () async => !await client.holdsChannel(w.support),
      );
      expect(await db.threadsDao.byId(printer), isNull);
      world.restoreChannel(w.alex.id, w.support);
      await waitFor(
        'the restored channel with its access and chats',
        () async =>
            await db.structureDao
                    .watchChannelAccess(ChannelId.parse(w.support))
                    .first !=
                null &&
            await db.threadsDao.byId(printer) != null,
      );

      world.deleteSpace(w.alex.id, w.space);
      await waitFor(
        'the space removal',
        () => client.notices.contains('removed:space:${w.space}'),
      );
      expect(await client.holdsChannel(w.general), isFalse);
      world.restoreSpace(w.alex.id, w.space);
      await waitFor(
        'the restored space with its channels',
        () async =>
            await db.structureDao.spaceById(SpaceId.parse(w.space)) != null &&
            await client.holdsChannel(w.general) &&
            await client.holdsChannel(w.support),
      );
      expect(server.violations, isEmpty);
    });
  });
}

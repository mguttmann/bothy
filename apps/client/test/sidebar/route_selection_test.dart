// SPDX-License-Identifier: Apache-2.0

// The selection of a route (brief M4-19 Design note 5): a forum opened by a link before its
// channel is stored shows its post list once the channel arrives.

import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';

void main() {
  test(
    'a channel stored after the route opened is selected with its kind',
    () async {
      final db = openInMemoryForTests(clock: () => t0);
      final c = ProviderContainer(
        overrides: [localDatabaseProvider.overrideWithValue(db)],
      );
      final path = '/c/${channelId(2).value}';
      final sub = c.listen(routeSelectionProvider(path), (_, _) {});
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(sub.read().value?.forum, isNull);

      await db.structureDao.upsertSpaceIfNewer(space(1));
      await db.structureDao.upsertChannelIfNewer(
        channel(2, kind: ChannelKind.forum),
      );
      final end = DateTime.now().add(const Duration(seconds: 5));
      while (sub.read().value?.forum != channelId(2)) {
        if (DateTime.now().isAfter(end)) fail('the forum was never selected');
        await Future<void>.delayed(const Duration(milliseconds: 20));
      }
      sub.close();
      c.dispose();
      await db.close();
    },
  );
}

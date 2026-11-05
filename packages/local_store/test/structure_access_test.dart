// SPDX-License-Identifier: Apache-2.0

// The caller's access to stored spaces and channels (brief FIX-access-on-created): a row stored
// from `space.created` or `channel.created` has no level until a view reports it, and the sync
// engine finds such rows to load their views.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  const manage = AccessView(
    level: AccessLevel.trigger,
    capabilities: [Capability.manage],
  );

  test('a channel without a view has no access until one is stored', () async {
    final dao = db.structureDao;
    final c1 = channel(1);
    final c2 = channel(2);
    await dao.upsertSpaceIfNewer(space(1));
    await dao.upsertChannelIfNewer(c1);
    await dao.upsertChannelIfNewer(c2);
    await dao.putChannelAccess(c1.id, manage);

    expect(await dao.channelAccess(c1.id), manage);
    expect(await dao.channelAccess(c2.id), isNull);
    expect(await dao.channelAccess(channel(9).id), isNull);
    expect(await dao.channelsWithoutAccess(), [c2.id]);

    await dao.putChannelAccess(c2.id, manage);
    expect(await dao.channelsWithoutAccess(), isEmpty);
  });

  test('a space without a view has no access until one is stored', () async {
    final dao = db.structureDao;
    final s1 = space(1);
    final s2 = space(2);
    await dao.upsertSpaceIfNewer(s1);
    await dao.upsertSpaceIfNewer(s2);
    await dao.putSpaceAccess(s2.id, manage);

    expect(await dao.spaceAccess(s1.id), isNull);
    expect(await dao.spaceAccess(s2.id), manage);
    expect(await dao.spacesWithoutAccess(), [s1.id]);
  });
}

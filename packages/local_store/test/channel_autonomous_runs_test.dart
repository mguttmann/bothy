// SPDX-License-Identifier: Apache-2.0

// The channel switch `allow_autonomous_runs` in the local mirror (AR-08): the sync snapshot and
// every channel event of a server of stage AR leave the member out, so storing such a channel
// keeps the value that `getChannel` or the switch's answer stored, also at the same version.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  Future<bool?> stored(Channel c) async =>
      (await db.structureDao.channelById(c.id))!.allowAutonomousRuns;

  test('a channel without the member keeps the stored switch', () async {
    final dao = db.structureDao;
    final c = channel(1);
    await dao.upsertSpaceIfNewer(space(1));
    await dao.upsertChannelIfNewer(c);
    expect(await stored(c), isNull);

    await dao.upsertChannelIfNewer(
      c.copyWith(allowAutonomousRuns: true, version: const ResourceVersion(2)),
    );
    expect(await stored(c), isTrue);

    // A channel.updated without the member (a rename) does not forget it.
    await dao.upsertChannelIfNewer(
      c.copyWith(name: 'renamed', version: const ResourceVersion(3)),
    );
    expect(await stored(c), isTrue);
    expect((await dao.channelById(c.id))!.name, 'renamed');

    // A carried value still replaces it.
    await dao.upsertChannelIfNewer(
      c.copyWith(allowAutonomousRuns: false, version: const ResourceVersion(4)),
    );
    expect(await stored(c), isFalse);
  });

  test('putChannelAutonomousRuns writes at the same version, not over a '
      'newer known value', () async {
    final dao = db.structureDao;
    final c = channel(1);
    await dao.upsertSpaceIfNewer(space(1));
    // The event of the switch won the race: version 2 stored without the member.
    await dao.upsertChannelIfNewer(
      c.copyWith(version: const ResourceVersion(2)),
    );
    await dao.putChannelAutonomousRuns(
      c.id,
      const ResourceVersion(2),
      allowed: true,
    );
    expect(await stored(c), isTrue);

    await dao.upsertChannelIfNewer(
      c.copyWith(allowAutonomousRuns: false, version: const ResourceVersion(3)),
    );
    await dao.putChannelAutonomousRuns(
      c.id,
      const ResourceVersion(2),
      allowed: true,
    );
    expect(await stored(c), isFalse);

    // Not stored: nothing to write.
    await dao.putChannelAutonomousRuns(
      channel(9).id,
      const ResourceVersion(1),
      allowed: true,
    );
    expect(await dao.channelById(channel(9).id), isNull);
  });
}

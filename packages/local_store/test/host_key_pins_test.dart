// SPDX-License-Identifier: Apache-2.0

// The pinned host keys of this device (`host_key_pins`, 15 6.1, 23.3; brief M4-36 Design note
// 2): a pin is written and replaced per host, the cache cleanup of 15 6.3 and the removal of
// targets (04 8.3) never touch it, and only the wipe of all local data deletes it.

import 'dart:io';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

/// The pin of host [n] with key generation [generation].
StoredHostKeyPin pin(int n, {int generation = 1, int minutes = 0}) =>
    StoredHostKeyPin(
      hostId: HostId.parse(uuid(0x700 + n)),
      noisePublicKey: 'key-$n-$generation',
      keyGeneration: generation,
      keyFingerprint: 'FP$n$generation',
      firstSeenAt: at(minutes),
    );

void main() {
  group('in memory', () {
    late FakeClock clock;
    late LocalDatabase db;
    setUp(() {
      clock = FakeClock();
      db = openInMemoryForTests(clock: clock.call);
    });
    tearDown(() => db.close());

    test('a pin is read back as written, first seen in UTC', () async {
      await db.hostKeyPinsDao.write(pin(1));
      final read = await db.hostKeyPinsDao.read(pin(1).hostId);
      expect(read, pin(1));
      expect(read!.firstSeenAt.isUtc, isTrue);
      expect(await db.hostKeyPinsDao.read(pin(2).hostId), isNull);
    });

    test('writing the pin of a host again replaces it', () async {
      await db.hostKeyPinsDao.write(pin(1));
      await db.hostKeyPinsDao.write(pin(1, generation: 2, minutes: 5));
      expect(
        await db.hostKeyPinsDao.read(pin(1).hostId),
        pin(1, generation: 2, minutes: 5),
      );
      expect(await db.hostKeyPinsDao.all(), [
        pin(1, generation: 2, minutes: 5),
      ]);
    });

    test(
      'the cache cleanup leaves every pin, however old and whatever the size',
      () async {
        await db.hostKeyPinsDao.write(pin(1));
        await db.hostKeyPinsDao.write(pin(2));
        clock.advance(const Duration(days: 400));
        await CacheCleaner(
          db,
          maxAge: Duration.zero,
          maxBytes: 0,
          now: clock.call,
        ).run();
        expect(await db.hostKeyPinsDao.all(), [pin(1), pin(2)]);
      },
    );

    test('removing targets, a host among them, leaves every pin', () async {
      await db.hostKeyPinsDao.write(pin(1));
      await db.removeTargets([
        TargetRef(RemovedTargetKind.host, pin(1).hostId.value),
        TargetRef.space(SpaceId.parse(uuid(0x101))),
      ]);
      expect(await db.hostKeyPinsDao.all(), [pin(1)]);
    });
  });

  test('the wipe deletes the pins with the database', () async {
    final dir = await tempDir();
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/store.db');
    final keys = FixedKeySource(testKeyA);
    var db = await openLocalStore(file: file, keys: keys, inBackground: false);
    await db.hostKeyPinsDao.write(pin(1));
    await db.wipe();
    expect(file.existsSync(), isFalse);

    db = await openLocalStore(file: file, keys: keys, inBackground: false);
    expect(await db.hostKeyPinsDao.all(), isEmpty);
    await db.close();
  });
}

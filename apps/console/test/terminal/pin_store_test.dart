// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../support/fakes.dart';
import 'terminal_fakes.dart';

final _pin = HostKeyPin(
  hostId: buildHostId,
  noisePublicKey: hostNoiseKey,
  keyGeneration: 2,
  fingerprint: hostFingerprint,
  firstSeen: DateTime.utc(2026, 10, 20, 9, 30),
);

/// A signed-in console over [db].
Future<(ConsoleController, TestBrowser)> _signedIn(FakePinDatabase db) async {
  final b = TestBrowser();
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, consoleSession(stepUp: true));
  final c = ConsoleController(
    b.environment,
    terminalPins: IndexedDbHostKeyPinStore(db),
  );
  await c.start(Uri.parse('/live'));
  expect(c.state, isA<ConsoleSignedIn>());
  return (c, b);
}

void main() {
  group('IndexedDbHostKeyPinStore', () {
    test('writes a pin and reads it back with every field', () async {
      final db = FakePinDatabase();
      final store = IndexedDbHostKeyPinStore(db);
      await store.write(_pin);
      final read = await store.read(buildHostId);
      expect(read, isNotNull);
      expect(read!.hostId, buildHostId);
      expect(read.noisePublicKey, hostNoiseKey);
      expect(read.keyGeneration, 2);
      expect(read.fingerprint, hostFingerprint);
      expect(read.firstSeen, DateTime.utc(2026, 10, 20, 9, 30));
      // One record per host in the store of the origin.
      expect(db.records.keys, [buildHostId]);
    });

    test('a host without a pin reads null, a broken record as well', () async {
      final db = FakePinDatabase();
      final store = IndexedDbHostKeyPinStore(db);
      expect(await store.read(buildHostId), isNull);
      db.records[buildHostId] = {'noise_public_key': 7};
      expect(await store.read(buildHostId), isNull);
    });

    test('clear deletes the database', () async {
      final db = FakePinDatabase();
      final store = IndexedDbHostKeyPinStore(db);
      await store.write(_pin);
      await store.clear();
      expect(db.deletions, 1);
      expect(await store.read(buildHostId), isNull);
    });
  });

  group('the pins are deleted before E7', () {
    test('on the logout of the person', () async {
      final db = FakePinDatabase();
      final (c, b) = await _signedIn(db);
      await IndexedDbHostKeyPinStore(db).write(_pin);
      b.server.routes['POST /api/v1/auth/logout'] = (_) => const Answer(204);
      db.deleteGate = Completer<void>();
      final done = c.logout();
      await pumpEventQueue();
      // The server ended the session, but E7 waits for the deletion.
      expect(b.server.to('POST', '/api/v1/auth/logout'), hasLength(1));
      expect(c.state, isNot(isA<ConsoleLoggedOut>()));
      db.deleteGate!.complete();
      await done;
      expect(db.deletions, 1);
      expect(db.records, isEmpty);
      expect(c.state, isA<ConsoleLoggedOut>());
    });

    test('on a 401 that ends the session (clearing cookie)', () async {
      final db = FakePinDatabase();
      final (c, b) = await _signedIn(db);
      await IndexedDbHostKeyPinStore(db).write(_pin);
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          const Answer(401, {'code': 'session_revoked', 'status': 401});
      db.deleteGate = Completer<void>();
      await c.refreshSession();
      await pumpEventQueue();
      expect(c.state, isNot(isA<ConsoleLoggedOut>()));
      db.deleteGate!.complete();
      await pumpEventQueue();
      expect(db.deletions, 1);
      expect(db.records, isEmpty);
      final state = c.state;
      expect(state, isA<ConsoleLoggedOut>());
      expect((state as ConsoleLoggedOut).reasonCode, 'session_revoked');
    });

    test(
      'a start without a session deletes pins an expired one left',
      () async {
        final db = FakePinDatabase();
        await IndexedDbHostKeyPinStore(db).write(_pin);
        final b = TestBrowser();
        final c = ConsoleController(
          b.environment,
          terminalPins: IndexedDbHostKeyPinStore(db),
        );
        await c.start(Uri.parse('/live'));
        expect(c.state, isA<ConsoleSignedOut>());
        expect(db.deletions, 1);
        expect(db.records, isEmpty);
      },
    );

    test('a step-up answer keeps the session and the pins', () async {
      final db = FakePinDatabase();
      final (c, b) = await _signedIn(db);
      await IndexedDbHostKeyPinStore(db).write(_pin);
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          const Answer(401, {'code': 'step_up_required', 'status': 401});
      await c.refreshSession(afterViewEnd: true);
      await pumpEventQueue();
      expect(db.deletions, 0);
      expect(c.state, isA<ConsoleSignedIn>());
    });

    test('a failing deletion still shows E7', () async {
      final db = _FailingPinDatabase();
      final b = TestBrowser();
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(stepUp: true));
      b.server.routes['POST /api/v1/auth/logout'] = (_) => const Answer(204);
      final c = ConsoleController(
        b.environment,
        terminalPins: IndexedDbHostKeyPinStore(db),
      );
      await c.start(Uri.parse('/live'));
      await c.logout();
      expect(db.attempts, 1);
      expect(c.state, isA<ConsoleLoggedOut>());
    });
  });
}

final class _FailingPinDatabase extends FakePinDatabase {
  int attempts = 0;

  @override
  Future<void> delete() async {
    attempts++;
    throw StateError('blocked');
  }
}

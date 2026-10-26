// SPDX-License-Identifier: Apache-2.0

// The wiring of sharing on iOS and macOS (brief M5-06 Design notes 3, 5 and 7) against the fake
// server with the app's real auth session, database and engine: after the login the app
// creates the child session (`createExtensionSession`, `client_component = share_ext`) and
// writes the settings; a confirmed send writes the recent targets; `access.changed` that takes
// the write level away removes the target; an inbox entry the extension left is sent once with
// its `client_msg_id`; a return to the foreground creates a deleted child session again; logout
// deletes the child session and the shared data.

import 'dart:async';
import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/share/providers.dart';
import 'package:client/share/share_apple.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import 'share_fakes.dart';
import 'share_harness.dart' show inboxEntry, targetRecord;

/// A lifecycle the test moves between foreground and background.
class _Lifecycle implements AppLifecycle {
  final StreamController<AppActivity> _changes =
      StreamController<AppActivity>.broadcast();
  AppActivity _current = AppActivity.foreground;

  void set(AppActivity activity) {
    _current = activity;
    _changes.add(activity);
  }

  @override
  Stream<AppActivity> get changes => _changes.stream;

  @override
  AppActivity get current => _current;
}

const String _shared = '0192f4a0-7b1c-7000-8000-000000000201';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late FakeShareHost host;
  late _Lifecycle lifecycle;
  late List<Object?> created;

  Future<void> signIn() async {
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
  }

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    host = FakeShareHost();
    lifecycle = _Lifecycle();
    created = [];
    fake = await FakeInstance.start(
      appleShare: AppleShare(host: host),
      lifecycle: lifecycle,
    );
    host.inbox[_shared] = inboxEntry(
      _shared,
      instanceId: fake.server.world.instanceId,
      target: targetRecord(
        'thread',
        fake.seed.support,
        threadId: fake.seed.printer,
        path: 'support > Printer',
      ),
      comment: 'Shared while offline',
    );
    fake.server.rest.handle('createExtensionSession', (r) {
      created.add(r.body);
      final token = r.headers['authorization']!.split(' ').last;
      final parent = r.world.sessions.values.firstWhere(
        (s) => s.accessToken == token,
      );
      final device = r.world.devices[parent.deviceId]!;
      return FakeResponse(201, {
        'access_token': 'child-access-${created.length}',
        'token_type': 'DPoP',
        'expires_in': 900,
        'refresh_token': 'child-refresh-${created.length}',
        'refresh_expires_at': parent.expiresAt,
        'session': {
          ...parent.toJson(),
          'id': r.world.nextId(),
          'client_component': 'share_ext',
        },
        'device': device.toJson(self: true),
      });
    });
    c = appContainer(fake);
    // The app widget watches the provider; a listener keeps it active the same way.
    final share = c.listen(appleShareProvider, (_, _) {});
    expect(share.read(), isNotNull);
    await signIn();
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test('the child session and the settings after the login; recent targets '
      'after a send and without the write level; logout clears all', () async {
    final id = fake.server.world.instanceId;
    await eventually('child session', () => host.keychain[id] != null);
    expect(created, [
      {'client_component': 'share_ext'},
    ]);
    expect(host.keychain[id]!.refreshToken, 'child-refresh-1');
    expect(host.keychain[id]!.serverUrl, fake.server.baseUri.toString());
    await eventually(
      'settings with limits',
      () => host.settings?.fileMaxUploadBytes != null,
    );
    expect(host.settings!.instanceId, id);
    expect(host.settings!.recentTargetsMax, 8);
    expect(host.settings!.messageMaxAttachments, 10);

    final engine = c.read(syncEngineProvider)!;
    await ThreadActions(engine)
        .send(ThreadId.parse(fake.seed.releaseNotes), 'Hello Sam');
    await eventually(
      'recent targets',
      () => host.lastPaths?.contains('general > Release notes') ?? false,
    );
    expect(host.written.last.first.path, 'general > Release notes');

    fake.server.world.setAccess(fake.seed.alex.id, fake.seed.general, 'read');
    await eventually(
      'target removed',
      () => !(host.lastPaths?.contains('general > Release notes') ?? true),
    );

    await engine.outbox.depth.firstWhere((n) => n == 0);
    await c.read(onboardingControllerProvider.notifier).logout();
    await eventually('cleared', () => host.cleared.contains('data'));
    expect(host.cleared, contains('session $id'));
    expect(host.keychain, isEmpty);
  });

  test(
    'an inbox entry of the extension is sent once with its client_msg_id',
    () async {
      final world = fake.server.world;
      await eventually(
        'sent',
        () => world
            .messagesOf(fake.seed.printer)
            .any((m) => m['client_msg_id'] == _shared),
      );
      expect(host.deletedEntries, [_shared]);
      final db = c.read(localDatabaseProvider)!;
      await eventually(
        'row removed after the confirmation',
        () async => (await db.shareInboxDao.all()).isEmpty,
      );
      expect(
        world
            .messagesOf(fake.seed.printer)
            .where((m) => m['client_msg_id'] == _shared),
        hasLength(1),
      );
      expect(created, hasLength(1));
    },
  );

  test('the foreground creates a deleted child session again', () async {
    final id = fake.server.world.instanceId;
    await eventually('child session', () => host.keychain[id] != null);
    // The extension met 401 session_revoked and deleted the child.
    host.keychain.clear();
    lifecycle
      ..set(AppActivity.background)
      ..set(AppActivity.foreground);
    await eventually('again', () => host.keychain[id] != null);
    expect(created, hasLength(2));
    expect(host.keychain[id]!.refreshToken, 'child-refresh-2');
  });
}

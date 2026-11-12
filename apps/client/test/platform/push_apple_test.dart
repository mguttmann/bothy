// SPDX-License-Identifier: Apache-2.0

// Push on iOS and macOS (brief M5-04 Design note 5) against a fake Pigeon host: registration
// only after the permission, again on a new token, unregistration and keychain deletion on
// logout, tap routing, the preview mode for the extension and the display counters as client
// metrics. A failing call leaves a log line without content.

import 'package:client/platform/push_apple.dart';
import 'package:client/platform/secure_store.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:flutter/services.dart' show PlatformException;
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/push_fakes.dart';

/// A runner whose App Group write and token call fail with a message that must not be logged.
class _BrokenHost extends FakePushHost {
  bool failToken = true;

  @override
  Future<void> writeSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  ) async => throw PlatformException(
    code: 'unavailable',
    message: 'keychain says no for $instanceId',
  );

  @override
  Future<String> deviceToken() async => failToken
      ? throw PlatformException(
          code: 'unavailable',
          message: 'keychain says no',
        )
      : token;
}

void main() {
  late FakePushHost host;
  late FakeRegistrations registrations;
  late ClientMetrics metrics;
  late MemorySecureBackend backend;
  late ApplePush push;

  PushAccount account() => PushAccount(
    instanceId: instance,
    serverUrl: Uri.parse('https://chat.example.com'),
    secrets: InstanceSecrets(backend, instance),
    registrations: registrations,
    fetchTimeout: const Duration(seconds: 20),
  );

  setUp(() {
    host = FakePushHost();
    registrations = FakeRegistrations();
    metrics = ClientMetrics();
    backend = MemorySecureBackend();
    push = ApplePush(host: host, metrics: metrics);
  });

  test(
    'registers only after the permission, asked once at the first thread',
    () async {
      await push.threadOpened();
      expect(host.permissionRequests, 0, reason: 'never before the login');
      await push.signedIn(account(), previewMode: 'full');
      expect(registrations.calls, isEmpty);
      expect(host.permissionRequests, 0);

      host.grant = false;
      await push.threadOpened();
      await push.threadOpened();
      expect(host.permissionRequests, 1);
      expect(registrations.calls, isEmpty);

      final second = ApplePush(host: host, metrics: metrics);
      host.grant = true;
      await second.signedIn(account(), previewMode: 'full');
      await second.threadOpened();
      expect(registrations.calls, ['register aa11 sandbox com.example.chat']);
      final stored = host.keychain[instance]!;
      expect(stored.registrationId, 'reg-1');
      expect(stored.previewToken, 'bothy_pvw_EXAMPLE1');
      expect(stored.serverUrl, 'https://chat.example.com');
      // Dart keeps only the APNs token, never the preview token.
      expect(backend.values.values, isNot(contains('bothy_pvw_EXAMPLE1')));
    },
  );

  test('registers again on a new token, not on the same one', () async {
    host.authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.signedIn(account(), previewMode: 'full');
    expect(registrations.calls, hasLength(1));
    host.token = 'bb22';
    await push.signedIn(account(), previewMode: 'full');
    expect(registrations.calls, [
      'register aa11 sandbox com.example.chat',
      'register bb22 sandbox com.example.chat',
    ]);
    expect(host.keychain[instance]!.registrationId, 'reg-2');
  });

  test('logout unregisters and deletes the keychain items', () async {
    host.authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.logout();
    expect(registrations.calls.last, 'unregister reg-1');
    expect(host.keychain, isEmpty);
    expect(backend.values, isEmpty);
    // Every other end of the account deletes the items too.
    await push.signedIn(account(), previewMode: 'full');
    await push.forget(instance);
    expect(host.keychain, isEmpty);
    expect(host.settings.last, 'deleted $instance');
  });

  test('a tap opens the thread at the message', () async {
    host.pending = [
      PushTap(instanceId: instance, threadId: 't1', messageId: 'm1'),
    ];
    final paths = <String>[];
    push.targets.listen(paths.add);
    await push.listen();
    push.onTap(PushTap(instanceId: instance, threadId: 't2'));
    push.onTap(PushTap(instanceId: instance, threadId: ''));
    await pumpEventQueue();
    expect(paths, ['/t/t1?m=m1', '/t/t2']);
  });

  test(
    'the preview mode goes to the App Group at start and on a change',
    () async {
      await push.signedIn(account(), previewMode: 'names');
      await push.previewModeChanged('none');
      await push.previewModeChanged('none');
      expect(host.settings, ['$instance names 20000', '$instance none 20000']);
    },
  );

  test('the display counters become client metrics', () async {
    host.counters = PushDisplayCounters(
      bucketBounds: [0.5, 1, 10],
      bucketCounts: [2, 0, 1],
      placeholders: 3,
    );
    await push.signedIn(account(), previewMode: 'full');
    final entries = {for (final e in metrics.take()) e.metric: e};
    expect(entries[ClientMetric.pushPlaceholder]!.value, 3);
    expect(entries[ClientMetric.pushDisplayDuration]!.buckets, {0.5: 2, 10: 1});
    await push.signedIn(account(), previewMode: 'full');
    expect(metrics.take(), isEmpty);
  });

  test('a failing runner or server call leaves one line without content in the '
      'log and stops nothing else', () async {
    final lines = <String>[];
    final broken = _BrokenHost()..authorized = true;
    final logged = ApplePush(host: broken, metrics: metrics, log: lines.add);
    await logged.signedIn(account(), previewMode: 'full');
    expect(lines, [
      'push: settings failed (PlatformException)',
      'push: registration failed (PlatformException)',
    ]);
    expect(registrations.calls, isEmpty, reason: 'no token, no registration');
    for (final line in lines) {
      expect(line, isNot(contains(instance)));
      expect(line, isNot(contains('keychain says no')));
    }

    lines.clear();
    broken.failToken = false;
    registrations.failure = StateError('bothy_pvw_EXAMPLE');
    await logged.signedIn(account(), previewMode: 'names');
    expect(lines, [
      'push: settings failed (PlatformException)',
      'push: registration failed (StateError)',
    ]);
    expect(lines.join(), isNot(contains('bothy_pvw_EXAMPLE')));
    expect(broken.keychain, isEmpty);
  });

  test('a build without the shared groups does nothing', () async {
    host
      ..available = false
      ..authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.threadOpened();
    await push.logout();
    expect(registrations.calls, isEmpty);
    expect(host.settings, isEmpty);
    expect(host.permissionRequests, 0);
  });
}

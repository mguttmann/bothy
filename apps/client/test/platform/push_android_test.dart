// SPDX-License-Identifier: Apache-2.0

// Push on Android (brief M5-05 Design note 4) against a fake Pigeon host: registration only
// after the permission, again on a token change, unregistration and store deletion on logout,
// tap routing, the preview mode for the push service and its display counters as client
// metrics.

import 'package:client/platform/push_android.dart';
import 'package:client/platform/secure_store.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/push_fakes.dart';

void main() {
  late FakeAndroidPushHost host;
  late FakeRegistrations registrations;
  late ClientMetrics metrics;
  late MemorySecureBackend backend;
  late AndroidPush push;

  PushAccount account() => PushAccount(
    instanceId: instance,
    serverUrl: Uri.parse('https://chat.example.com'),
    secrets: InstanceSecrets(backend, instance),
    registrations: registrations,
    fetchTimeout: const Duration(seconds: 20),
  );

  setUp(() {
    host = FakeAndroidPushHost();
    registrations = FakeRegistrations();
    metrics = ClientMetrics();
    backend = MemorySecureBackend();
    push = AndroidPush(host: host, metrics: metrics);
  });

  test('registers with fcm only after the permission, asked once', () async {
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

    final second = AndroidPush(host: host, metrics: metrics);
    host.grant = true;
    await second.signedIn(account(), previewMode: 'full');
    await second.threadOpened();
    expect(registrations.calls, ['register fcm fcm-token-1 com.example.chat']);
    final stored = host.store[instance]!;
    expect(stored.registrationId, 'reg-1');
    expect(stored.previewToken, 'bothy_pvw_EXAMPLE1');
    expect(stored.serverUrl, 'https://chat.example.com');
    // Dart keeps only the FCM token, never the preview token.
    expect(backend.values.values, isNot(contains('bothy_pvw_EXAMPLE1')));
  });

  test('a token change registers again, the same token does not', () async {
    host.authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.signedIn(account(), previewMode: 'full');
    expect(registrations.calls, hasLength(1));
    // FCM rotates the token while the app runs (onNewToken).
    host.fcmToken = 'fcm-token-2';
    push.onNewToken('fcm-token-2');
    await pumpEventQueue();
    expect(registrations.calls, [
      'register fcm fcm-token-1 com.example.chat',
      'register fcm fcm-token-2 com.example.chat',
    ]);
    expect(host.store[instance]!.registrationId, 'reg-2');
  });

  test('a new token without the permission registers nothing', () async {
    await push.signedIn(account(), previewMode: 'full');
    push.onNewToken('fcm-token-2');
    await pumpEventQueue();
    expect(registrations.calls, isEmpty);
  });

  test('logout unregisters and clears the Kotlin store', () async {
    host.authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.logout();
    expect(registrations.calls.last, 'unregister reg-1');
    expect(host.store, isEmpty);
    expect(backend.values, isEmpty);
    await push.signedIn(account(), previewMode: 'full');
    await push.forget(instance);
    expect(host.store, isEmpty);
    expect(host.settings.last, 'deleted $instance');
  });

  test('a tap opens the thread at the message', () async {
    host.pending = [
      AndroidPushTap(instanceId: instance, threadId: 't1', messageId: 'm1'),
    ];
    final paths = <String>[];
    push.targets.listen(paths.add);
    await push.listen();
    push.onTap(AndroidPushTap(instanceId: instance, threadId: 't2'));
    push.onTap(AndroidPushTap(instanceId: instance, threadId: ''));
    await pumpEventQueue();
    expect(paths, ['/t/t1?m=m1', '/t/t2']);
  });

  test(
    'the preview mode goes to the push service at start and on a change',
    () async {
      await push.signedIn(account(), previewMode: 'names');
      await push.previewModeChanged('none');
      await push.previewModeChanged('none');
      expect(host.settings, ['$instance names 20000', '$instance none 20000']);
    },
  );

  test('the display counters become client metrics', () async {
    host.counters = AndroidPushCounters(
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

  test('a build without Firebase does nothing', () async {
    host
      ..available = false
      ..authorized = true;
    await push.signedIn(account(), previewMode: 'full');
    await push.threadOpened();
    push.onNewToken('fcm-token-2');
    await push.logout();
    await pumpEventQueue();
    expect(registrations.calls, isEmpty);
    expect(host.settings, isEmpty);
    expect(host.permissionRequests, 0);
  });
}

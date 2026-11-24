// SPDX-License-Identifier: Apache-2.0

// The wiring of push on iOS and macOS (brief M5-04 Design note 5) against the fake server: after
// the login the extension's App Group gets the instance, `notify.preview_mode` and
// `client.push.fetch_timeout`; a change of the preview mode follows; logout deletes the keychain
// items through the push and the wiper. The fake server has no push endpoints, so the
// registration fails and nothing is stored, which the app survives; every return to the
// foreground tries it again (13 13).

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/notifications/push_provider.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/push_apple.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart';
import '../support/push_fakes.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late FakePushHost host;
  late SwitchLifecycle lifecycle;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    host = FakePushHost()..authorized = true;
    lifecycle = SwitchLifecycle();
    fake = await FakeInstance.start(
      push: ApplePush(host: host, metrics: ClientMetrics()),
      lifecycle: lifecycle,
    );
    c = appContainer(fake);
    // The app widget watches the provider; a listener keeps it active the same way.
    final push = c.listen(pushPlatformProvider, (_, _) {});
    expect(push.read(), isNotNull);
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    // The engine runs before the settings change, so its write is sent and acknowledged
    // before the logout wipes the database.
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test('the signed-in account and its preview mode reach the App Group; '
      'logout deletes the items', () async {
    final id = c.read(signedInServicesProvider)!.instance.instanceId;
    await eventually(
      'settings written',
      () => host.settings.contains('$id full 20000'),
    );
    await c
        .read(settingsRepositoryProvider)!
        .set(NotifyKeys.previewMode, 'none');
    await eventually(
      'mode follows',
      () => host.settings.contains('$id none 20000'),
    );
    expect(host.keychain, isEmpty, reason: 'the fake has no registerPush');
    final engine = c.read(syncEngineProvider)!;
    await engine.outbox.depth.firstWhere((n) => n == 0);

    await c.read(onboardingControllerProvider.notifier).logout();
    await eventually('deleted', () => host.settings.contains('deleted $id'));
  });

  test(
    'a return to the foreground tries the failed registration again',
    () async {
      await eventually('registration tried', () => host.tokenReads > 0);
      // Let the attempt of the login end before counting.
      await eventually(
        'settings written',
        () => host.settings.any((l) => l.endsWith(' full 20000')),
      );
      final before = host.tokenReads;
      lifecycle.set(AppActivity.background);
      expect(
        host.tokenReads,
        before,
        reason: 'the background registers nothing',
      );
      // An attempt still running takes in a return at that moment, so the test returns until a
      // new attempt starts.
      await eventually('registration tried again', () {
        lifecycle
          ..set(AppActivity.background)
          ..set(AppActivity.foreground);
        return host.tokenReads > before;
      });
      expect(host.keychain, isEmpty, reason: 'the fake has no registerPush');
    },
  );
}

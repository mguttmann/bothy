// SPDX-License-Identifier: Apache-2.0

// The real `device_key` plugin of the host opens an instance (docs/decisions.d/
// FIX-windows-keystore.md): on Windows the plugin refuses the step-up key by design
// (D-M4-11-2), so the app connects and signs in with its DPoP key alone and registers the
// device without `step_up_key`. Runs in the CI job `apps (Windows)` (platform-tests.json).

import 'dart:io';
import 'dart:typed_data';

import 'package:client/platform/device_key_signer.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'the platform device keys connect and sign in without a step-up key',
    (tester) async {
      final client = dk.DeviceKeyClient();
      final keys = PlatformDeviceKeyStore(
        client,
        () => const dk.StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
      );
      final app = await TestApp.start(
        tester,
        platform: Platform.operatingSystem,
        deviceKeys: keys,
      );
      final scope = app.fake.server.world.instanceId;
      try {
        // Before the fix E1 showed "keychain refused" here and never reached E2.
        await app.connect();
        final opened = await keys.ensure(scope);
        if (Platform.isWindows) expect(opened.stepUp, isNull);
        expect(await opened.dpop.sign(Uint8List(8)), hasLength(64));

        await app.loginWithPassword();
        await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
        await app.waitLive();
        final device = app.fake.server.world.devices[app.deviceId]!;
        expect(device.stepUpJwk == null, opened.stepUp == null);
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
        await client.delete(scope);
      }
    },
  );
}

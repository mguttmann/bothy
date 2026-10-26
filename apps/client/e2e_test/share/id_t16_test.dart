// SPDX-License-Identifier: Apache-2.0

// ID-T16 with the real Share Extension (docs/spec/08-identity.md 7.4 and 15; brief M5-06 Tests):
// the app on the iOS simulator signs in to the real server of the Compose stack (infra/compose/
// e2e.sh with the ID-T16 overlay: short access tokens, a reauth lead time below their lifetime,
// and no reuse grace, so a second use of a refresh token would be detected at once) and creates
// the Share Extension's child session. With the share of the sample photo into "general" ready to
// send and the child's access token expired, the app refreshes its own family at the moment the
// extension's send refreshes the child's family; both succeed, the shared photo arrives, the server
// records no refused refresh (the reuse detection, cause refresh_reuse) and revokes no session.
// tools/e2e/id_t16.sh runs it; it is not part of integration_test/, so the platform jobs
// without a server never pick it up.

import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/share/share_apple.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import '../../integration_test/share/support/share_flow.dart'
    show
        photosApp,
        nativeWait,
        shareAutomatorConfig,
        shareNewestPhoto,
        untilInBackground;
import '../support/actors.dart';
import '../support/e2e_app.dart';
import '../support/e2e_network.dart';
import '../support/e2e_world.dart';

/// `auth.access_token_ttl` of the ID-T16 overlay, handed in by the runner.
const int _ttlSeconds = int.fromEnvironment('ID_T16_ACCESS_TOKEN_TTL_S');

/// `auth.clock_leeway` of the ID-T16 overlay: the server still accepts a token this long after
/// its expiry.
const int _leewaySeconds = int.fromEnvironment(
  'ID_T16_CLOCK_LEEWAY_S',
  defaultValue: -1,
);

void main() {
  final world = E2eWorld.fromEnvironment();

  patrolTest(
    'ID-T16: app and Share Extension refresh at the same time without reuse detection',
    platformAutomatorConfig: shareAutomatorConfig,
    ($) async {
      expect(
        _ttlSeconds,
        greaterThan(0),
        reason: 'run through tools/e2e/id_t16.sh',
      );
      final ttl = const Duration(seconds: _ttlSeconds);
      final started = DateTime.now().toUtc();
      final network = await E2eNetwork.start(world.server, world.caPem);
      // The app's connections trust the test CA and go to the stack directly: the tunnel of
      // E2eNetwork.install would also take the HTTP connection of patrol to its automation
      // server, which is no stack address. The owner's requests use the network directly.
      HttpOverrides.global = _TrustTestCa(world.caPem);
      final owner = await ApiActor.enroll(
        world,
        network,
        world.alex.code,
        name: 'Alex desk',
      );
      final app = await E2eApp.start(
        $.tester,
        world,
        platform: 'ios',
        deviceKeys: PlatformDeviceKeyStore(
          dk.DeviceKeyClient(),
          () =>
              const dk.StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
        ),
        appleShare: AppleShare(host: ShareAppleHostApi()),
      );
      Future<List<Session>> sessions() => network.direct(() async {
        final page = await owner.api.getDevicesApi().listMySessions();
        return page.data!.items.toList();
      });
      try {
        await app.connect();
        await app.enroll(await owner.issueCode(world.alex.id));
        await app.waitFor(find.byKey(const ValueKey('phone.shell')));
        await app.waitLive();
        final appSession = app.state.services!.auth.session!;

        Session? child;
        await app.untilAsync(
          'the child session of the Share Extension',
          () async {
            child = (await sessions())
                .where(
                  (s) =>
                      s.clientComponent == SessionClientComponentEnum.shareExt,
                )
                .firstOrNull;
            return child != null;
          },
        );
        // From here Photos and the Share Extension are in front and the app is in the
        // background, where the binding draws no frames: the waits poll without pumping.
        await shareNewestPhoto($);
        final ios = $.platform.ios;
        await ios.enterText(
          const IOSSelector(identifier: 'share.search'),
          text: 'general',
          appId: photosApp,
          timeout: nativeWait,
        );
        await ios.waitUntilVisible(
          const IOSSelector(
            identifier: 'share.target.0',
            labelContains: 'general',
          ),
          appId: photosApp,
          timeout: nativeWait,
        );
        await ios.tap(
          const IOSSelector(identifier: 'share.target.0'),
          appId: photosApp,
          timeout: nativeWait,
        );
        // The child's access token was issued at the latest now (the search may have
        // refreshed it). Once it has expired and the server's clock leeway has passed too, the
        // server refuses it, so the send can only succeed by refreshing the child's family.
        // The app's own token is renewed by the gateway's reauth ahead of its expiry (the
        // overlay keeps sync.reauth_lead_time below the lifetime), so the test makes the app
        // refresh its family at the very moment of the send.
        final childExpiry = DateTime.now().toUtc().add(
          ttl + const Duration(seconds: _leewaySeconds),
        );
        await untilInBackground(
          'the child access token expired',
          () => DateTime.now().toUtc().isAfter(childExpiry),
        );
        late final String renewed;
        await Future.wait([
          app.state.services!.auth.refresh.refreshNow().then(
            (t) => renewed = t,
          ),
          ios.tap(
            const IOSSelector(identifier: 'share.send'),
            appId: photosApp,
            timeout: nativeWait,
          ),
        ]);

        await _untilInBackground('the shared photo in general', () async {
          final messages = await owner.messages(world.generalThreadId);
          return messages.any((m) => m.attachments.isNotEmpty);
        });
        expect(renewed, isNotEmpty, reason: 'the app refreshed its own family');
        final refused = await network.direct(
          () => owner.api.getAuditApi().listAuditEvents(
            action: 'auth.token.refresh',
            decision: AuditDecision.deny,
            from: started,
          ),
        );
        expect(refused.data!.items, isEmpty, reason: 'no reuse detection');
        final live = {for (final s in await sessions()) s.id};
        expect(live, containsAll([appSession.sid, child!.id]));
      } finally {
        await app.stop();
        await owner.dispose();
        await network.stop();
      }
    },
  );
}

/// Polls [condition] without pumping (the app is in the background, where no frame comes) until
/// it holds; fails naming [what] after the [patience].
Future<void> _untilInBackground(
  String what,
  Future<bool> Function() condition,
) async {
  final watch = Stopwatch()..start();
  while (!await condition()) {
    if (watch.elapsed >= patience) fail('timed out waiting for $what');
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
}

/// Every `HttpClient` of the process trusts the test CA of the run besides the system's roots.
final class _TrustTestCa extends HttpOverrides {
  _TrustTestCa(this._ca);

  final List<int> _ca;

  @override
  HttpClient createHttpClient(SecurityContext? context) =>
      super.createHttpClient(
        SecurityContext(withTrustedRoots: true)
          ..setTrustedCertificatesBytes(_ca),
      );
}

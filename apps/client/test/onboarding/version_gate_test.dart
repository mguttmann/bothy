// SPDX-License-Identifier: Apache-2.0

// The version gate of the app against the fake server (docs/spec/18-release.md 9; brief M7-13
// requirements 13 to 16): below `client.min_version.<platform>` the app opens no gateway
// connection, sends no authenticated request and shows the update path, also when the instance
// raises the minimum while the app runs, and its outbox and local data stay; below
// `client.recommended_version.<platform>` it works and shows a notice that the person dismisses
// once per version; `4007` shows "Update required" and the engine stops reconnecting.

import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/update/update_notice.dart';
import 'package:client/app/update/update_path.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart';

/// A build of 1.4.2 with an update address, as a release writes it (brief M7-13 Design note 2).
const BuildBranding release = BuildBranding(
  appName: 'ExampleChat',
  urlScheme: 'chat-example',
  bundleId: 'com.example.chat',
  defaultServer: 'https://chat.example.com',
  appVersion: '1.4.2',
  updateUrl: 'https://apps.example.com/chat',
);

void main() {
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  late RecordingUpdateLauncher launcher;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    launcher = RecordingUpdateLauncher();
    f = await FakeInstance.start(
      branding: release,
      updateLauncher: launcher,
      platform: 'ios',
    );
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);
  OnboardingController control() =>
      c.read(onboardingControllerProvider.notifier);
  void rules(Map<String, Object?> rule) => f.server.world.minClientVersions = [
    {'platform': 'ios', ...rule},
  ];

  Future<void> signIn() async {
    await connected(c, f);
    await control().loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    // The reads after the snapshot (the instance) run after `live`; they belong to the
    // session the gate admitted.
    await eventually(
      'the reads after the snapshot',
      () => f.server.requests.any((r) => r.operationId == 'getInstance'),
    );
  }

  bool authenticated(FakeRequestRecord r) =>
      r.headers.keys.any((k) => k.toLowerCase() == 'authorization');

  test(
    'below the minimum the app does not connect and offers the update path',
    () async {
      rules({'min_version': '2.0.0'});
      await connected(c, f);
      expect(state().step, OnboardingStep.server);
      expect(state().serverStatus, ServerStatus.appTooOld);
      expect(state().minVersion, '2.0.0');
      expect(state().services, isNull);
      expect(f.server.gateway.connections, isEmpty);
      expect(f.server.requests.where(authenticated), isEmpty);
      expect(f.server.requests.map((r) => r.operationId).toSet(), {
        'getDiscovery',
      });
      // The path of the build: the address the release compiled in (iOS: TestFlight or store).
      expect(
        c.read(appPortsProvider).updatePath,
        UpdatePath(url: Uri.parse('https://apps.example.com/chat')),
      );
    },
  );

  test('a minimum raised while the app runs: no reconnect, no request, the outbox and data stay', () async {
    await signIn();
    final services = state().services!;
    final network = f.ports.network as SwitchNetwork;
    // The device goes offline; a message waits in the outbox.
    network.set(false);
    f.server.disconnect(deviceId: services.auth.session!.deviceId.value);
    await eventually(
      'offline',
      () => services.engine.state.phase == ConnectionPhase.offline,
    );
    await services.engine.outbox.enqueue(
      OutboxRequest(
        kind: MessageOps.createMessage,
        threadId: ThreadId.parse(f.seed.releaseNotes),
        payload: const {'content_md': 'Still here'},
      ),
    );
    // Meanwhile the instance raises the minimum above this build.
    rules({'min_version': '1.5.0'});
    final connections = f.server.gateway.connections.length;
    final before = f.server.requests.length;
    network.set(true);
    await eventually(
      'E1 shows "App too old"',
      () =>
          state().step == OnboardingStep.server &&
          state().serverStatus == ServerStatus.appTooOld,
    );
    expect(state().minVersion, '1.5.0');
    expect(state().services, isNull);
    expect(f.server.gateway.connections.length, connections);
    // The gate's discovery is the only request; the closed services send nothing.
    expect(f.server.requests.skip(before).map((r) => r.operationId).toSet(), {
      'getDiscovery',
    });
    expect(f.server.requests.skip(before).where(authenticated), isEmpty);
    // Nothing was deleted: the local data and the session stay.
    expect(f.instanceFolder.existsSync(), isTrue);
    expect(
      f.secure.values.keys.where((k) => k.endsWith('/refresh_token')),
      isNotEmpty,
    );

    // The update arrives (here: the instance lowers the minimum again); the stored session
    // resumes and the message that waited in the outbox reaches the server.
    f.server.world.minClientVersions = [];
    await control().connect(f.server.baseUri.toString());
    expect(state().step, OnboardingStep.signedIn);
    await eventually(
      'the waiting message reaches the server',
      () => f.server.requests.any(
        (r) =>
            r.operationId == 'createMessage' &&
            r.status == 201 &&
            '${r.body}'.contains('Still here'),
      ),
    );
  });

  test('below the recommended version the app works and shows a notice, dismissed once per version', () async {
    rules({'min_version': '1.0.0', 'recommended_version': '1.5.0'});
    await signIn();
    // The main view watches the notice; so does the test.
    final notice = c.listen(recommendedUpdateProvider, (_, _) {});
    addTearDown(notice.close);
    await eventually('the notice', () => notice.read() == '1.5.0');
    await c.read(updateNoticeDismissalProvider.notifier).dismiss('1.5.0');
    expect(notice.read(), isNull);
    // The next reconnect learns of a newer recommendation: the notice shows again.
    rules({'min_version': '1.0.0', 'recommended_version': '1.6.0'});
    f.server.faults.closeGateway(1001);
    await eventually(
      'the notice of the next version',
      () => notice.read() == '1.6.0',
    );
    expect(
      state().services?.engine.state.phase,
      isNot(ConnectionPhase.updateRequired),
    );
    // The dismissal is kept across starts.
    expect(f.secure.values['app/update_notice_dismissed'], '1.5.0');
  });

  test(
    '4007 shows "Update required" and the engine stops reconnecting',
    () async {
      await signIn();
      final engine = state().services!.engine;
      final connections = f.server.gateway.connections.length;
      f.server.faults.closeGateway(4007);
      await eventually(
        'update required',
        () => engine.state.phase == ConnectionPhase.updateRequired,
      );
      expect(engine.state.reason, ConnectionReason.protocolUnsupported);
      // No later event brings it back: a user action does nothing either.
      await engine.connection.connectNow();
      expect(engine.state.phase, ConnectionPhase.updateRequired);
      expect(f.server.gateway.connections.length, connections);
      // The account stays: "Update required" is no sign-out.
      expect(state().step, OnboardingStep.signedIn);
      expect(c.read(signedInServicesProvider), isNotNull);
    },
  );
}

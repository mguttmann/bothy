// SPDX-License-Identifier: Apache-2.0

// E1 Server address (01 5.6, R-PR-14): each discovery result leads to its text; a TLS error
// offers no continue; setup_required shows the setup pointer; an old app shows "App too old";
// the address is remembered.

import 'dart:io';

import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/e1_server_address.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';

Future<void> pumpE1(
  WidgetTester tester,
  ServerStatus status, {
  String? minVersion,
  ValueChanged<String>? onConnect,
}) async {
  desktopWindow(tester);
  await tester.pumpWidget(
    hostScreen(
      ServerAddressScreen(
        address: 'https://chat.example.com',
        status: status,
        minVersion: minVersion,
        onConnect: onConnect ?? (_) {},
      ),
    ),
  );
}

void main() {
  testWidgets('each result shows its text', (tester) async {
    final texts = {
      ServerStatus.invalidAddress: en.onboardingAddressInvalid,
      ServerStatus.noInstance: en.onboardingNoInstance,
      ServerStatus.keychainUnavailable: en.onboardingKeychainUnavailable,
      ServerStatus.openFailed: en.onboardingOpenFailed,
      ServerStatus.certificateInvalid: en.onboardingCertificateInvalid,
      ServerStatus.notSetUp: en.onboardingNotSetUp,
      ServerStatus.appTooOld: en.onboardingAppTooOld('2.0.0'),
    };
    for (final MapEntry(key: status, value: text) in texts.entries) {
      await pumpE1(tester, status, minVersion: '2.0.0');
      expect(find.text(text), findsOneWidget, reason: status.name);
    }
    await pumpE1(tester, ServerStatus.idle);
    expect(find.byKey(const ValueKey('e1.status.idle')), findsNothing);
    expect(find.text(en.onboardingNoInstance), findsNothing);
  });

  testWidgets('the setup pointer names the setup wizard on the host', (
    tester,
  ) async {
    await pumpE1(tester, ServerStatus.notSetUp);
    expect(find.textContaining('install.sh'), findsOneWidget);
  });

  testWidgets('an invalid certificate offers no way on: no exception button', (
    tester,
  ) async {
    await pumpE1(tester, ServerStatus.certificateInvalid);
    final buttons = tester.widgetList<ButtonStyleButton>(
      find.byWidgetPredicate((w) => w is ButtonStyleButton),
    );
    expect(buttons, hasLength(1), reason: 'only Connect');
    expect(find.text(en.onboardingConnect), findsOneWidget);
  });

  testWidgets(
    'Connect and Enter send the typed address; connecting disables it',
    (tester) async {
      final sent = <String>[];
      await pumpE1(tester, ServerStatus.idle, onConnect: sent.add);
      await tester.enterText(
        find.byKey(const ValueKey('e1.address')),
        'chat.example.com',
      );
      await tester.tap(find.byKey(const ValueKey('e1.connect')));
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(sent, ['chat.example.com', 'chat.example.com']);
      await pumpE1(tester, ServerStatus.connecting, onConnect: sent.add);
      expect(find.text(en.onboardingConnecting), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('e1.connect')));
      expect(sent, hasLength(2));
    },
  );

  group('controller', () {
    late Directory root;
    setUp(() async => root = await Directory.systemTemp.createTemp('e1'));
    tearDown(() => root.delete(recursive: true));

    Future<(ProviderContainer, ScriptedDiscovery, MemorySecureBackend)> start(
      DiscoveryOutcome answer, {
      MemorySecureBackend? backend,
    }) async {
      final discovery = ScriptedDiscovery(answer);
      final secure = backend ?? MemorySecureBackend();
      final container = ProviderContainer(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(
              dataRoot: root,
              discovery: discovery,
              secureBackend: secure,
            ),
          ),
        ],
      );
      container.read(onboardingControllerProvider);
      await Future<void>.delayed(Duration.zero);
      return (container, discovery, secure);
    }

    test('the first start suggests the default server of the build', () async {
      final (c, _, _) = await start(const DiscoveryNoInstance());
      expect(
        c.read(onboardingControllerProvider).address,
        testBranding.defaultServer,
      );
      expect(c.read(onboardingControllerProvider).step, OnboardingStep.server);
      c.dispose();
    });

    test(
      'each outcome sets its status; nothing is remembered without an instance',
      () async {
        final cases = <DiscoveryOutcome, ServerStatus>{
          const DiscoveryNoInstance(): ServerStatus.noInstance,
          const DiscoveryUnreachable(): ServerStatus.noInstance,
          const DiscoveryCertificateInvalid(): ServerStatus.certificateInvalid,
          const DiscoverySetupRequired(): ServerStatus.notSetUp,
          const DiscoveryAppTooOld('3.0.0'): ServerStatus.appTooOld,
        };
        for (final MapEntry(key: outcome, value: status) in cases.entries) {
          final (c, _, secure) = await start(outcome);
          await c
              .read(onboardingControllerProvider.notifier)
              .connect('chat.example.com');
          final state = c.read(onboardingControllerProvider);
          expect(state.serverStatus, status);
          expect(state.step, OnboardingStep.server);
          expect(await AppPreferences(secure).serverAddress(), isNull);
          c.dispose();
        }
        final (c, discovery, _) = await start(const DiscoveryNoInstance());
        await c.read(onboardingControllerProvider.notifier).connect('ftp://x');
        expect(
          c.read(onboardingControllerProvider).serverStatus,
          ServerStatus.invalidAddress,
        );
        expect(discovery.asked, isEmpty);
        c.dispose();
      },
    );

    test('a found instance is remembered, goes to E2 and is asked again at the next start', () async {
      final instance = InstanceInfo(
        address: Uri.parse('https://chat.example.com'),
        instanceId: '0193f2a7-0000-7000-8000-0000000000aa',
        productName: 'Team Chat',
        localEnabled: true,
      );
      final (c, _, secure) = await start(DiscoveryFound(instance));
      await c
          .read(onboardingControllerProvider.notifier)
          .connect('chat.example.com');
      expect(c.read(onboardingControllerProvider).step, OnboardingStep.login);
      expect(
        c.read(onboardingControllerProvider).instance?.productName,
        'Team Chat',
      );
      expect(
        await AppPreferences(secure).serverAddress(),
        Uri.parse('https://chat.example.com'),
      );
      expect(
        c.read(onboardingControllerProvider).serverStatus,
        ServerStatus.idle,
      );
      await c.read(onboardingControllerProvider.notifier).changeServer();
      expect(c.read(onboardingControllerProvider).step, OnboardingStep.server);
      expect(
        c.read(onboardingControllerProvider).serverStatus,
        ServerStatus.idle,
      );
      expect(
        c.read(onboardingControllerProvider).address,
        'https://chat.example.com',
      );
      c.dispose();

      final (again, discovery, _) = await start(
        DiscoveryFound(instance),
        backend: secure,
      );
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(discovery.asked, [Uri.parse('https://chat.example.com')]);
      expect(
        again.read(onboardingControllerProvider).step,
        OnboardingStep.login,
      );
      await again.read(onboardingControllerProvider.notifier).changeServer();
      again.dispose();
    });
  });
}

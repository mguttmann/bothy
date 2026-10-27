// SPDX-License-Identifier: Apache-2.0

// The update path of a build and the update notices (docs/spec/18-release.md 8 and 9, 15 15;
// brief M7-13 requirements 14 to 16): which path each platform gets from its build
// configuration, how the launcher opens it, and the actions of E1 "App too old", "Update
// required" and "Update recommended".

import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/update/update_notice.dart';
import 'package:client/app/update/update_path.dart';
import 'package:client/onboarding/e1_server_address.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';

const String address = 'https://apps.example.com/chat';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('UpdatePath.of', () {
    test('iOS, Android and Windows open the address of the build', () {
      for (final platform in ['ios', 'android', 'windows']) {
        final path = UpdatePath.of(platform: platform, updateUrl: address);
        expect(path.url, Uri.parse(address), reason: platform);
        expect(path.inApp, isFalse, reason: platform);
        expect(path.available, isTrue, reason: platform);
      }
    });

    test('macOS checks with Sparkle in the app, the address as fallback', () {
      final path = UpdatePath.of(platform: 'macos', updateUrl: address);
      expect(path.inApp, isTrue);
      expect(path.url, Uri.parse(address));
      expect(UpdatePath.of(platform: 'macos', updateUrl: '').available, isTrue);
    });

    test('a build without an https address has no path', () {
      for (final url in [
        '',
        'http://apps.example.com/x',
        'itms://x',
        'nonsense',
      ]) {
        expect(
          UpdatePath.of(platform: 'ios', updateUrl: url).available,
          isFalse,
          reason: url,
        );
      }
    });
  });

  group('PlatformUpdateLauncher', () {
    const channel = MethodChannel('bothy/updater-test');
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    tearDown(() => messenger.setMockMethodCallHandler(channel, null));

    test(
      'macOS asks Sparkle first and opens nothing else when it answers',
      () async {
        final calls = <String>[];
        messenger.setMockMethodCallHandler(channel, (call) async {
          calls.add(call.method);
          return true;
        });
        final opened = <Uri>[];
        final launcher = PlatformUpdateLauncher(
          channel: channel,
          opener: (url) async {
            opened.add(url);
            return true;
          },
        );
        expect(
          await launcher.open(
            UpdatePath.of(platform: 'macos', updateUrl: address),
          ),
          isTrue,
        );
        expect(calls, ['checkForUpdates']);
        expect(opened, isEmpty);
      },
    );

    test('a build without Sparkle opens the address', () async {
      messenger.setMockMethodCallHandler(channel, (_) async => false);
      final opened = <Uri>[];
      final launcher = PlatformUpdateLauncher(
        channel: channel,
        opener: (url) async {
          opened.add(url);
          return true;
        },
      );
      expect(
        await launcher.open(
          UpdatePath.of(platform: 'macos', updateUrl: address),
        ),
        isTrue,
      );
      expect(opened, [Uri.parse(address)]);
      expect(
        await launcher.open(UpdatePath.of(platform: 'macos', updateUrl: '')),
        isFalse,
      );
    });

    test('elsewhere the address opens; a failing opener is no crash', () async {
      final launcher = PlatformUpdateLauncher(
        channel: channel,
        opener: (_) async => throw const SocketException('no browser'),
      );
      expect(
        await launcher.open(
          UpdatePath.of(platform: 'android', updateUrl: address),
        ),
        isFalse,
      );
    });
  });

  group('actions', () {
    late RecordingUpdateLauncher launcher;
    setUp(() => launcher = RecordingUpdateLauncher());

    Widget scoped(Widget child, {String updateUrl = address}) => ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(
            dataRoot: Directory.systemTemp,
            platform: 'android',
            updateLauncher: launcher,
            branding: BuildBranding(
              appName: 'ExampleChat',
              urlScheme: 'chat-example',
              bundleId: 'com.example.chat',
              defaultServer: 'https://chat.example.com',
              appVersion: '1.0.0',
              updateUrl: updateUrl,
            ),
          ),
        ),
      ],
      child: hostScreen(Scaffold(body: child)),
    );

    testWidgets('"Update required" offers the update path of the build', (
      tester,
    ) async {
      await tester.pumpWidget(scoped(const UpdateRequiredBanner()));
      expect(find.text(en.shellUpdateRequired), findsOneWidget);
      await tester.tap(find.text(en.updateGet));
      await tester.pump();
      expect(launcher.opened, [UpdatePath(url: Uri.parse(address))]);
    });

    testWidgets('without a path "Update required" only explains', (
      tester,
    ) async {
      await tester.pumpWidget(
        scoped(const UpdateRequiredBanner(), updateUrl: ''),
      );
      expect(find.text(en.shellUpdateRequired), findsOneWidget);
      expect(find.text(en.updateGet), findsNothing);
    });

    testWidgets('"Update recommended" opens the path and is dismissed', (
      tester,
    ) async {
      var dismissed = 0;
      var updates = 0;
      await tester.pumpWidget(
        hostScreen(
          Scaffold(
            body: RecommendedUpdateBannerView(
              version: '1.5.0',
              onUpdate: () => updates++,
              onDismiss: () => dismissed++,
            ),
          ),
        ),
      );
      expect(find.text(en.updateRecommended('1.5.0')), findsOneWidget);
      await tester.tap(find.text(en.updateGet));
      await tester.tap(find.byTooltip(en.updateDismiss));
      expect((updates, dismissed), (1, 1));
    });

    testWidgets('E1 "App too old" offers the update', (tester) async {
      var updates = 0;
      desktopWindow(tester);
      await tester.pumpWidget(
        hostScreen(
          ServerAddressScreen(
            address: 'https://chat.example.com',
            status: ServerStatus.appTooOld,
            minVersion: '2.0.0',
            onConnect: (_) {},
            onUpdate: () => updates++,
          ),
        ),
      );
      await tester.tap(find.text(en.updateGet));
      expect(updates, 1);
      // No other status shows the action.
      await tester.pumpWidget(
        hostScreen(
          ServerAddressScreen(
            address: 'https://chat.example.com',
            status: ServerStatus.noInstance,
            onConnect: (_) {},
            onUpdate: () => updates++,
          ),
        ),
      );
      expect(find.text(en.updateGet), findsNothing);
    });
  });
}

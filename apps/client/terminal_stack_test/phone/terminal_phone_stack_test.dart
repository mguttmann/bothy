// SPDX-License-Identifier: Apache-2.0

// The phone terminal against the client stack of the terminal (brief M5-12 Tests, Integration;
// 01 5.3 M8 and M9; 15 23.2 items 6 and 7; 24 6.2; R-PR-12): on an iOS simulator or an Android
// emulator, against the stack of M2-48 with a real server and a real host service in a
// container, Alex opens a shell from M9 with the step-up (the simulated biometric of the test
// keys), runs `echo` and sees the output, ends a running `sleep` with Ctrl+C through the extra
// keys bar; in the background the app shows its privacy cover and detaches (the server lists
// the session as detached); back in front within `terminal.quick_reattach_window` (20s in the
// default profile of the stack) it attaches without a step-up and the replay shows the earlier
// output; after the window the step-up prompt appears, and after it the attach succeeds.
//
// `make terminal-stack` runs it through tools/ci/terminal_stack.py --mode phone with
// TERMINAL_STACK_DEVICE naming the simulator or emulator (a SKIP line without one), on the
// helper mode perf (a paired host assigned to Alex, the control endpoint for enrollment codes).
// It lives outside integration_test/, so the platform jobs without a stack never pick it up.

import 'dart:io';
import 'dart:math';

import 'package:api_client/api_client.dart' as rest show TerminalSessionState;
import 'package:client/platform/terminal_guard.dart';
import 'package:client/src/generated/android_platform_api.g.dart';
import 'package:client/src/generated/ios_platform_api.g.dart';
import 'package:client/terminal/providers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;
import 'package:terminal_ui/terminal_ui.dart';

import '../../e2e_test/support/e2e_app.dart';
import '../../e2e_test/support/e2e_network.dart';
import '../../e2e_test/support/terminal_stack.dart';
import '../../test/support/fakes.dart' show SwitchLifecycle;

/// `terminal.quick_reattach_window` of the stack's default profile
/// (services/server/test/terminal/profiles.yaml); the runner passes it.
const int _quickReattachSeconds = int.fromEnvironment(
  'TERMINAL_STACK_QUICK_REATTACH_SECONDS',
  defaultValue: 20,
);

/// The guard of the platform, recording the secure window, and with the hardware keyboard
/// reported as absent: a simulator reports the keyboard of its Mac, which would hide the bar the
/// test taps.
final class _Guard implements TerminalPlatformGuard {
  _Guard(this._inner);

  final TerminalPlatformGuard _inner;
  final List<bool> secure = [];

  @override
  ValueListenable<bool> get captured => _inner.captured;

  @override
  Listenable get keyboardChanges => _inner.keyboardChanges;

  @override
  Future<void> setSecure(bool on) async {
    secure.add(on);
    await _inner.setSecure(on);
  }

  @override
  Future<bool> connected() async => false;
}

AppLocalizations _l(E2eApp app) =>
    AppLocalizations.of(app.tester.element(find.byType(Navigator).first));

List<String> _lines(TerminalController tab) => [
  for (final l in tab.screen.visibleText().split('\n')) l.trim(),
];

/// Types [text] and return on the soft keyboard of the phone (the private client of M5-12).
Future<void> _type(WidgetTester tester, String text) async {
  await tester.tap(find.byType(TerminalScreenView));
  await tester.pump();
  for (final c in '$text\n'.split('')) {
    final base = tester.testTextInput.editingState!['text']! as String;
    tester.testTextInput.updateEditingValue(
      TextEditingValue(
        text: '$base$c',
        selection: TextSelection.collapsed(offset: base.length + 1),
      ),
    );
    await tester.pump();
  }
}

/// Waits until [done] holds, continuing the step-up dialog if the server asks for one first;
/// returns whether a step-up was asked.
Future<bool> _stepUpThen(E2eApp app, String what, bool Function() done) async {
  final dialog = find.byKey(const ValueKey('stepUp.continue'));
  await app.until(what, () => done() || dialog.evaluate().isNotEmpty);
  if (!done() && dialog.evaluate().isNotEmpty) {
    await app.tapSettled(dialog);
    await app.until(what, done);
    return true;
  }
  return false;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  final stack = TerminalStack.fromEnvironment();
  const hostLoopback = String.fromEnvironment('TERMINAL_STACK_HOST_LOOPBACK');
  late E2eNetwork network;
  setUpAll(() async {
    network = await E2eNetwork.start(
      stack.server,
      stack.caPem,
      upstreamPort: stack.publishedPort,
      upstreamHost: hostLoopback.isEmpty ? null : InternetAddress(hostLoopback),
    );
    network.install();
  });
  tearDownAll(() => network.stop());

  testWidgets(
    'open from M9, Ctrl+C through the bar, background detaches, reattach within and after the window',
    (tester) async {
      final marker = 'm512-${Random().nextInt(1 << 30).toRadixString(16)}';
      final lifecycle = SwitchLifecycle();
      final guard = _Guard(
        Platform.isIOS
            ? IosTerminalGuard(IosPlatformApi())
            : AndroidTerminalGuard(AndroidPlatformApi()),
      );
      final app = await E2eApp.start(
        tester,
        stack,
        platform: Platform.isIOS ? 'ios' : 'android',
        lifecycle: lifecycle,
        terminalGuard: guard,
      );
      try {
        await app.signIn((await tester.runAsync(stack.enrollmentCode))!);
        final hostId = stack.hostId!;

        // M9 under Me, a new shell with the step-up.
        await app.tap('phone.tab.me');
        await app.tapSettled(find.byKey(const ValueKey('phone.me.hosts')));
        await app.tap('phone.hosts.new.$hostId');
        await app.tapSettled(
          find.byKey(ValueKey('phone.hosts.new.$hostId.shell')),
        );
        await app.until(
          'the dialog "Open terminal"',
          () => find.text(_l(app).terminalOpenConfirm).evaluate().isNotEmpty,
        );
        await app.tapSettled(find.text(_l(app).terminalOpenConfirm));
        final tabs = app.container.read(terminalTabsProvider(hostId))!;
        await _stepUpThen(
          app,
          'the shell connected',
          () => tabs.current?.state.phase == TerminalPhase.connected,
        );
        final tab = tabs.current!;
        if (Platform.isAndroid) expect(guard.secure, contains(true));

        await _type(tester, 'echo $marker');
        await app.until('the output', () => _lines(tab).contains(marker));

        // Ctrl+C through the extra keys bar ends a running sleep: the next command runs.
        await _type(tester, 'sleep 600');
        await app.tapSettled(find.text(_l(app).terminalKeyCtrl));
        await _type(tester, 'c');
        await _type(tester, 'echo after-$marker');
        await app.until(
          'the command after Ctrl+C',
          () => _lines(tab).contains('after-$marker'),
        );

        Future<void> background() async {
          tester.binding.handleAppLifecycleStateChanged(
            AppLifecycleState.inactive,
          );
          await tester.pump();
          // The app switcher picture: the privacy cover, the terminal offstage (R-PR-12).
          expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
          expect(find.byType(TerminalScreenView), findsNothing);
          tester.binding
            ..handleAppLifecycleStateChanged(AppLifecycleState.hidden)
            ..handleAppLifecycleStateChanged(AppLifecycleState.paused);
          lifecycle.set(AppActivity.background);
          await app.until(
            'detached in the app',
            () => tab.state.phase == TerminalPhase.detached,
          );
          await app.untilAsync('detached on the server', () async {
            final s = await tabs.services.api.getSession(tab.state.sessionId!);
            return s.state == rest.TerminalSessionState.detached;
          });
        }

        Future<void> foreground() async {
          lifecycle.set(AppActivity.foreground);
          tester.binding
            ..handleAppLifecycleStateChanged(AppLifecycleState.hidden)
            ..handleAppLifecycleStateChanged(AppLifecycleState.inactive)
            ..handleAppLifecycleStateChanged(AppLifecycleState.resumed);
          await tester.pump();
        }

        // Within the window: no step-up, the replay shows the earlier output.
        await background();
        await foreground();
        final askedWithin = await _stepUpThen(
          app,
          'reattached within the window',
          () => tab.state.phase == TerminalPhase.connected,
        );
        expect(askedWithin, isFalse);
        await app.until('the replay', () => _lines(tab).contains(marker));

        // After the window: the step-up prompt, then the attach.
        await background();
        await tester.runAsync(
          () => Future<void>.delayed(
            const Duration(seconds: _quickReattachSeconds + 5),
          ),
        );
        await foreground();
        final askedAfter = await _stepUpThen(
          app,
          'reattached after the window',
          () => tab.state.phase == TerminalPhase.connected,
        );
        expect(askedAfter, isTrue);

        await tab.end();
        await app.until('ended', () => tab.state.phase == TerminalPhase.ended);
      } finally {
        await app.stop();
      }
    },
    timeout: Timeout.none,
  );
}

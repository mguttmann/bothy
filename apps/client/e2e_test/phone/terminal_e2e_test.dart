// SPDX-License-Identifier: Apache-2.0

// The terminal on the phone end to end (brief M5-13 Design note 2; PLAN row M5-13; 01 5.3 M8 and
// M9; 15 23.2 items 6 to 8 and 23.5 row "E2E"; 24 6.1, 6.2, TM-T12 and TM-T16): on the iOS
// simulator or the Android emulator, against the terminal stack of `make e2e-mobile` (the stack of
// M2-48 with the real server, its TLS front and a real host service in a container, which the
// helper paired through the API and assigned to Alex, an owner, with his session user), Alex
// signs in, sees "Build host" online in M9, opens a shell with the step-up (the device key of the
// test and the confirmation that stands for the biometric match), runs `echo`, ends a running
// `sleep` with Ctrl and `c` of the extra keys bar, loses the app's network and reattaches after
// it returns with the scrollback replayed, and moves the app to the background twice: back within
// `terminal.quick_reattach_window` it attaches without a step-up, back after the window the
// step-up prompt comes first. While the app is in the background the app switcher picture shows
// no terminal content: the privacy cover over an offstage terminal (iOS) or the secure window
// (Android `FLAG_SECURE`), docs/decisions.d/M5-13.md D-M5-13-2.
//
// tools/e2e/mobile_e2e.py runs it through tool/mobile_e2e_suite.sh with the define
// TERMINAL_STACK; it lives outside integration_test/, so the platform jobs without a stack never
// pick it up, and without the define it fails at once (P-09: never skipped).

import 'dart:io';
import 'dart:math';

import 'package:api_client/api_client.dart' as rest show TerminalSessionState;
import 'package:client/platform/app_lifecycle.dart';
import 'package:client/platform/network_status.dart';
import 'package:client/platform/terminal_guard.dart';
import 'package:client/src/generated/android_platform_api.g.dart';
import 'package:client/src/generated/ios_platform_api.g.dart';
import 'package:client/terminal/providers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:patrol/patrol.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../support/e2e_app.dart';
import '../support/e2e_network.dart';
import '../support/terminal_stack.dart';
import 'support/phone_run.dart'
    show phoneAutomatorConfig, phonePlatform, untilInBackgroundAsync;

/// The host the helper of the stack paired, as M9 names it (brief M5-13 requirement 8).
const String _hostName = 'Build host';

/// The platform guard of the phone, recording every request for the secure window, with the
/// hardware keyboard reported as absent: a simulator reports the keyboard of its Mac, which
/// would hide the extra keys bar the suite taps.
final class _Guard implements TerminalPlatformGuard {
  _Guard(this._inner);

  final TerminalPlatformGuard _inner;

  /// The requests for the secure window, in order.
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

/// The visible lines of [tab], trimmed.
List<String> _lines(TerminalController tab) => [
  for (final l in tab.screen.visibleText().split('\n')) l.trim(),
];

/// Types [text] and Return on the soft keyboard of the phone (the private input client of
/// M5-12), one character per edit as a keyboard sends them.
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
/// returns whether the step-up was asked.
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

/// Whether Flutter reports the app in the foreground and active.
bool get _resumed =>
    WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;

void main() {
  patrolTest(
    'M9 to M8 with the step-up, Ctrl+C through the bar, network loss and reattach, the app '
    'switcher without terminal content, reattach within and after the quick window',
    platformAutomatorConfig: phoneAutomatorConfig,
    ($) async {
      final stack = TerminalStack.fromEnvironment();
      final windows =
          stack.windows ??
          (throw StateError(
            'the connection data name no terminal windows; run make e2e-mobile',
          ));
      final network = await E2eNetwork.start(
        stack.server,
        stack.caPem,
        upstreamPort: stack.publishedPort,
      );
      // Only the app's connections to the server go through the tunnel: patrol's own
      // connection to its automation server must survive the network loss of the suite.
      network.installForServer();
      final lifecycle = FlutterAppLifecycle();
      final status = LifecycleNetworkStatus(lifecycle);
      final guard = _Guard(
        Platform.isIOS
            ? IosTerminalGuard(IosPlatformApi())
            : AndroidTerminalGuard(AndroidPlatformApi()),
      );
      E2eApp? started;
      try {
        final app = started = await E2eApp.start(
          $.tester,
          stack,
          platform: phonePlatform,
          deviceView: true,
          deviceName: 'Alex phone',
          lifecycle: lifecycle,
          network: status,
          terminalGuard: guard,
        );
        await _flow($, app, stack, windows, network, guard);
      } finally {
        // A failure while the network was cut or the app was in the background leaves the app
        // nothing to end through and no frames to draw.
        network.restore();
        if (!_resumed) {
          await $.platform.mobile.openApp();
          await untilInBackgroundAsync(
            'the app in the foreground',
            () async => _resumed,
          );
        }
        await started?.stop();
        await status.dispose();
        await lifecycle.dispose();
        await network.stop();
        HttpOverrides.global = null;
      }
    },
  );
}

Future<void> _flow(
  PatrolIntegrationTester $,
  E2eApp app,
  TerminalStack stack,
  TerminalWindows windows,
  E2eNetwork network,
  _Guard guard,
) async {
  final tester = app.tester;
  final hostId = stack.hostId!;
  final marker = 'm513-${Random().nextInt(1 << 30).toRadixString(16)}';

  // E1 and E3 with a fresh code of Alex from the stack's control endpoint.
  await app.connect();
  await app.enroll((await tester.runAsync(stack.enrollmentCode))!);
  await app.waitFor(find.byKey(const ValueKey('phone.shell')));
  await app.waitLive();

  // M9 under Me: "Build host", online.
  await app.tap('phone.tab.me');
  await app.tapSettled(find.byKey(const ValueKey('phone.me.hosts')));
  final row = find.byKey(ValueKey('phone.hosts.$hostId'));
  // The name is shown isolated (dsIsolate), so it is found as a part of the text.
  Iterable<String> rowTexts() => [
    for (final e
        in find.descendant(of: row, matching: find.byType(Text)).evaluate())
      (e.widget as Text).data ?? '',
  ];
  await app.until(
    '"$_hostName" online in M9',
    () =>
        rowTexts().any((t) => t.contains(_hostName)) &&
        rowTexts().contains(_l(app).hostConnected),
    detail: () => rowTexts().join(' | '),
  );

  // A new shell, the dialog "Open terminal" and the step-up.
  await app.tap('phone.hosts.new.$hostId');
  await app.tapSettled(find.byKey(ValueKey('phone.hosts.new.$hostId.shell')));
  await app.until(
    'the dialog "Open terminal"',
    () => find.text(_l(app).terminalOpenConfirm).evaluate().isNotEmpty,
  );
  await app.tapSettled(find.text(_l(app).terminalOpenConfirm));
  final tabs = app.container.read(terminalTabsProvider(hostId))!;
  final opened = await _stepUpThen(
    app,
    'the shell connected',
    () => tabs.current?.state.phase == TerminalPhase.connected,
  );
  expect(opened, isTrue, reason: 'opening a terminal asks for the step-up');
  final tab = tabs.current!;
  final sessionId = tab.state.sessionId!;
  if (Platform.isAndroid) {
    expect(guard.secure.last, isTrue, reason: 'M8 keeps the window secure');
  }

  await _type(tester, 'echo $marker');
  await app.until('the output of echo', () => _lines(tab).contains(marker));

  // Ctrl+C through the extra keys bar ends a running sleep: the next command runs.
  await _type(tester, 'sleep 600');
  await app.tapSettled(find.text(_l(app).terminalKeyCtrl));
  await _type(tester, 'c');
  await _type(tester, 'echo after-$marker');
  await app.until(
    'the prompt back after Ctrl+C',
    () => _lines(tab).contains('after-$marker'),
  );

  // Network loss: the stream is lost, the tab shows "detached", the server keeps the session
  // for terminal.detach_grace; the network returns well within it, and Reattach replays the
  // scrollback into a fresh screen.
  final lost = Stopwatch()..start();
  network.cut();
  await app.until(
    'detached after the network loss',
    () =>
        tab.state.phase == TerminalPhase.detached &&
        tab.state.detachCause == DetachCause.connectionLost,
    detail: () => '${tab.state}',
  );
  await app.waitFor(find.text(_l(app).terminalReattach));
  network.restore();
  expect(
    lost.elapsed,
    lessThan(windows.detachGrace),
    reason: 'the network returns within terminal.detach_grace',
  );
  final before = tab.screen.terminal;
  await app.tapFinder(find.text(_l(app).terminalReattach));
  await _stepUpThen(
    app,
    'reattached with the scrollback after the network loss',
    () =>
        tab.state.phase == TerminalPhase.connected &&
        !tab.state.replaying &&
        !identical(tab.screen.terminal, before) &&
        _lines(tab).contains(marker) &&
        _lines(tab).contains('after-$marker'),
  );
  expect(tab.state.sessionId, sessionId, reason: 'the same session');

  /// Moves the app to the background with the home button, checks the app switcher picture
  /// and waits until the session is detached in the app and on the server; returns the
  /// stopwatch started when the server listed it detached.
  Future<Stopwatch> background() async {
    final secureBefore = guard.secure.length;
    await $.platform.mobile.pressHome();
    if (Platform.isIOS) {
      // iOS takes the app switcher picture right after the app resigned active: the cover is
      // in that frame and the terminal is offstage, so it is neither painted nor read.
      await untilInBackgroundAsync(
        'the privacy cover in the background',
        () async =>
            find.byKey(const ValueKey('privacy.cover')).evaluate().isNotEmpty,
      );
      expect(find.byType(TerminalScreenView), findsNothing);
    } else {
      // Android takes the picture of a secure window blank; the window stays secure while M8
      // shows, also in the background (RecentsProtection, D-M5-12-6).
      expect(guard.secure.last, isTrue);
      expect(
        guard.secure.skip(secureBefore),
        isNot(contains(false)),
        reason: 'nothing lifted FLAG_SECURE on the way to the background',
      );
    }
    await untilInBackgroundAsync(
      'detached in the app',
      () async => tab.state.phase == TerminalPhase.detached,
      detail: () => '${tab.state}',
    );
    if (Platform.isAndroid) {
      // The pause has reached the app now: the window is still secure.
      expect(guard.secure.skip(secureBefore), isNot(contains(false)));
    }
    await untilInBackgroundAsync('detached on the server', () async {
      final s = await tabs.services.api.getSession(sessionId);
      return s.state == rest.TerminalSessionState.detached;
    });
    return Stopwatch()..start();
  }

  Future<void> foreground() async {
    await $.platform.mobile.openApp();
    await untilInBackgroundAsync(
      'the app in the foreground',
      () async => _resumed,
    );
    await tester.pump();
  }

  // Back within terminal.quick_reattach_window: the attach needs no step-up and replays. The
  // stack's profile makes the window at least the patience of every wait here (profile mobile,
  // services/server/test/terminal/profiles.yaml), so the return is within it on any machine.
  final within = await background();
  await foreground();
  // The attach starts on the return to the foreground; the replay after it may take its time.
  expect(
    within.elapsed,
    lessThan(windows.quickReattach),
    reason: 'the app returned within terminal.quick_reattach_window',
  );
  final askedWithin = await _stepUpThen(
    app,
    'reattached within the window',
    () => tab.state.phase == TerminalPhase.connected && !tab.state.replaying,
  );
  expect(askedWithin, isFalse, reason: 'no step-up within the window');
  await app.until('the replay', () => _lines(tab).contains(marker));

  // Back after the window: the server asks for the step-up first, then the attach succeeds.
  final after = await background();
  // The server lists the session detached from the moment of the detach on, which is before
  // the stopwatch started; waiting the window from here puts the attach after it.
  await untilInBackgroundAsync(
    'terminal.quick_reattach_window over',
    () async => after.elapsed > windows.quickReattach,
  );
  await foreground();
  final askedAfter = await _stepUpThen(
    app,
    'reattached after the window',
    () => tab.state.phase == TerminalPhase.connected && !tab.state.replaying,
  );
  expect(askedAfter, isTrue, reason: 'the step-up after the window');
  await app.until('the replay', () => _lines(tab).contains(marker));

  await tab.end();
  await app.until('ended', () => tab.state.phase == TerminalPhase.ended);
}

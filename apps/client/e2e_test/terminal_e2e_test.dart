// SPDX-License-Identifier: Apache-2.0

// The terminal of the desktop app end to end (brief M4-38 Design note 1; docs/spec/
// 15-client-architecture.md 20 and 23.5 row "E2E"; 24 3.2, 6.1, 6.2): against the client stack of
// the terminal (the stack of M2-48 with its TLS front, the real server and a real host service in
// a container), Alex, an owner, pairs the host through the administration of M4-37 (the code of
// the wizard goes to the pairing step of the stack, which runs `bothy-host pair` in the host
// container; Alex types the start of the printed fingerprint, answers organization and "no" for
// the server host and assigns his session user), opens a shell from the group "Hosts" with the
// step-up, runs a command and sees its output, detaches (the session stays detached), reattaches
// and sees the scrollback replayed, runs `exit 3` and sees the exit code, and ends the tab. At the
// end the canary he typed is in none of the app's paths (database rows and files of the data
// folder, print and debugPrint, the client metrics) and in none of the stack's logs (R-CL-17).
//
// `make e2e-desktop` runs it through tools/ci/terminal_stack.py and tool/e2e_suite.sh terminal;
// it is not part of integration_test/, so the platform jobs without a stack never pick it up.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:admin_ui/admin_ui.dart' show fingerprintConfirmCharsDefault;
import 'package:client/terminal/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/e2e_app.dart';
import 'support/e2e_network.dart';
import 'support/terminal_stack.dart';

/// The texts of the app in the locale it runs in (the system's: a German desktop shows German).
AppLocalizations _l(E2eApp app) =>
    AppLocalizations.of(app.tester.element(find.byType(Navigator).first));

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  final stack = TerminalStack.fromEnvironment();
  late E2eNetwork network;
  setUpAll(() async {
    network = await E2eNetwork.start(
      stack.server,
      stack.caPem,
      upstreamPort: stack.publishedPort,
    );
    network.install();
  });
  tearDownAll(() => network.stop());

  testWidgets(
    'pair a host, open a terminal, run a command, detach, reattach, end',
    (tester) async {
      final canary = stack.canary!;
      final marker = 'm438-${Random().nextInt(1 << 30).toRadixString(16)}';
      // Everything printed is kept for the canary check and still shown, so a failure keeps
      // its report.
      final logs = <String>[];
      final previous = debugPrint;
      debugPrint = (message, {wrapWidth}) {
        logs.add(message ?? '');
        previous(message, wrapWidth: wrapWidth);
      };
      addTearDown(() => debugPrint = previous);
      await runZoned(
        () => _flow(tester, stack, marker, canary),
        zoneSpecification: ZoneSpecification(
          print: (self, parent, zone, line) {
            logs.add(line);
            parent.print(zone, line);
          },
        ),
      );
      expect(logs.where((l) => l.contains(canary)), isEmpty);
      // The stack: container logs, database dump, collector export, host journal.
      final findings = await stack.canaryFindings();
      expect(findings, isEmpty, reason: 'the canary reached the stack');
    },
    timeout: Timeout.none,
  );
}

Future<void> _flow(
  WidgetTester tester,
  TerminalStack stack,
  String marker,
  String canary,
) async {
  final app = await E2eApp.start(tester, stack);
  try {
    await app.signIn((await tester.runAsync(stack.enrollmentCode))!);
    final hostId = await _pairHost(app, stack, 'Build host $marker');

    // The group "Hosts", the host, a new shell with the step-up.
    await app.tap('hosts.row.$hostId');
    final tabs = app.container.read(terminalTabsProvider(hostId))!;
    await app.tap('terminal.new');
    await app.tapSettled(find.byKey(const ValueKey('terminal.new.shell')));
    await app.until(
      'the dialog "Open terminal"',
      () => find.text(_l(app).terminalOpenConfirm).evaluate().isNotEmpty,
      detail: () => '${tabs.current?.state}',
    );
    await app.tapSettled(find.text(_l(app).terminalOpenConfirm));
    await _stepUpThen(
      app,
      'the shell connected',
      () => tabs.current?.state.phase == TerminalPhase.connected,
    );
    final tab = tabs.current!;

    await _type(tester, 'echo $marker');
    await app.until(
      'the output of the command',
      () => _lines(tab).contains(marker),
      detail: () => _lines(tab).join(' | '),
    );
    await _type(tester, 'echo $canary');
    await app.until('the canary on screen', () => _lines(tab).contains(canary));

    // Detach: the tab and the server's session are detached, the session keeps running.
    await app.tapFinder(find.text(_l(app).terminalDetach));
    await app.until(
      'detached',
      () => tab.state.phase == TerminalPhase.detached,
    );
    final sessionId = tab.state.sessionId!;
    final api = app.state.services!.auth.api.getTerminalApi();
    await app.untilAsync('the server keeps the session detached', () async {
      final s = await api.getTerminalSession(sessionId: sessionId);
      return s.data?.state.name == 'detached';
    });

    // Reattach: the scrollback of the host replaces the screen (a fresh terminal model) and
    // still shows the output.
    final before = tab.screen.terminal;
    await app.tapFinder(find.text(_l(app).terminalReattach));
    await _stepUpThen(
      app,
      'reattached with the scrollback',
      () =>
          tab.state.phase == TerminalPhase.connected &&
          !tab.state.replaying &&
          !identical(tab.screen.terminal, before) &&
          _lines(tab).contains(marker),
    );

    // The shell ends with a code of its own; the tab shows it, End closes the tab's session.
    await _type(tester, 'exit 3');
    await app.until(
      'the exit code',
      () => tab.state.phase == TerminalPhase.exited,
      detail: () => '${tab.state}',
    );
    expect(tab.state.exitCode, 3);
    await app.waitFor(find.textContaining(_l(app).terminalStateExited(3)));
    await app.tapFinder(find.text(_l(app).terminalEnd));
    await app.until('ended', () => tab.state.phase == TerminalPhase.ended);

    await _appPathsWithoutCanary(app, canary);
  } finally {
    await app.stop();
  }
}

/// Administration, Hosts: creates the host with Alex as assignee, hands the code of the wizard to
/// the pairing step of the stack, types the start of the printed fingerprint, answers the
/// questions, assigns Alex's session user and confirms. Returns the host ID.
Future<String> _pairHost(E2eApp app, TerminalStack stack, String name) async {
  final tester = app.tester;
  GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
      .go('/admin/hosts');
  await app.tap('hosts.add');
  await _enter(app, 'createHost.name', name);
  await _pick(app, 'createHost.assignee', 'Alex');
  await _enter(app, 'createHost.reason', 'Terminal of the E2E run');
  await app.tap('pairing.create');
  await _stepUpThen(
    app,
    'the pairing code',
    () => find.byKey(const ValueKey('pairing.command')).evaluate().isNotEmpty,
  );
  final command = tester
      .widget<SelectableText>(find.byKey(const ValueKey('pairing.command')))
      .data!;
  final code = RegExp(r'--code (\S+)').firstMatch(command)!.group(1)!;

  final paired = (await tester.runAsync(() => stack.pair(code)))!;
  expect(paired.exitCode, 0, reason: 'bothy-host pair in the host container');
  final prefix = paired.fingerprint
      .replaceAll('-', '')
      .substring(0, fingerprintConfirmCharsDefault);

  await app.tap('pairing.next');
  await _enter(app, 'fingerprint.prefix', prefix);
  await app.tap('llmChoice.organization');
  await app.tap('serverHost.no');
  await app.tap('sessionRows.add');
  await _pick(app, 'sessionRow.0.person', 'Alex');
  // The wizard offers only the names the host reported (SR12); the report follows the pairing.
  await _pick(app, 'sessionRow.0.name', stack.sessionUser);
  await _enter(app, 'pairing.reason', 'Terminal of the E2E run');
  await app.tap('pairing.confirm');
  await _stepUpThen(
    app,
    'the confirmed host with the session user of Alex',
    () => find
        .byKey(ValueKey('sessionUser.${stack.alexId}'))
        .evaluate()
        .isNotEmpty,
  );

  // The host page of the administration names the host; back in the shell, the group "Hosts"
  // of the sidebar lists it.
  final page = find.byWidgetPredicate(
    (w) =>
        w.key is ValueKey<String> &&
        (w.key! as ValueKey<String>).value.startsWith('admin.host.'),
  );
  await app.waitFor(page);
  final hostId = (page.evaluate().first.widget.key! as ValueKey<String>).value
      .substring('admin.host.'.length);
  await app.tap('admin.back');
  await app.waitFor(find.byKey(ValueKey('hosts.row.$hostId')));
  return hostId;
}

/// Types [text] into the field with [key].
Future<void> _enter(E2eApp app, String key, String text) async {
  await app.waitFor(find.byKey(ValueKey(key)));
  await app.tester.enterText(find.byKey(ValueKey(key)).first, text);
  await app.tester.pump();
}

/// Opens the menu with [key] and chooses [label]. While the menu does not offer it yet (the names
/// of the host's report), the menu closes and opens again, at most for [patience].
Future<void> _pick(E2eApp app, String key, String label) async {
  final tester = app.tester;
  final watch = Stopwatch()..start();
  // The label in the open menu: one more than the field may already show.
  final shownBefore = find.text(label).evaluate().length;
  bool offered() => find.text(label).evaluate().length > shownBefore;
  for (;;) {
    await app.tap(key);
    await app.untilOr(offered, const Duration(seconds: 2));
    if (offered()) {
      await app.tapSettled(find.text(label).last);
      return;
    }
    if (watch.elapsed >= patience) fail('$key never offered "$label"');
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pump();
  }
}

/// Waits until [done] holds, continuing the step-up dialog if the server asks for one first.
Future<void> _stepUpThen(E2eApp app, String what, bool Function() done) async {
  final dialog = find.byKey(const ValueKey('stepUp.continue'));
  await app.until(what, () => done() || dialog.evaluate().isNotEmpty);
  if (!done() && dialog.evaluate().isNotEmpty) {
    await app.tapSettled(dialog);
    await app.until(what, done);
  }
}

/// Types [text] and Enter into the focused terminal like a person: through the platform text
/// input where the screen has one (macOS), else key by key (Windows reads the hardware keyboard
/// only, D-M4-36-10). The texts of the flow use letters, digits, spaces and hyphens only.
Future<void> _type(WidgetTester tester, String text) async {
  await tester.tap(find.byType(TerminalScreenView));
  await tester.pump();
  if (tester.testTextInput.hasAnyClients) {
    // xterm takes what was added to its empty editing value.
    tester.testTextInput.enterText(text);
  } else {
    for (final c in text.split('')) {
      final key =
          LogicalKeyboardKey.findKeyByKeyId(c.codeUnitAt(0)) ??
          (throw ArgumentError.value(c, 'text', 'no key'));
      await tester.sendKeyEvent(key, character: c);
    }
  }
  await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  await tester.pump();
}

/// The visible lines of [tab], trimmed.
List<String> _lines(TerminalController tab) => [
  for (final l in tab.screen.visibleText().split('\n')) l.trim(),
];

/// The canary check of M4-36 on the app's paths: every decrypted row of every table, every file
/// of the data folder (the database file included) and the client metrics still held.
Future<void> _appPathsWithoutCanary(E2eApp app, String canary) async {
  final db = app.state.services!.database;
  await app.tester.runAsync(() async {
    final tables = await db
        .customSelect("SELECT name FROM sqlite_master WHERE type = 'table'")
        .get();
    for (final t in tables) {
      final name = t.read<String>('name');
      for (final row in await db.customSelect('SELECT * FROM "$name"').get()) {
        for (final value in row.data.values) {
          expect('$value', isNot(contains(canary)), reason: name);
        }
      }
    }
    for (final f
        in app.device.root.listSync(recursive: true).whereType<File>()) {
      expect(
        latin1.decode(await f.readAsBytes(), allowInvalid: true),
        isNot(contains(canary)),
        reason: f.path,
      );
    }
  });
  final metrics = app.device.ports.measurements.metrics.take();
  expect(
    jsonEncode([for (final m in metrics) m.toJson()]),
    isNot(contains(canary)),
  );
}

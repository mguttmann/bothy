// SPDX-License-Identifier: Apache-2.0

/// Support of the `patrol` web flows of the console administration (brief M6-04 Design note 4):
/// the seed of `tools/console-e2e` (`window.__e2eSeed`), the start of the console in the
/// browser that already holds the owner's session cookie, and waits that confirm every
/// step-up of an action with the passkey of the virtual authenticator.
///
/// The flows run in the bundle `admin_bundle.dart`, served by the server under its enforced CSP
/// (`make console-e2e`, phase 2). They share one server, so they run in the order of the bundle.
library;

import 'dart:convert';
import 'dart:js_interop';

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

@JS('__e2eSeed')
external JSString? get _seedJson;

/// One seeded object: its ID and its name.
final class Seeded {
  /// Creates the entry.
  const Seeded(this.id, this.name);

  /// The ID of the server.
  final String id;

  /// The name or display name.
  final String name;
}

/// The data the runner seeded through the API before the flows (tools/console-e2e
/// `admin_suites.py`).
final class E2eSeed {
  E2eSeed._(this._json);

  /// Reads the seed the runner put on the page.
  ///
  /// Throws a [StateError] when the page has none (the bundle runs only inside
  /// `make console-e2e`).
  factory E2eSeed.read() {
    final raw = _seedJson?.toDart;
    if (raw == null) throw StateError('window.__e2eSeed is not set');
    return E2eSeed._(jsonDecode(raw) as Map<String, Object?>);
  }

  final Map<String, Object?> _json;

  Seeded _entry(Object? v) {
    final m = v! as Map<String, Object?>;
    return Seeded(m['id']! as String, m['name']! as String);
  }

  Seeded _in(String group, String key) =>
      _entry((_json[group]! as Map<String, Object?>)[key]);

  /// Alex, the owner the console is signed in as.
  Seeded get owner => _entry(_json['owner']);

  /// Sam.
  Seeded get sam => _entry(_json['sam']);

  /// Sam's waiting device.
  Seeded get pendingDevice => _entry(_json['pendingDevice']);

  /// The verification code the waiting device shows (08 5.2).
  String get pendingCode =>
      (_json['pendingDevice']! as Map<String, Object?>)['code']! as String;

  /// A space by its key (`support`, `marketing`, `engineering`).
  Seeded space(String key) => _in('spaces', key);

  /// A channel by its key (`marketingCampaigns`, ...).
  Seeded channel(String key) => _in('channels', key);

  /// An agent by its key (`supportAgent`, `texter`).
  Seeded agent(String key) => _in('agents', key);
}

/// The number of step-ups the waits confirmed in the current flow.
int stepUps = 0;

/// The key finder of [key].
Finder byKey(String key) => find.byKey(ValueKey(key));

const _passkey = ValueKey('consoleStepUp.passkey');

/// Starts the console at [path] in English and waits for the signed-in shell. The runner logged
/// the owner in and confirmed the console step-up before the flows, so the cookie of this
/// origin already carries a stepped-up session.
Future<ConsoleController> openConsole(
  PatrolIntegrationTester $,
  String path,
) async {
  stepUps = 0;
  await $.tester.pumpWidget(
    ConsoleApp(
      environment: ConsoleEnvironment.browser(),
      initialLocation: Uri(path: path),
      locale: const Locale('en'),
    ),
  );
  await waitFor($, find.textContaining('Signed in: '));
  return controllerOf($);
}

/// The controller of the console on screen.
ConsoleController controllerOf(PatrolIntegrationTester $) =>
    ConsoleApp.controllerOf($.tester.element(find.byType(Navigator).first));

/// Pumps until [finder] finds something, confirming every step-up dialog of an action on the
/// way with the passkey (the virtual authenticator answers with user verification).
///
/// Throws a [TestFailure] after [timeout].
Future<void> waitFor(
  PatrolIntegrationTester $,
  Finder finder, {
  Duration timeout = const Duration(seconds: 60),
}) async {
  final end = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(end)) {
    await $.tester.pump(const Duration(milliseconds: 200));
    if (finder.evaluate().isNotEmpty) return;
    final passkey = find.byKey(_passkey);
    if (passkey.evaluate().isNotEmpty) {
      stepUps++;
      await $.tester.tap(passkey);
    }
  }
  throw _failure('timed out waiting for $finder; ${screenText()}');
}

/// A failure whose message also goes to the browser console, where the runner prints the
/// lines that start with PATROL_LOG (patrol's own report of a web failure has no details).
TestFailure _failure(String message) {
  // ignore: avoid_print
  print('PATROL_LOG ${jsonEncode({'type': 'e2e', 'message': message})}');
  return TestFailure(message);
}

/// The texts on screen, for the message of a failed wait (the runner prints the details of a
/// failed test).
String screenText() {
  final texts = <String>[
    for (final e in find.byType(Text).evaluate())
      if ((e.widget as Text).data case final t? when t.trim().isNotEmpty) t,
  ];
  return 'screen: ${texts.take(80).join(' | ')}';
}

/// Pumps until [finder] finds nothing any more (step-ups are confirmed on the way).
Future<void> waitGone(
  PatrolIntegrationTester $,
  Finder finder, {
  Duration timeout = const Duration(seconds: 60),
}) async {
  final end = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(end)) {
    await $.tester.pump(const Duration(milliseconds: 200));
    if (finder.evaluate().isEmpty) return;
    final passkey = find.byKey(_passkey);
    if (passkey.evaluate().isNotEmpty) {
      stepUps++;
      await $.tester.tap(passkey);
    }
  }
  throw _failure('timed out waiting until $finder is gone; ${screenText()}');
}

/// Whether the widget of [e] takes a tap: a button with an `onPressed` handler (a list that is
/// still loading shows its buttons disabled).
bool _enabled(Element e) => switch (e.widget) {
  final ButtonStyleButton b => b.onPressed != null,
  final IconButton b => b.onPressed != null,
  final ListTile t => t.enabled,
  _ => true,
};

/// Waits for [finder] to find an enabled widget, scrolls it into view and taps its last match
/// (the open menu item of a dropdown is the last one).
Future<void> tapOn(PatrolIntegrationTester $, Finder finder) async {
  final end = DateTime.now().add(const Duration(seconds: 60));
  await waitFor($, finder);
  while (!_enabled(finder.evaluate().last)) {
    if (!DateTime.now().isBefore(end)) {
      throw _failure('$finder stays disabled; ${screenText()}');
    }
    await waitFor($, finder);
  }
  await $.tester.ensureVisible(finder.last);
  await $.tester.pump();
  await $.tester.tap(finder.last);
  await $.tester.pump(const Duration(milliseconds: 300));
}

/// Taps the widget with [key].
Future<void> tapKey(PatrolIntegrationTester $, String key) =>
    tapOn($, byKey(key));

/// Types [text] into the field with [key].
Future<void> typeKey(PatrolIntegrationTester $, String key, String text) async {
  final f = byKey(key);
  await waitFor($, f);
  await $.tester.ensureVisible(f);
  await $.tester.enterText(f, text);
  await $.tester.pump(const Duration(milliseconds: 300));
}

/// Opens the dropdown with [key] and picks its item [item].
Future<void> pick(PatrolIntegrationTester $, String key, String item) async {
  await tapKey($, key);
  await tapKey($, item);
}

/// Fails unless the flow confirmed at least one step-up through the console's adapter.
void expectStepUp() =>
    expect(stepUps, greaterThan(0), reason: 'no step-up was asked');

/// A day as `YYYY-MM-DD`, [days] after today.
String dayFromToday(int days) {
  final d = DateTime.now().add(Duration(days: days));
  String two(int v) => v.toString().padLeft(2, '0');
  return '${d.year}-${two(d.month)}-${two(d.day)}';
}

/// Opens the grants of the console at the target [key] (`space.<id>` or `channel.<id>`).
Future<void> grantsAt(PatrolIntegrationTester $, String key) async {
  await openConsole($, '/groups/grants');
  await pick($, 'grants.target', 'grants.target.$key');
  await waitFor($, byKey('grants.create'));
}

/// Opens the grant wizard on the current target for [person] and goes to step 3.
Future<void> wizardFor(PatrolIntegrationTester $, Seeded person) async {
  await tapKey($, 'grants.create');
  await waitFor($, byKey('wizard'));
  await tapKey($, 'subject.${person.id}');
  await tapKey($, 'wizard.next');
  await tapKey($, 'wizard.next');
  await waitFor($, byKey('wizard.reason'));
}

// SPDX-License-Identifier: Apache-2.0

/// Support of the `patrol` web flows of the console administration (brief M6-04 Design note 4):
/// the seed of `tools/console-e2e` (`window.__e2eSeed`), the start of the console in the
/// browser that already holds the owner's session cookie, and waits that confirm every
/// step-up of an action with the passkey of the virtual authenticator.
///
/// The flows run in the bundle `admin_bundle.dart`, served by the server under its enforced CSP
/// (`make console-e2e`, phase 2). They share one server, so they run in the order of the bundle.
///
/// The flows of apps and webhooks (brief M6-08) also ask the runner for a manifest file
/// ([chooseManifest]), call the API from the page with the console session or without any
/// credential ([consoleApi], [postWithoutSession]), and check that a secret shown once leaves no
/// trace on screen ([expectNoTrace]); a secret never reaches a log or a failure message
/// ([hideInFailures]).
library;

import 'dart:convert';
import 'dart:js_interop';
import 'dart:math';

import 'package:console/console.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';
import 'package:web/web.dart' as web;

@JS('__e2eSeed')
external JSString? get _seedJson;

/// patrol's binding to the runner (tools/console-e2e PlatformHandler); the same binding patrol's
/// own web actions use.
@JS('__patrol__platformHandler')
external JSPromise<JSAny?> _platformHandler(JSAny? request);

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
  // The live binding of patrol leaves text input to the browser, where a test cannot type; the
  // test text input takes the framework's text input channel, so enterText reaches the fields.
  if (!$.tester.testTextInput.isRegistered) $.tester.testTextInput.register();
  await $.tester.pumpWidget(
    ConsoleApp(
      // A new app each time: the same widget type would keep the state of the last one.
      key: UniqueKey(),
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
/// failed test). A value given to [hideInFailures] is replaced by `[hidden]`.
String screenText() {
  final texts = <String>[
    for (final e in find.byType(Text).evaluate())
      if ((e.widget as Text).data case final t? when t.trim().isNotEmpty) t,
  ];
  return _hide('screen: ${texts.take(80).join(' | ')}');
}

/// The secrets of the running flow that a failure message must not show.
final Set<String> _hidden = {};

/// Keeps [secret] out of every failure message of this file (`screenText`): the runner prints
/// the details of a failed test and keeps the console of the page as a CI artifact, and a token
/// shown once must never reach either (brief M6-08 requirement 17). [forgetHidden] drops them.
void hideInFailures(String secret) {
  if (secret.isNotEmpty) _hidden.add(secret);
}

/// Forgets the secrets of [hideInFailures]; a flow calls it at its end.
void forgetHidden() => _hidden.clear();

String _hide(String text) {
  var out = text;
  for (final s in _hidden) {
    out = out.replaceAll(s, '[hidden]');
  }
  return out;
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
  await _settleStepUp($);
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
  await _settleStepUp($);
  await $.tester.ensureVisible(f);
  await $.tester.enterText(f, text);
  await $.tester.pump(const Duration(milliseconds: 300));
}

/// Opens the dropdown with [key] and picks its item [item].
Future<void> pick(PatrolIntegrationTester $, String key, String item) async {
  await tapKey($, key);
  await tapKey($, item);
}

/// Confirms a step-up dialog that is open or opens within a moment, and waits until it is gone,
/// so the next tap or key never lands on its barrier (which would cancel it).
Future<void> _settleStepUp(PatrolIntegrationTester $) async {
  for (var i = 0; i < 3; i++) {
    await $.tester.pump(const Duration(milliseconds: 100));
  }
  final dialog = find.byKey(const ValueKey('consoleStepUp.dialog'));
  if (dialog.evaluate().isEmpty) return;
  final passkey = find.byKey(_passkey);
  if (passkey.evaluate().isNotEmpty) {
    stepUps++;
    await $.tester.tap(passkey);
  }
  await waitGone($, dialog);
}

/// Fails unless the flow confirmed at least one step-up through the console's adapter.
void expectStepUp() =>
    expect(stepUps, greaterThan(0), reason: 'no step-up was asked');

/// Asks the runner for the platform action [action] with [params] through patrol's binding and
/// returns its answer. The runner fails a test that asks for an action it does not know.
Future<Object?> askRunner(String action, Map<String, Object?> params) async {
  final answer = await _platformHandler(
    {'action': action, 'params': params}.jsify(),
  ).toDart;
  return answer.dartify();
}

/// Taps "Choose file" ([pickKey]), asks the runner for its manifest file [name] (platform action
/// `e2eSetManifestFile`: name, MIME type and text) and sets it on the file input the console
/// appended (`data-bothy-file-input="manifest"`), then fires the input's `change` event. Without a
/// user activation Chromium opens no dialog, and the runner cannot touch the page while the
/// binding call is pending, so the page sets the file itself.
Future<void> chooseManifest(
  PatrolIntegrationTester $,
  String name, {
  String pickKey = 'install.pick',
}) async {
  await tapKey($, pickKey);
  final answer = await askRunner('e2eSetManifestFile', {'name': name});
  if (answer is! Map) throw _failure('no manifest file $name from the runner');
  final input = await _manifestInput($);
  final file = web.File(
    [(answer['text'] as String).toJS].toJS,
    answer['name'] as String,
    web.FilePropertyBag(type: answer['mimeType'] as String),
  );
  final transfer = web.DataTransfer()..items.add(file);
  input
    ..files = transfer.files
    ..dispatchEvent(web.Event('change'));
  await $.tester.pump(const Duration(milliseconds: 300));
}

/// The manifest input the console appends on "Choose file", once it is in the page.
Future<web.HTMLInputElement> _manifestInput(
  PatrolIntegrationTester $, {
  Duration timeout = const Duration(seconds: 60),
}) async {
  const selector = 'input[data-bothy-file-input="manifest"]';
  final end = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(end)) {
    final input = web.document.querySelector(selector);
    if (input != null) return input as web.HTMLInputElement;
    await $.tester.pump(const Duration(milliseconds: 200));
  }
  throw _failure('timed out waiting for $selector; ${screenText()}');
}

/// The state of the checkbox with [key]: its value and whether it takes a tap. The key may sit
/// on a Checkbox, a CheckboxListTile or a widget around one of them.
({bool? value, bool enabled}) checkboxOf(String key) {
  final widget = byKey(key).evaluate().single.widget;
  if (widget is CheckboxListTile) {
    return (value: widget.value, enabled: widget.onChanged != null);
  }
  if (widget is Checkbox) {
    return (value: widget.value, enabled: widget.onChanged != null);
  }
  final inner = find.descendant(
    of: byKey(key),
    matching: find.byType(Checkbox),
  );
  final box = inner.evaluate().single.widget as Checkbox;
  return (value: box.value, enabled: box.onChanged != null);
}

/// Taps the checkbox with [key] until its value is [value].
Future<void> setCheckbox(
  PatrolIntegrationTester $,
  String key,
  bool value,
) async {
  await waitFor($, byKey(key));
  if (checkboxOf(key).value != value) await tapKey($, key);
  expect(checkboxOf(key).value, value, reason: '$key did not change');
}

/// The text of the SelectableText with [key] (the secret of `OnceOnlySecret`). The caller keeps
/// it in a local variable only and passes it to [hideInFailures] at once.
String selectableTextOf(String key) {
  final widget = byKey(key).evaluate().single.widget;
  final text = widget is SelectableText
      ? widget.data ?? widget.textSpan?.toPlainText() ?? ''
      : '';
  if (text.isEmpty) throw _failure('$key shows no text');
  return text;
}

/// The texts of every widget on screen: rendered text, text fields and selectable text.
List<String> _textsOnScreen() => [
  for (final e in find.byType(RichText).evaluate())
    (e.widget as RichText).text.toPlainText(),
  for (final e in find.byType(EditableText).evaluate())
    (e.widget as EditableText).controller.text,
  for (final e in find.byType(SelectableText).evaluate())
    (e.widget as SelectableText).data ?? '',
];

/// The pieces of [secret] that [expectNoTrace] looks for: every run of 8 characters after its
/// fixed prefix (for example `bothy_whi_`), which a screen may show to name the kind of token.
List<String> _pieces(String secret) {
  final body = secret.replaceFirst(RegExp(r'^[a-z]+(?:_[a-z]+)*_'), '');
  const n = 8;
  if (body.length <= n) return [body];
  return [for (var i = 0; i + n <= body.length; i++) body.substring(i, i + n)];
}

/// Fails when any widget on screen shows any part of [secret] (a run of 8 of its characters
/// after the prefix). The failure names the kind ([what]), never the secret.
void expectNoTrace(String secret, String what) {
  final pieces = _pieces(secret);
  final shown = _textsOnScreen().any((t) => pieces.any(t.contains));
  expect(shown, isFalse, reason: 'a widget still shows a part of the $what');
}

/// An answer of the API: the status and the JSON body (null when there is none).
typedef ApiAnswer = ({int status, Object? json});

final _uuid = Uuid7Generator(
  nowMillis: () => DateTime.now().millisecondsSinceEpoch,
  // pow, not a shift: on the web a shift by 32 gives 0.
  random: (bits) => Random.secure().nextInt(pow(2, bits).toInt()),
);

/// A fresh UUIDv7 for a client_msg_id or an Idempotency-Key (06 3).
String uuid7() => _uuid.next();

Future<ApiAnswer> _fetch(
  String url,
  String method,
  String credentials,
  Map<String, String> headers,
  Object? body,
) async {
  final h = web.Headers()..append('Accept', 'application/json');
  // A lambda: dart2js refuses a tear-off of an external interop member.
  headers.forEach((name, value) => h.append(name, value));
  if (body != null) h.append('Content-Type', 'application/json');
  final response = await web.window
      .fetch(
        url.toJS,
        web.RequestInit(
          method: method,
          credentials: credentials,
          headers: h,
          body: body == null ? null : jsonEncode(body).toJS,
        ),
      )
      .toDart;
  final text = (await response.text().toDart).toDart;
  Object? json;
  try {
    json = text.isEmpty ? null : jsonDecode(text);
  } on FormatException {
    json = null;
  }
  return (status: response.status, json: json);
}

/// A request of the console session to the API path [path] (below `/api/v1`): the cookie of
/// the page (`credentials: same-origin`) and the CSRF header `X-Console-Request: 1`; a changing
/// operation that takes an Idempotency-Key gets a fresh one with [idempotent]. A step-up the
/// server asks for fails the request; the flows call only operations without one here.
Future<ApiAnswer> consoleApi(
  String method,
  String path, {
  Object? body,
  bool idempotent = false,
}) => _fetch('/api/v1$path', method, 'same-origin', {
  'X-Console-Request': '1',
  if (idempotent) 'Idempotency-Key': uuid7(),
}, body);

/// A POST of [body] to [url] without any credential of the page (`credentials: omit`, no
/// cookie, no CSRF header): the call of an incoming webhook by an outside sender (11 8.4).
Future<ApiAnswer> postWithoutSession(String url, Object body) =>
    _fetch(url, 'POST', 'omit', const {}, body);

/// The `items` of a list answer, or a failure naming [what].
List<Map<String, Object?>> itemsOf(ApiAnswer answer, String what) {
  final json = answer.json;
  if (answer.status != 200 || json is! Map || json['items'] is! List) {
    throw _failure('$what answered ${answer.status}');
  }
  return [for (final i in json['items']! as List) (i as Map).cast()];
}

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

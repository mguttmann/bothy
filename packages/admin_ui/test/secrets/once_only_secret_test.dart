// SPDX-License-Identifier: Apache-2.0

// A secret the server returns once is shown once and then held by nothing (PLAN row of M6-08;
// brief M6-08 Design note 4, fixed requirement 17; 08 12): the address of a new incoming
// webhook with its canary token, and the HMAC secret of a new outgoing subscription, appear
// exactly once in the dialog; after "Close" no widget, no state description, no render or
// semantics node, no text field, no log line (print and debugPrint) and no storage plugin holds
// them, and reopening the channel lists the webhook without any part of the token.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';
import '../apps/hooks_support.dart';
import '../incoming_webhooks/incoming_support.dart';

/// The method channels of the storage plugins a host could reach (`shared_preferences`, which
/// is the browser's local storage on the web, and `flutter_secure_storage` of the desktop
/// client). Every call on them is recorded in [_Traces.storage].
const List<String> _storageChannels = [
  'plugins.flutter.io/shared_preferences',
  'plugins.it_nomads.com/flutter_secure_storage',
];

/// Everything a secret could leak into besides the widget tree.
final class _Traces {
  final List<String> logs = [];
  final Map<String, Object?> storage = {};
}

/// Installs the log capture and the fake storage for one test and runs [body] in a zone that
/// captures `print`.
Future<void> _traced(
  WidgetTester tester,
  Future<void> Function(_Traces traces) body,
) async {
  final traces = _Traces();
  final messenger = tester.binding.defaultBinaryMessenger;
  for (final name in _storageChannels) {
    messenger.setMockMethodCallHandler(MethodChannel(name), (call) async {
      traces.storage['$name ${call.method}'] = call.arguments;
      return null;
    });
  }
  addTearDown(() {
    for (final name in _storageChannels) {
      messenger.setMockMethodCallHandler(MethodChannel(name), null);
    }
  });
  final previous = debugPrint;
  debugPrint = (message, {wrapWidth}) => traces.logs.add(message ?? '');
  try {
    await runZoned(
      () => body(traces),
      zoneSpecification: ZoneSpecification(
        print: (self, parent, zone, line) => traces.logs.add(line),
      ),
    );
  } finally {
    // Restored inside the test body: the binding checks the debug variables before tear-down.
    debugPrint = previous;
  }
}

/// Every text the person can read or select: [Text], [RichText] and the text fields (a
/// [SelectableText] shows its text through the [EditableText] it builds, so it counts once).
List<String> _texts(WidgetTester tester) => [
  for (final w in tester.allWidgets)
    switch (w) {
      Text(:final data?) => data,
      Text(:final textSpan?) => textSpan.toPlainText(),
      RichText(:final text) => text.toPlainText(),
      EditableText(:final controller) => controller.text,
      _ => '',
    },
];

/// Fails when [secret] is anywhere in the tree, in the captured logs or in the storage.
void _expectNoTrace(WidgetTester tester, String secret, _Traces traces) {
  expect(_texts(tester).where((t) => t.contains(secret)), isEmpty);
  // The element tree describes every widget and every state object reachable from it.
  expect(tester.binding.rootElement!.toStringDeep(), isNot(contains(secret)));
  for (final w in tester.allWidgets) {
    expect(w.toStringDeep(), isNot(contains(secret)), reason: '$w');
  }
  for (final view in tester.binding.renderViews) {
    expect(view.toStringDeep(), isNot(contains(secret)));
    final semantics = view.owner?.semanticsOwner?.rootSemanticsNode;
    expect(semantics?.toStringDeep() ?? '', isNot(contains(secret)));
  }
  expect(traces.logs.where((l) => l.contains(secret)), isEmpty);
  expect(traces.storage, isEmpty);
}

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> incomingView(WidgetTester tester) => pumpInApp(
    tester,
    IncomingWebhooksView(
      // A new key per pump: reopening builds a new view with a new state.
      key: UniqueKey(),
      repository: repository,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
    navigatorKey: navigator,
  );

  testWidgets('the token of an incoming webhook appears exactly once, and '
      'after Close nothing holds it', (tester) async {
    seedIncoming(repository);
    await _traced(tester, (traces) async {
      await incomingView(tester);
      await tap(tester, 'incoming.create');
      await tester.enterText(
        find.byKey(const ValueKey('incoming.name')),
        'Deploy alerts',
      );
      await tap(tester, 'incoming.app');
      await tap(tester, 'incoming.app.app-repo-bridge');
      await tap(tester, 'incoming.submit');
      await tap(tester, 'stepUp.continue');

      // The canary of the fake server: handed out once, by this create.
      final token = repository.handedOut.single;
      expect(
        _texts(tester).where((t) => t.contains(token)),
        hasLength(1),
        reason: 'shown exactly once',
      );
      expect(find.byKey(const ValueKey('incoming.secret')), findsOneWidget);

      await tap(tester, 'incoming.secretClose');
      expect(find.byKey(const ValueKey('incoming.secretDialog')), findsNothing);
      _expectNoTrace(tester, token, traces);

      // Reopening the channel lists the webhook, without any part of the token.
      await incomingView(tester);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('incoming.row.hook-0')),
          matching: find.text('Deploy alerts'),
        ),
        findsOneWidget,
      );
      final texts = _texts(tester).join('\n');
      for (var i = 0; i + 8 <= token.length; i++) {
        expect(texts, isNot(contains(token.substring(i, i + 8))));
      }
      _expectNoTrace(tester, token, traces);
    });
  });

  testWidgets('the HMAC secret of a subscription appears exactly once, and '
      'after Close nothing holds it', (tester) async {
    final app = seedBridgeHooks(repository);
    repository.stepUpFresh = true;
    await _traced(tester, (traces) async {
      await pumpInApp(
        tester,
        AppSubscriptionsSection(
          repository: repository,
          app: app,
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
        navigatorKey: navigator,
      );
      await tap(tester, 'subs.create');
      await tester.enterText(
        find.byKey(const ValueKey('subs.url')),
        'https://bridge.example.com/hooks/signed',
      );
      await tap(tester, 'subs.event.message.created');
      await tap(tester, 'subs.hmac');
      await tap(tester, 'subs.submit');

      final secret = repository.handedOut.single;
      expect(_texts(tester).where((t) => t.contains(secret)), hasLength(1));

      await tap(tester, 'hmac.secretClose');
      _expectNoTrace(tester, secret, traces);
    });
  });
}

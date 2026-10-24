// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'terminal_fakes.dart';

const _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';

const _statement =
    'In the browser this check only notices a key change that the server reports; it does '
    'not protect against the server itself.';

final _host = Host(
  (b) => b
    ..id = buildHostId
    ..name = 'Build host'
    ..state = HostState.active
    ..llmCredentials = LlmCredentials.organization
    ..declaredServerHost = false
    ..noisePublicKey = hostNoiseKey
    ..keyFingerprint = hostFingerprint
    ..keyGeneration = 1
    ..profiles = ListBuilder<HostProfile>([
      HostProfile(
        (p) => p
          ..name = 'shell'
          ..displayName = 'Shell'
          ..kind = HostProfileKindEnum.shell,
      ),
    ])
    ..connected = true
    ..createdAt = DateTime.utc(2026, 10)
    ..version = 1,
);

Future<ConsoleTerminalPrompts> _pump(WidgetTester tester, Locale locale) async {
  final key = GlobalKey<NavigatorState>();
  await tester.pumpWidget(dsApp(key, locale: locale));
  return ConsoleTerminalPrompts(() => key.currentContext, selfUserId: _alex);
}

void main() {
  testWidgets('the open dialog carries the pinning statement', (tester) async {
    final prompts = await _pump(tester, const Locale('en'));
    final choice = prompts.confirmOpen(
      _host,
      const OpenChoice(profile: 'shell'),
    );
    await tester.pumpAndSettle();
    expect(find.byType(OpenTerminalDialog), findsOneWidget);
    expect(find.text(_statement), findsOneWidget);
    expect(
      find.byKey(const ValueKey('console.terminal.pinningNotice')),
      findsOneWidget,
    );
    await tester.tap(find.text('Confirm with passkey or device key'));
    await tester.pumpAndSettle();
    final chosen = await choice;
    expect(chosen?.profile, 'shell');
    expect(find.text(_statement), findsNothing);
  });

  testWidgets('the key change warning carries the pinning statement', (
    tester,
  ) async {
    final prompts = await _pump(tester, const Locale('en'));
    final accept = prompts.confirmKeyChange(
      _host,
      PinChanged(
        pinned: HostKeyPin(
          hostId: buildHostId,
          noisePublicKey: 'b3RoZXJrZXlvdGhlcmtleW90aGVya2V5b3RoZXJrZXkx',
          keyGeneration: 1,
          fingerprint: 'ABCD-M9XD-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000',
          firstSeen: DateTime.utc(2026, 10, 1),
        ),
        currentFingerprint: hostFingerprint,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(KeyChangedDialog), findsOneWidget);
    expect(find.text(_statement), findsOneWidget);
    // "Do not connect" stays the main action and opens nothing.
    await tester.tap(find.text('Do not connect'));
    await tester.pumpAndSettle();
    expect(await accept, isFalse);
  });

  testWidgets('the statement is translated', (tester) async {
    final prompts = await _pump(tester, const Locale('de'));
    final choice = prompts.confirmOpen(
      _host,
      const OpenChoice(profile: 'shell'),
    );
    await tester.pumpAndSettle();
    final notice = tester.widget<Text>(
      find.byKey(const ValueKey('console.terminal.pinningNotice')),
    );
    expect(notice.data, isNot(_statement));
    expect(notice.data, contains('Server'));
    Navigator.of(tester.element(find.byType(OpenTerminalDialog))).pop();
    await tester.pumpAndSettle();
    expect(await choice, isNull);
  });

  testWidgets(
    'the warning of another person\'s host and the purpose come from terminal_ui',
    (tester) async {
      final prompts = await _pump(tester, const Locale('en'));
      expect(prompts.stepUpPurpose(_host, StepUpPurpose.open), isNot(isEmpty));
      final go = prompts.confirmForeignHost(_host);
      await tester.pumpAndSettle();
      expect(find.byType(ForeignHostWarningDialog), findsOneWidget);
      expect(find.text(_statement), findsNothing);
      Navigator.of(tester.element(find.byType(ForeignHostWarningDialog))).pop();
      await tester.pumpAndSettle();
      expect(await go, isFalse);
    },
  );
}

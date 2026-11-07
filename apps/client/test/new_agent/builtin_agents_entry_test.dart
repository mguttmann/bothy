// SPDX-License-Identifier: Apache-2.0

// "Built-in agents" under Me on the phone (brief A1-09 requirement 16 and Scope; 01 6.20 item
// 6; docs/decisions.d/A1-09.md): the phone has no agent list of the administration, so an
// owner whose server has built-in agents reaches their edit, versions, runner change and
// disable from Me; the entry is hidden otherwise, the page lies on the stack of Me, and a
// link to it without the entry is "Not found or no access".

import 'dart:io';

import 'package:client/app/router.dart';
import 'package:client/new_agent/builtin_agents_entry.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';

import '../phone/support/phone_app.dart';

/// A phone window tall enough for every entry of the Me tab at once.
const Size _tallPhone = Size(390, 1200);

void main() {
  late Directory root;

  setUp(() => root = Directory.systemTemp.createTempSync('builtin-agents'));
  tearDown(() => root.deleteSync(recursive: true));

  List<Override> available(bool value) => [
    builtinAgentsAvailableProvider.overrideWith((ref) async => value),
  ];

  test('the page lies on the stack of Me', () {
    final at = parseLocation(Uri.parse(builtinAgentsPath));
    expect(tabOf(at, PhoneTab.spaces), PhoneTab.me);
    expect(phoneStack(PhoneTab.me, Uri.parse(builtinAgentsPath)), const [
      PhonePage(PhonePageKind.me, '/settings'),
      PhonePage(PhonePageKind.builtinAgents, '/admin/agents'),
    ]);
    // One agent has no page of its own on the phone: the list shows each one in full.
    expect(
      phoneStack(PhoneTab.me, Uri.parse('/admin/agents/notes')).last.kind,
      PhonePageKind.notFound,
    );
  });

  for (final platform in ['ios', 'android']) {
    testWidgets('$platform: an owner with built-in agents has the entry', (
      tester,
    ) async {
      final app = await PhoneApp.start(
        tester,
        root: root,
        overrides: available(true),
        platform: platform,
      );
      phoneWindow(tester, size: _tallPhone);
      await app.go('/settings');
      final entry = find.byKey(const ValueKey('phone.me.builtinAgents'));
      expect(entry, findsOneWidget);
      expect(find.text('Built-in agents'), findsOneWidget);
      await tester.tap(entry);
      await tester.pumpAndSettle();
      expect(app.location, builtinAgentsPath);
      expect(app.shell.pages.map((p) => p.kind), [
        PhonePageKind.me,
        PhonePageKind.builtinAgents,
      ]);
    });
  }

  testWidgets(
    'no entry without built-in agents or for a person who is no owner',
    (tester) async {
      final app = await PhoneApp.start(
        tester,
        root: root,
        overrides: available(false),
      );
      phoneWindow(tester, size: _tallPhone);
      await app.go('/settings');
      expect(
        find.byKey(const ValueKey('phone.me.builtinAgents')),
        findsNothing,
      );
    },
  );

  testWidgets('a link to the page without the entry is not found', (
    tester,
  ) async {
    final app = await PhoneApp.start(
      tester,
      root: root,
      overrides: available(false),
    );
    await app.go(builtinAgentsPath);
    expect(find.byKey(const ValueKey('target.notFound')), findsOneWidget);
  });
}

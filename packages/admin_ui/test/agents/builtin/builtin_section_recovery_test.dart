// SPDX-License-Identifier: Apache-2.0

// The built-in agent card when a read or a change fails: a read that fails after a saved edit
// leaves the card with its retry instead of an unhandled error, a failed load offers "Try
// again", a refused "Disable" shows as a danger banner, a program error never reads as
// "offline", and a cancelled step-up says nothing.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../admin/support.dart';
import '../new_agent/fake_new_agent_port.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

/// A port whose first read of a definition after a saved one fails without an answer.
class _Port extends FakeNewAgentPort {
  bool failAfterSave = false;

  @override
  Future<StoredDefinition?> definition(String agentId) async {
    if (failAfterSave && writes.contains('putDefinition')) {
      failAfterSave = false;
      throw const NewAgentFailure.offline();
    }
    return super.definition(agentId);
  }
}

void main() {
  late _Port port;
  var keys = 0;
  var changed = 0;

  setUp(() {
    keys = 0;
    changed = 0;
    port = _Port()..addAgent('a', definitions: [_plain]);
  });

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key));
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> section(WidgetTester tester) async {
    await pumpInApp(
      tester,
      BuiltinAgentSection(
        host: NewAgentHost(
          port: port,
          newKey: () => 'key-${++keys}',
          announceMinInterval: const Duration(seconds: 10),
        ),
        agentId: 'a',
        onChanged: () => changed++,
      ),
    );
    await tester.pumpAndSettle();
  }

  DsBanner banner(WidgetTester tester, String key) =>
      tester.widget<DsBanner>(find.byKey(ValueKey(key)));

  Future<void> disable(WidgetTester tester) async {
    await tap(tester, 'builtinAgent.disable');
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')),
      'Not needed any more',
    );
    await tap(tester, 'confirm.submit');
  }

  testWidgets('a failed read after a saved edit leaves a retry', (
    tester,
  ) async {
    await section(tester);
    await tap(tester, 'builtinAgent.edit');
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.instructions.field')),
      'Keep the notes of Robin.',
    );
    await tester.pumpAndSettle();
    // The save goes through; the card's next read of the definition finds no server.
    port.failAfterSave = true;
    await tap(tester, 'agentEdit.save');
    expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
    expect(port.definitionsOf('a'), hasLength(2));
    expect(changed, 1);
    expect(banner(tester, 'builtinAgent.error').actionLabel, 'Try again');
    banner(tester, 'builtinAgent.error').onAction!();
    await tester.pumpAndSettle();
    expect(find.text('Definition version 2'), findsOneWidget);
  });

  testWidgets('a failed load offers Try again', (tester) async {
    port.failNext['agent'] = const NewAgentFailure.offline();
    await section(tester);
    final error = banner(tester, 'builtinAgent.error');
    expect(error.tone, DsTone.danger);
    expect(error.actionLabel, 'Try again');
    error.onAction!();
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('builtinAgent.error')), findsNothing);
    expect(find.byKey(const ValueKey('builtinAgent.version')), findsOneWidget);
  });

  testWidgets('a program error of the load is no "offline"', (tester) async {
    port.failNext['agent'] = StateError('malformed answer');
    await section(tester);
    expect(
      banner(tester, 'builtinAgent.error').text,
      isNot(contains('connection')),
    );
    expect(
      banner(tester, 'builtinAgent.error').text,
      'Something went wrong on the server',
    );
  });

  testWidgets('a refused Disable shows as a danger banner', (tester) async {
    await section(tester);
    port.failNext['disable'] = const NewAgentFailure.problem('forbidden');
    await disable(tester);
    expect(banner(tester, 'builtinAgent.notice').tone, DsTone.danger);
    expect(port.activeOf('a'), isTrue);
  });

  testWidgets('a cancelled step-up of Disable says nothing', (tester) async {
    await section(tester);
    port.failNext['disable'] = const NewAgentFailure.stepUpCancelled();
    await disable(tester);
    expect(find.byKey(const ValueKey('builtinAgent.notice')), findsNothing);
  });

  testWidgets('a program error of Disable shows as a danger banner', (
    tester,
  ) async {
    await section(tester);
    port.failNext['disable'] = StateError('malformed answer');
    await disable(tester);
    expect(banner(tester, 'builtinAgent.notice').tone, DsTone.danger);
  });
}

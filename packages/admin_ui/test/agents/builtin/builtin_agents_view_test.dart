// SPDX-License-Identifier: Apache-2.0

// The built-in agents of the phone (brief A1-09 Scope and requirement 16; 01 6.20 item 6; 26
// 8.4; docs/decisions.d/A1-09.md): every agent of kind claude_agent with its section, a
// disabled one marked, none with a sentence, and a failing list with the server's sentence.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../admin/support.dart';
import '../new_agent/fake_new_agent_port.dart';

void main() {
  late FakeNewAgentPort port;

  setUp(() => port = FakeNewAgentPort());

  Future<void> view(WidgetTester tester) async {
    await pumpInApp(
      tester,
      BuiltinAgentsView(
        host: NewAgentHost(
          port: port,
          newKey: () => 'key',
          announceMinInterval: const Duration(seconds: 10),
          fullScreen: true,
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('lists every built-in agent with its section', (tester) async {
    port
      ..addAgent('a')
      ..addAgent(
        'b',
        handle: 'files',
        displayName: 'Files',
        definitions: [
          {
            'instructions': 'Sort the files.',
            'skills': <Object?>[],
            'mcp_servers': <Object?>[],
            'settings': {'tool_class_wish': 'chat'},
            'origin': {'kind': 'manual'},
          },
        ],
      );
    await view(tester);
    expect(find.byKey(const ValueKey('builtinAgents.a')), findsOneWidget);
    expect(find.byKey(const ValueKey('builtinAgents.b')), findsOneWidget);
    expect(find.text('Notes'), findsOneWidget);
    expect(find.text('Files'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('builtinAgent.continueSetup')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('builtinAgent.edit')), findsOneWidget);
    // The phone names each agent itself; the heading of the agent card is not repeated.
    expect(find.text('Built-in agent'), findsNothing);
  });

  testWidgets('says so without a built-in agent', (tester) async {
    await view(tester);
    expect(find.text('No built-in agents yet.'), findsOneWidget);
  });

  testWidgets('a failing list shows the sentence of the server', (
    tester,
  ) async {
    port.failNext['builtinAgents'] = const NewAgentFailure.offline();
    await view(tester);
    expect(
      find.text('The server cannot be reached. Try again when you are online.'),
      findsOneWidget,
    );
  });
}

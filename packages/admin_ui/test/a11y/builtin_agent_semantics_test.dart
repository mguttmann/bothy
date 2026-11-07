// SPDX-License-Identifier: Apache-2.0

// Accessibility of editing a built-in agent (brief A1-09 acceptance criteria; 01 8): the
// section on the agent card, the editor with its changed parts, the versions with labelled
// Show and Restore of each version, and Change runner have labelled controls, contrast and
// targets in light and dark, and keep every control at text scale 2.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../agents/new_agent/fake_new_agent_port.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

FakeNewAgentPort _port() => FakeNewAgentPort()
  ..addAgent(
    'a',
    definitions: [
      _plain,
      {..._plain, 'instructions': 'B.'},
    ],
  );

NewAgentHost _host(NewAgentPort port) => NewAgentHost(
  port: port,
  newKey: () => 'key',
  announceMinInterval: const Duration(seconds: 10),
);

const NewAgentState _agent = NewAgentState(
  id: 'a',
  version: 3,
  toolClass: AgentToolClass.chat,
  name: 'Notes',
  handle: 'notes',
  hostedBy: 'runner-1',
);

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

void main() {
  dsAccessibilityTests(
    'the built-in agent section',
    build: () =>
        _frame(BuiltinAgentSection(host: _host(_port()), agentId: 'a')),
    reachable: (l) => [
      for (final k in ['edit', 'versions', 'changeRunner', 'disable'])
        find.byKey(ValueKey('builtinAgent.$k')),
    ],
  );

  dsAccessibilityTests(
    'the editor of a built-in agent',
    build: () => _frame(
      NewAgentDialog(
        port: _port(),
        offer: standardOffer,
        newKey: () => 'key',
        announceMinInterval: const Duration(seconds: 10),
        existing: ExistingAgent(
          agent: _agent,
          runner: devRunner,
          stored: StoredDefinition(2, {..._plain, 'instructions': 'B.'}),
          content: _plain,
          restoredVersion: 1,
        ),
      ),
    ),
    window: const Size(1280, 4000),
    semantics: (tester, l) {
      expect(find.text('Instructions changed'), findsOneWidget);
    },
  );

  dsAccessibilityTests(
    'the versions of a built-in agent',
    build: () => _frame(
      AgentVersionsDialog(
        port: _port(),
        agent: _agent,
        ownerNames: const {'alex': 'Alex'},
        onShow: (_) async {},
        onRestore: (_) async => false,
      ),
    ),
    semantics: (tester, l) {
      expect(find.bySemanticsLabel('Show version 2'), findsOneWidget);
      expect(find.bySemanticsLabel('Restore version 1'), findsOneWidget);
    },
  );

  dsAccessibilityTests(
    'Change runner',
    build: () => _frame(
      ChangeRunnerDialog(
        port: _port(),
        agent: _agent,
        runners: const [alexRunner],
      ),
    ),
    reachable: (l) => [
      find.byKey(const ValueKey('agentHost.runner.runner-2')),
      find.byKey(const ValueKey('agentHost.confirm')),
    ],
  );
}

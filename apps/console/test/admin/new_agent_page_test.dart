// SPDX-License-Identifier: Apache-2.0

// "New agent" in the console (brief A1-08 Tests and requirement 16; 01 6.20; 26 8.1): the
// agent list shows the entry only when the server answers listAgentTemplates and a runner
// reports hosts_agents; the dialog talks to the server only through the console's API with the
// cookie session (15: no gateway), runs create, definition and the class steps, and then opens
// the agent card.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_fakes.dart';

const String _runner = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _notes = '0192f4a0-7b1c-7d07-84b0-358556a08a97';

Map<String, Object?> _agent(
  String id,
  String handle, {
  String kind = 'code_runner',
  String toolClass = 'chat',
  Map<String, Object?> presence = const {'state': 'online'},
  int version = 1,
}) => {
  'id': id,
  'handle': handle,
  'display_name': handle,
  'status': 'active',
  'adapter_kind': kind,
  'description': '',
  'billing_mode': 'organization',
  'tool_class': toolClass,
  'presence': presence,
  'operator_config': <String, Object?>{},
  'version': version,
};

TestBrowser _browser({bool templates = true, bool hosting = true}) {
  final b = TestBrowser();
  adminRoutes(b.server);
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, adminSession());
  b.server.routes['GET /api/v1/owners'] = (_) => const Answer(200, {
    'items': [
      {
        'user': {
          'id': alexId,
          'kind': 'user',
          'handle': 'alex',
          'display_name': 'Alex',
        },
        'granted_at': '2026-10-10T09:30:00.000Z',
      },
    ],
  });
  b.server.routes['GET /api/v1/agents'] = (_) => Answer(200, {
    'items': [
      _agent(
        _runner,
        'dev-runner',
        presence: {'state': 'online', if (hosting) 'hosts_agents': 'host'},
      ),
    ],
  });
  b.server.routes['GET /api/v1/agent-templates'] = (_) => templates
      ? const Answer(200, {'items': <Object>[]})
      : const Answer(404, {'code': 'not_found', 'status': 404});
  return b;
}

void main() {
  testWidgets('no entry on a server without the operations', (tester) async {
    final b = _browser(templates: false);
    await pumpConsole(tester, b, location: '/agents');
    expect(find.byType(AgentsList), findsOneWidget);
    expect(find.byKey(const ValueKey('agents.new')), findsNothing);
  });

  testWidgets('no entry without a runner that hosts agents', (tester) async {
    final b = _browser(hosting: false);
    await pumpConsole(tester, b, location: '/agents');
    expect(find.byKey(const ValueKey('agents.new')), findsNothing);
  });

  testWidgets(
    'the empty form creates the agent through the console API and opens its card',
    (tester) async {
      tester.view.physicalSize = const Size(1600, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final b = _browser();
      var version = 1;
      var toolClass = 'chat';
      b.server.routes['POST /api/v1/agents'] = (_) =>
          Answer(201, _agent(_notes, 'notes', kind: 'claude_agent'));
      b.server.routes['GET /api/v1/agents/$_notes'] = (_) => Answer(
        200,
        _agent(
          _notes,
          'notes',
          kind: 'claude_agent',
          toolClass: toolClass,
          version: version,
        ),
      );
      b.server.routes['PUT /api/v1/agents/$_notes/tool-class'] = (r) {
        toolClass = (r.body! as Map)['tool_class']! as String;
        version++;
        return Answer(
          200,
          _agent(
            _notes,
            'notes',
            kind: 'claude_agent',
            toolClass: toolClass,
            version: version,
          ),
        );
      };
      b.server.routes['PUT /api/v1/agents/$_notes/definition'] = (r) {
        version++;
        return Answer(200, {
          'agent_id': _notes,
          'version': 1,
          'origin_kind': 'manual',
          'created_at': '2026-11-07T09:30:00.000Z',
          'created_by': alexId,
          'definition': r.body,
        });
      };
      final c = await pumpConsole(tester, b, location: '/agents');
      await tester.tap(find.byKey(const ValueKey('agents.new')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('newAgent.way.empty')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('newAgent.name.field')),
        'Notes',
      );
      await tester.enterText(
        find.byKey(const ValueKey('newAgent.handle.field')),
        'notes',
      );
      final scoped = find.byKey(const ValueKey('newAgent.class.scoped'));
      await tester.ensureVisible(scoped);
      await tester.tap(scoped);
      await tester.pumpAndSettle();
      final create = find.byKey(const ValueKey('newAgent.create'));
      await tester.tap(create);
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
      expect(c.path, '/agents/$_notes');
      final writes = [
        for (final r in b.server.seen)
          if (r.method != 'GET') '${r.method} ${r.path}',
      ];
      expect(writes, [
        'POST /api/v1/agents',
        'PUT /api/v1/agents/$_notes/tool-class',
        'PUT /api/v1/agents/$_notes/definition',
      ]);
      final create0 = b.server.to('POST', '/api/v1/agents').single;
      expect(create0.body, {
        'handle': 'notes',
        'display_name': 'Notes',
        'adapter_kind': 'claude_agent',
        'description': '',
        'hosted_by': _runner,
      });
      expect(create0.headers['idempotency-key'], isNotEmpty);
      // The console's API only: no gateway, every request under /api/v1 with the cookie session.
      expect(b.server.seen.every((r) => !r.path.contains('gateway')), isTrue);
    },
  );
}

// SPDX-License-Identifier: Apache-2.0

// Editing a built-in agent in the console (brief A1-09 Tests and requirements 12 to 16; 01 6.20
// item 6; 26 3.1, 4.1, 8.3, 8.4; R-BA-06; 10 10.3): the agent card of a built-in agent shows
// its section; Edit saves with putAgentDefinition and If-Match of the edited version; a 412
// offers Reload; Versions restores an older version as a new one; Change runner shows the
// class ceiling and the billing holder as sentences and moves the agent; a lowered class with
// a stdio server kept shows definition_needs_host and the way to remove the server; an added
// stdio server with host sets the class before the definition, a removed one with chat after
// it; Disable sends the reason; Continue setup resumes at the class step. Every request goes
// to the console's API with the cookie session (15: no gateway).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_fakes.dart';

const String _runner = '0192f4a0-7b1c-7d07-84b0-358556a08a99';
const String _small = '0192f4a0-7b1c-7d07-84b0-358556a08a96';
const String _notes = '0192f4a0-7b1c-7d07-84b0-358556a08a97';
const String _at = '2026-11-07T09:30:00.000Z';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

const Map<String, Object?> _withStdio = {
  'instructions': 'Sort the files Alex names.',
  'skills': <Object?>[],
  'mcp_servers': [
    {
      'name': 'files',
      'transport': 'stdio',
      'command': 'npx',
      'package': '@example/mcp-files@1.4.2',
    },
  ],
  'settings': {'tool_class_wish': 'host'},
  'origin': {'kind': 'manual'},
};

int _rank(String c) => switch (c) {
  'host' => 2,
  'scoped' => 1,
  _ => 0,
};

bool _stdio(Object? d) =>
    d is Map &&
    ((d['mcp_servers'] as List?) ?? const []).any(
      (s) => s is Map && s['transport'] == 'stdio',
    );

/// The built-in agent Notes as the server keeps it, with the rules of 26 3.1, 4.3 item 7, 4.4
/// item 5 and 10 10.3 that the console must show.
class _Notes {
  String toolClass = 'chat';
  String hostedBy = _runner;
  String status = 'active';
  String billing = 'organization';
  int version = 1;
  final List<Map<String, Object?>> definitions = [];

  Map<String, Object?> card() => {
    'id': _notes,
    'handle': 'notes',
    'display_name': 'Notes',
    'status': status,
    'adapter_kind': 'claude_agent',
    'description': '',
    'billing_mode': billing,
    if (billing == 'personal') 'credential_holder_id': alexId,
    'tool_class': toolClass,
    'presence': {'state': 'online'},
    'operator_config': <String, Object?>{},
    'hosted_by': hostedBy,
    'version': version,
  };

  Map<String, Object?> stored(int n, {bool withDefinition = true}) => {
    'agent_id': _notes,
    'version': n,
    'origin_kind': 'manual',
    'created_at': _at,
    'created_by': alexId,
    if (withDefinition) 'definition': definitions[n - 1],
  };
}

Map<String, Object?> _runnerCard(String id, String name, String ceiling) => {
  'id': id,
  'handle': name.toLowerCase().replaceAll(' ', '-'),
  'display_name': name,
  'status': 'active',
  'adapter_kind': 'code_runner',
  'description': '',
  'billing_mode': 'organization',
  'tool_class': 'host',
  'presence': {'state': 'online', 'hosts_agents': ceiling},
  'operator_config': <String, Object?>{},
  'version': 1,
};

int? _ifMatch(SeenRequest r) =>
    int.tryParse((r.headers['if-match'] ?? '').replaceAll('"', ''));

Answer _conflict(String reason) =>
    Answer(409, {'code': 'conflict', 'status': 409, 'reason': reason});

TestBrowser _browser(_Notes a) {
  final b = TestBrowser();
  adminRoutes(b.server);
  final routes = b.server.routes;
  routes['GET /api/v1/console/session'] = (_) => Answer(200, adminSession());
  routes['GET /api/v1/agents'] = (_) => Answer(200, {
    'items': [
      _runnerCard(_runner, 'Dev runner', 'host'),
      _runnerCard(_small, 'Small runner', 'scoped'),
      a.card(),
    ],
  });
  routes['GET /api/v1/agent-templates'] = (_) => Answer(200, {
    'items': [
      {
        'slug': 'file-helper',
        'source': 'shipped',
        'name': 'File helper',
        'description': 'Sorts files',
        'version': '1.0.0',
        'required_secrets': <String>[],
        'definition': _withStdio,
      },
    ],
  });
  routes['GET /api/v1/agents/$_notes'] = (_) => Answer(200, a.card());
  routes['PUT /api/v1/agents/$_notes/tool-class'] = (r) {
    if (_ifMatch(r) != a.version) {
      return Answer(412, {'code': 'version_conflict', 'status': 412});
    }
    final to = (r.body! as Map)['tool_class']! as String;
    if (_rank(to) < 2 &&
        a.definitions.isNotEmpty &&
        _stdio(a.definitions.last)) {
      return _conflict('definition_needs_host');
    }
    a
      ..toolClass = to
      ..version += 1;
    return Answer(200, a.card());
  };
  routes['GET /api/v1/agents/$_notes/definition'] = (r) {
    final n = int.tryParse(r.query['version'] ?? '') ?? a.definitions.length;
    if (n < 1 || n > a.definitions.length) {
      return const Answer(404, {'code': 'not_found', 'status': 404});
    }
    return Answer(200, a.stored(n));
  };
  routes['GET /api/v1/agents/$_notes/definition/versions'] = (_) =>
      Answer(200, {
        'items': [
          for (var n = a.definitions.length; n >= 1; n--)
            a.stored(n, withDefinition: false),
        ],
      });
  routes['PUT /api/v1/agents/$_notes/definition'] = (r) {
    final n = a.definitions.length;
    if (_ifMatch(r) != (n == 0 ? null : n)) {
      return Answer(412, {
        'code': 'version_conflict',
        'status': 412,
        if (n > 0) 'current': a.stored(n),
      });
    }
    if (a.toolClass != 'host' && _stdio(r.body)) {
      return const Answer(400, {
        'code': 'validation_failed',
        'status': 400,
        'errors': [
          {'pointer': '/mcp_servers/0/transport', 'code': 'tool_class_too_low'},
        ],
      });
    }
    a
      ..definitions.add(Map<String, Object?>.from(r.body! as Map))
      ..version += 1;
    return Answer(200, a.stored(a.definitions.length));
  };
  routes['PUT /api/v1/agents/$_notes/host'] = (r) {
    if (_ifMatch(r) != a.version) {
      return Answer(412, {'code': 'version_conflict', 'status': 412});
    }
    final to = (r.body! as Map)['hosted_by']! as String;
    if (to == _small && _rank(a.toolClass) > 1) {
      return _conflict('host_tool_class_ceiling');
    }
    if (a.billing == 'personal') return _conflict('hosted_holder_mismatch');
    a
      ..hostedBy = to
      ..version += 1;
    return Answer(200, a.card());
  };
  routes['POST /api/v1/agents/$_notes/disable'] = (_) {
    a
      ..status = 'disabled'
      ..version += 1;
    return Answer(200, a.card());
  };
  return b;
}

void main() {
  late _Notes notes;
  late TestBrowser b;

  setUp(() {
    notes = _Notes();
    b = _browser(notes);
  });

  Future<void> open(WidgetTester tester) async {
    tester.view.physicalSize = const Size(1600, 1600);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await pumpConsole(tester, b, location: '/agents/$_notes');
    await tester.pumpAndSettle();
  }

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key));
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  List<String> writes() => [
    for (final r in b.server.seen)
      if (r.method != 'GET') '${r.method} ${r.path}',
  ];

  void noGateway() =>
      expect(b.server.seen.every((r) => !r.path.contains('gateway')), isTrue);

  String textOf(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
        matchRoot: true,
      ),
    ))
      t.data ?? '',
  ].join(' | ');

  testWidgets('Edit saves through the console API with If-Match', (
    tester,
  ) async {
    notes
      ..definitions.add(_plain)
      ..version = 2;
    await open(tester);
    expect(find.byKey(const ValueKey('builtinAgent')), findsOneWidget);
    await tap(tester, 'builtinAgent.edit');
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.instructions.field')),
      'Keep the notes of Robin.',
    );
    await tester.pumpAndSettle();
    await tap(tester, 'agentEdit.save');
    expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
    expect(writes(), ['PUT /api/v1/agents/$_notes/definition']);
    final put = b.server.to('PUT', '/api/v1/agents/$_notes/definition').single;
    expect(put.headers['if-match'], '"1"');
    expect((put.body! as Map)['instructions'], 'Keep the notes of Robin.');
    expect(notes.definitions, hasLength(2));
    noGateway();
  });

  testWidgets('a 412 offers Reload', (tester) async {
    notes
      ..definitions.add(_plain)
      ..version = 2;
    await open(tester);
    await tap(tester, 'builtinAgent.edit');
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.instructions.field')),
      'Mine.',
    );
    await tester.pumpAndSettle();
    notes
      ..definitions.add({..._plain, 'instructions': 'Theirs.'})
      ..version += 1;
    await tap(tester, 'agentEdit.save');
    expect(
      textOf(tester, 'newAgent.failure'),
      startsWith('Another device changed this agent.'),
    );
    await tap(tester, 'agentEdit.reload');
    expect(find.byKey(const ValueKey('agentEdit.noChanges')), findsOneWidget);
  });

  testWidgets('Versions: Restore saves version 1 again', (tester) async {
    notes
      ..definitions.addAll([
        _plain,
        {..._plain, 'instructions': 'B.'},
      ])
      ..version = 3;
    await open(tester);
    await tap(tester, 'builtinAgent.versions');
    await tap(tester, 'agentVersions.restore.1');
    await tap(tester, 'agentEdit.save');
    final put = b.server.to('PUT', '/api/v1/agents/$_notes/definition').single;
    expect(put.headers['if-match'], '"2"');
    expect(notes.definitions.last['instructions'], 'Keep the notes of Sam.');
    expect(notes.definitions, hasLength(3));
  });

  testWidgets('Change runner: the ceiling as a sentence, then the move', (
    tester,
  ) async {
    notes
      ..toolClass = 'host'
      ..definitions.add(_withStdio)
      ..version = 2;
    b.server.routes['GET /api/v1/agents'] = (_) => Answer(200, {
      'items': [
        _runnerCard(_runner, 'Dev runner', 'host'),
        _runnerCard(_small, 'Small runner', 'scoped'),
        _runnerCard(
          '0192f4a0-7b1c-7d07-84b0-358556a08a95',
          'Build host',
          'host',
        ),
        notes.card(),
      ],
    });
    await open(tester);
    await tap(tester, 'builtinAgent.changeRunner');
    await tap(tester, 'agentHost.runner.$_small');
    await tap(tester, 'agentHost.confirm');
    expect(
      textOf(tester, 'agentHost.error'),
      startsWith('Small runner loads at most the tool class Scoped'),
    );
    await tap(tester, 'agentHost.runner.0192f4a0-7b1c-7d07-84b0-358556a08a95');
    await tap(tester, 'agentHost.confirm');
    expect(find.byKey(const ValueKey('agentHost.dialog')), findsNothing);
    expect(notes.hostedBy, '0192f4a0-7b1c-7d07-84b0-358556a08a95');
    final put = b.server.to('PUT', '/api/v1/agents/$_notes/host').last;
    expect(put.headers['if-match'], '"2"');
    noGateway();
  });

  testWidgets('Change runner: the billing holder as a sentence', (
    tester,
  ) async {
    notes
      ..billing = 'personal'
      ..definitions.add(_plain)
      ..version = 2;
    await open(tester);
    await tap(tester, 'builtinAgent.changeRunner');
    await tap(tester, 'agentHost.runner.$_small');
    await tap(tester, 'agentHost.confirm');
    expect(
      textOf(tester, 'agentHost.error'),
      startsWith('The agent bills Personal'),
    );
    expect(
      find.byKey(const ValueKey('agentHost.switchBilling')),
      findsOneWidget,
    );
  });

  testWidgets(
    'a lowered class with the stdio server kept: definition_needs_host, removed, Try again',
    (tester) async {
      notes
        ..toolClass = 'host'
        ..definitions.add(_withStdio)
        ..version = 2;
      await open(tester);
      await tap(tester, 'builtinAgent.edit');
      await tap(tester, 'newAgent.class.chat');
      await tap(tester, 'agentEdit.save');
      expect(
        textOf(tester, 'newAgent.failure'),
        startsWith('The stored definition still has a local MCP server'),
      );
      await tap(tester, 'agentEdit.removeLocal');
      await tap(tester, 'newAgent.retry');
      expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
      expect(notes.toolClass, 'chat');
      expect(_stdio(notes.definitions.last), isFalse);
    },
  );

  testWidgets('an added stdio server with host: class, then definition', (
    tester,
  ) async {
    notes
      ..definitions.add(_plain)
      ..version = 2;
    await open(tester);
    await tap(tester, 'builtinAgent.edit');
    await tap(tester, 'newAgent.server.add');
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.server.0.name.field')),
      'files',
    );
    await tap(tester, 'newAgent.server.0.transport.stdio');
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.server.0.command.field')),
      'npx',
    );
    await tester.enterText(
      find.byKey(const ValueKey('newAgent.server.0.package.field')),
      '@example/mcp-files@1.4.2',
    );
    await tester.pumpAndSettle();
    await tap(tester, 'newAgent.class.host');
    await tap(tester, 'agentEdit.save');
    expect(writes(), [
      'PUT /api/v1/agents/$_notes/tool-class',
      'PUT /api/v1/agents/$_notes/definition',
    ]);
    expect(notes.toolClass, 'host');
  });

  testWidgets('a removed stdio server with chat: definition, then class', (
    tester,
  ) async {
    notes
      ..toolClass = 'host'
      ..definitions.add(_withStdio)
      ..version = 2;
    await open(tester);
    await tap(tester, 'builtinAgent.edit');
    await tap(tester, 'newAgent.server.0.remove');
    await tap(tester, 'newAgent.class.chat');
    await tap(tester, 'agentEdit.save');
    expect(writes(), [
      'PUT /api/v1/agents/$_notes/definition',
      'PUT /api/v1/agents/$_notes/tool-class',
    ]);
    expect(notes.toolClass, 'chat');
  });

  testWidgets('Disable sends the reason', (tester) async {
    notes
      ..definitions.add(_plain)
      ..version = 2;
    await open(tester);
    await tap(tester, 'builtinAgent.disable');
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')),
      'Not needed any more',
    );
    await tap(tester, 'confirm.submit');
    final post = b.server.to('POST', '/api/v1/agents/$_notes/disable').single;
    expect(post.body, {'reason': 'Not needed any more'});
    expect(notes.status, 'disabled');
  });

  testWidgets('Continue setup resumes at the class step', (tester) async {
    await open(tester);
    expect(find.byKey(const ValueKey('builtinAgent.notSetUp')), findsOneWidget);
    await tap(tester, 'builtinAgent.continueSetup');
    await tap(tester, 'newAgent.way.template');
    await tap(tester, 'newAgent.template.file-helper');
    await tap(tester, 'agentEdit.continue');
    expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
    expect(writes(), [
      'PUT /api/v1/agents/$_notes/tool-class',
      'PUT /api/v1/agents/$_notes/definition',
    ]);
    expect(writes().where((w) => w == 'POST /api/v1/agents'), isEmpty);
    noGateway();
  });
}

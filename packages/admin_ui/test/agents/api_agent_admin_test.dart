// SPDX-License-Identifier: Apache-2.0

// The administration of an adapter agent from the screens down to the wire (rulings AUD-AA1/5
// to AUD-AA1/8; 11 13 step 1; 09 2.1, 2.2; 07 2.11): "Add adapter agent" of the list sends
// `createAgent`, and the agent page sends `disableAgent`, `enableAgent`, `setAgentToolClass`
// and `updateAgent` with the members, the `If-Match` and the `Idempotency-Key` the contract
// asks for. The widgets run over `ApiAdminRepository`, the generated client and its decoder;
// the adapter below keeps the agents like the server (docs/api/openapi.yaml: `AgentCreate`,
// `ReasonBody`, `ToolClassSet`, `AgentUpdate`, `Agent`). The widget tests use only the keys of
// the controls, so they read the same on a build without the controls.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' hide AgentPage;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../self/support.dart';

typedef _Seen = ({RequestOptions options, Object? json});

const String _helper = '0192f4a0-7b1c-7d07-84b0-358556a08a98';
const String _created = '0192f4a0-7b1c-7d07-84b0-358556a08a90';

/// The agent routes of the server over a map of agents.
class _AgentServer implements HttpClientAdapter {
  final Map<String, Map<String, Object?>> agents = {
    _helper: _agent(_helper, 'helper', 'Helper', 'hermes'),
  };
  final List<_Seen> seen = [];

  static Map<String, Object?> _agent(
    String id,
    String handle,
    String name,
    String kind, {
    String description = '',
  }) => {
    'id': id,
    'handle': handle,
    'display_name': name,
    'status': 'active',
    'adapter_kind': kind,
    'description': description,
    'billing_mode': 'organization',
    'tool_class': 'chat',
    'presence': {'state': 'unknown'},
    'operator_config': <String, Object?>{},
    'version': 1,
  };

  /// The changing requests in order, as `METHOD path`.
  List<String> get changes => [
    for (final s in seen)
      if (s.options.method != 'GET')
        '${s.options.method} ${s.options.uri.path}',
  ];

  _Seen last(String method, String path) => seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    final text = utf8.decode(bytes);
    final json = text.isEmpty ? null : jsonDecode(text);
    seen.add((options: options, json: json));
    final (status, answer) = _answer(
      options,
      json is Map ? json.cast<String, Object?>() : const {},
    );
    return ResponseBody.fromString(
      jsonEncode(answer),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  (int, Object) _problem(int status, String code, [List<Object>? errors]) => (
    status,
    {
      'type': '/api/v1/problems/$code',
      'title': code,
      'status': status,
      'code': code,
      'errors': ?errors,
    },
  );

  (int, Object) _answer(RequestOptions options, Map<String, Object?> body) {
    final method = options.method;
    final path = options.uri.path.replaceFirst('/api/v1', '');
    const empty = {'items': <Object?>[]};
    if (method == 'GET' && path == '/agents') {
      return (200, {'items': agents.values.toList()});
    }
    if (method == 'GET' && (path == '/users' || path == '/spaces')) {
      return (200, empty);
    }
    if (method == 'GET' && path.startsWith('/principals/')) return (200, empty);
    if (method == 'POST' && path == '/agents') {
      if (agents.values.any((a) => a['handle'] == body['handle'])) {
        return _problem(409, 'conflict', [
          {'pointer': '/handle', 'code': 'taken'},
        ]);
      }
      return (
        201,
        agents[_created] = _agent(
          _created,
          body['handle']! as String,
          body['display_name']! as String,
          body['adapter_kind']! as String,
          description: body['description'] as String? ?? '',
        ),
      );
    }
    final parts = path.split('/');
    final agent = parts.length > 2 && parts[1] == 'agents'
        ? agents[parts[2]]
        : null;
    if (agent == null) return _problem(404, 'not_found');
    final step = parts.length > 3 ? parts[3] : '';
    if (method == 'GET' && step.isEmpty) return (200, agent);
    void change(Map<String, Object?> members) => agent
      ..addAll(members)
      ..['version'] = (agent['version']! as int) + 1;
    if (method == 'POST' && (step == 'disable' || step == 'enable')) {
      if ((body['reason'] as String? ?? '').trim().isEmpty) {
        return _problem(400, 'validation_failed', [
          {'pointer': '/reason', 'code': 'required'},
        ]);
      }
      change({'status': step == 'disable' ? 'disabled' : 'active'});
      return (200, agent);
    }
    if (options.headers['If-Match'] != '"${agent['version']}"') {
      return _problem(412, 'version_conflict');
    }
    if (method == 'PUT' && step == 'tool-class') {
      change({'tool_class': body['tool_class']});
      return (200, agent);
    }
    if (method == 'PATCH' && step.isEmpty) {
      change({'operator_config': body['operator_config']});
      return (200, agent);
    }
    return _problem(404, 'not_found');
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late _AgentServer server;
  late AdminRepository repository;
  var keys = 0;

  setUp(() {
    server = _AgentServer();
    keys = 0;
    repository = ApiAdminRepository(
      createApiClient(
        baseUrl: Uri.parse('https://chat.example.com'),
        traceContext: () => TraceContext(
          traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
          sampled: true,
        ),
        readRetry: ReadRetryPolicy.fromBackoff(
          base: const Duration(milliseconds: 1),
          cap: const Duration(milliseconds: 1),
          sleep: (_) async {},
        ),
        adapter: server,
      ),
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${keys++}',
    );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pumpAndSettle();
  }

  // The requests of the widgets are real asynchronous work of the client, so each step runs
  // in `runAsync` until the request it waits for was seen.
  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 20; i++) {
      await tester.runAsync(() => Future<void>.delayed(Duration.zero));
      await tester.pump();
    }
    await tester.pumpAndSettle();
  }

  group('the list', () {
    testWidgets('"Add adapter agent" creates the agent of a runner and opens '
        'its page', (tester) async {
      final opened = <String>[];
      await pumpInApp(
        tester,
        AgentsList(repository: repository, onOpen: opened.add),
        settle: false,
      );
      await settle(tester);
      expect(find.byKey(const ValueKey('agent.$_helper')), findsOneWidget);

      await tap(tester, 'agents.add');
      await type(tester, 'addAgent.name', 'Dev runner');
      await type(tester, 'addAgent.handle', 'dev-runner');
      await tap(tester, 'addAgent.kind');
      await tester.tap(find.text('Claude Code runner').last);
      await tester.pumpAndSettle();
      await type(tester, 'addAgent.description', 'Runs the builds');
      await tap(tester, 'confirm.submit');
      await settle(tester);

      final sent = server.last('POST', '/api/v1/agents');
      expect(sent.json, {
        'handle': 'dev-runner',
        'display_name': 'Dev runner',
        'adapter_kind': 'code_runner',
        'description': 'Runs the builds',
      });
      expect(
        sent.options.headers['Idempotency-Key'],
        '0192f4a0-0000-7000-8000-000000000000',
      );
      expect(
        sent.options.extra[stepUpPurposeExtra],
        'Create the agent Dev runner',
      );
      expect(opened, [_created]);
      expect(find.byKey(const ValueKey('agent.$_created')), findsOneWidget);
    });

    testWidgets('a taken handle says so and opens nothing', (tester) async {
      final opened = <String>[];
      await pumpInApp(
        tester,
        AgentsList(repository: repository, onOpen: opened.add),
        settle: false,
      );
      await settle(tester);

      await tap(tester, 'agents.add');
      await type(tester, 'addAgent.name', 'Second helper');
      await type(tester, 'addAgent.handle', 'helper');
      await tap(tester, 'addAgent.kind');
      await tester.tap(find.text('Own adapter').last);
      await tester.pumpAndSettle();
      await tap(tester, 'confirm.submit');
      await settle(tester);

      expect(
        find.text('This handle is taken. Change it and try again.'),
        findsOneWidget,
      );
      expect(opened, isEmpty);
      expect(server.agents, hasLength(1));
    });
  });

  group('the page of an adapter agent', () {
    Future<void> page(WidgetTester tester) async {
      await pumpInApp(
        tester,
        AgentPage(repository: repository, agentId: _helper, clock: testClock),
        settle: false,
      );
      await settle(tester);
      expect(find.byKey(const ValueKey('agentPage.badge')), findsOneWidget);
    }

    testWidgets('"Disable" and "Enable" send the reason', (tester) async {
      await page(tester);

      await tap(tester, 'agentPage.disable');
      await type(tester, 'confirm.reason', 'Adapter is being replaced');
      await tap(tester, 'confirm.submit');
      await settle(tester);

      final disabled = server.last('POST', '/api/v1/agents/$_helper/disable');
      expect(disabled.json, {'reason': 'Adapter is being replaced'});
      expect(disabled.options.headers['Idempotency-Key'], isNotNull);
      expect(server.agents[_helper]!['status'], 'disabled');
      expect(find.text('This agent is disabled.'), findsOneWidget);
      expect(find.byKey(const ValueKey('agentPage.disable')), findsNothing);

      await tap(tester, 'agentPage.enable');
      await type(tester, 'confirm.reason', 'New adapter is ready');
      await tap(tester, 'confirm.submit');
      await settle(tester);

      final enabled = server.last('POST', '/api/v1/agents/$_helper/enable');
      expect(enabled.json, {'reason': 'New adapter is ready'});
      expect(server.agents[_helper]!['status'], 'active');
      expect(find.byKey(const ValueKey('agentPage.enable')), findsNothing);
      expect(find.byKey(const ValueKey('agentPage.disable')), findsOneWidget);
    });

    testWidgets(
      '"Change tool class" sends the class, the reason and If-Match',
      (tester) async {
        await page(tester);
        expect(
          tester
              .widget<Text>(find.byKey(const ValueKey('agentPage.toolClass')))
              .data,
          'Chat',
        );

        await tap(tester, 'agentPage.toolClassChange');
        await tap(tester, 'toolClass.scoped');
        await type(tester, 'confirm.reason', 'Works in a project checkout');
        await tap(tester, 'confirm.submit');
        await settle(tester);

        final sent = server.last('PUT', '/api/v1/agents/$_helper/tool-class');
        expect(sent.json, {
          'tool_class': 'scoped',
          'reason': 'Works in a project checkout',
        });
        expect(sent.options.headers['If-Match'], '"1"');
        expect(
          tester
              .widget<Text>(find.byKey(const ValueKey('agentPage.toolClass')))
              .data,
          'Scoped',
        );
      },
    );

    testWidgets('"Edit features" stores operator_config with If-Match', (
      tester,
    ) async {
      await page(tester);
      expect(
        find.text('Everything the adapter reports is allowed.'),
        findsOneWidget,
      );

      await tap(tester, 'agentPage.featuresEdit');
      await tap(tester, 'features.supports_voice');
      await type(tester, 'features.defaultModel', 'model-medium');
      await tap(tester, 'confirm.submit');
      await settle(tester);

      final sent = server.last('PATCH', '/api/v1/agents/$_helper');
      expect(sent.json, {
        'operator_config': {
          'default_model': 'model-medium',
          'supports_voice': false,
        },
      });
      expect(sent.options.headers['If-Match'], '"1"');
      expect(sent.options.headers['Idempotency-Key'], isNotNull);
      expect(find.text('Switched off: Voice input'), findsOneWidget);
      expect(find.text('Default model: model-medium'), findsOneWidget);
      expect(server.changes, ['PATCH /api/v1/agents/$_helper']);
    });
  });
}

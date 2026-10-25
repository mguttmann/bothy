// SPDX-License-Identifier: Apache-2.0

// The REST side of the terminal (brief M4-36; 15 23.1): the adapter calls `getHost`,
// `openTerminalSession`, `attachTerminalSession` and `closeTerminalSession` with their bodies,
// reads every page of `listHosts` and `listTerminalSessions`, and passes a refusal of the
// server on unchanged with its problem.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:client/terminal/terminal_api_adapter.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/testing.dart';

import 'support.dart';

/// Answers the requests from [routes] (method and path to status and JSON) and records them.
final class _Routes implements HttpClientAdapter {
  _Routes(this.routes);

  final Map<String, (int, Object?) Function(RequestOptions o)> routes;
  final List<String> seen = [];
  final List<Object?> bodies = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final key = '${options.method} ${options.uri.path}';
    seen.add('$key${options.uri.hasQuery ? '?${options.uri.query}' : ''}');
    if (requestStream != null) {
      final bytes = await requestStream.expand((c) => c).toList();
      bodies.add(bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)));
    }
    final route = routes[key];
    final (status, body) = route == null ? (404, null) : route(options);
    return ResponseBody.fromString(
      jsonEncode(body ?? {'code': 'not_found', 'status': 404}),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late FakeHostsApi fake;
  late ApiClient client;
  late _Routes routes;

  Object? hostJson(Host h) =>
      client.serializers.serializeWith(Host.serializer, h);
  Object? sessionJson(TerminalSession s) =>
      client.serializers.serializeWith(TerminalSession.serializer, s);
  Object? grantJson(TerminalSessionGrant g) =>
      client.serializers.serializeWith(TerminalSessionGrant.serializer, g);

  setUp(() async {
    fake = FakeHostsApi(await FakeHost.create(hostId: hostIdText));
    const base = '/api/v1';
    routes = _Routes({
      'GET $base/hosts': (o) => o.uri.queryParameters['cursor'] == null
          ? (
              200,
              {
                'items': [hostJson(fake.current)],
                'next_cursor': 'c2',
              },
            )
          : (
              200,
              {
                'items': [hostJson(fake.current)],
              },
            ),
      'GET $base/hosts/$hostIdText': (_) => (200, hostJson(fake.current)),
      'GET $base/terminal/sessions': (o) => (
        200,
        {
          'items': [
            sessionJson(FakeHostsApi.session(FakeHostsApi.sessionId(1))),
          ],
        },
      ),
      'POST $base/hosts/$hostIdText/terminal/sessions': (_) => (
        403,
        {'code': 'terminal_denied', 'status': 403, 'reason': 'session_limit'},
      ),
      'POST $base/terminal/sessions/${FakeHostsApi.sessionId(1)}/attach': (_) =>
          (200, grantJson(fake.grantFor(FakeHostsApi.sessionId(1)))),
      'POST $base/terminal/sessions/${FakeHostsApi.sessionId(1)}/close': (_) =>
          (200, sessionJson(FakeHostsApi.session(FakeHostsApi.sessionId(1)))),
    });
    client = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(traceId: '4' * 32, sampled: false),
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      adapter: routes,
    );
  });

  test('lists every page, reads the host and the sessions', () async {
    final api = ApiTerminalServerApi(client, closeReason: 'Done');
    expect(await api.listHosts(), hasLength(2));
    expect((await api.getHost(hostIdText)).name, 'Build host');
    expect(await api.listSessions(hostId: hostIdText), hasLength(1));
    expect(routes.seen, [
      'GET /api/v1/hosts',
      'GET /api/v1/hosts?cursor=c2',
      'GET /api/v1/hosts/$hostIdText',
      'GET /api/v1/terminal/sessions?host_id=$hostIdText',
    ]);
  });

  test(
    'attach and close send their bodies; a refused opening keeps its problem',
    () async {
      final api = ApiTerminalServerApi(
        client,
        closeReason: 'Ended in the view',
      );
      final id = FakeHostsApi.sessionId(1);
      final grant = await api.attachSession(
        id,
        TerminalSessionAttach(
          (b) => b
            ..cols = 80
            ..rows = 24
            ..clientEphemeralKeyHash = 'A' * 43,
        ),
      );
      expect(grant.session.id, id);
      await api.closeSession(id);
      expect(routes.bodies.last, {'reason': 'Ended in the view'});
      Object? error;
      try {
        await api.openSession(
          hostIdText,
          TerminalSessionOpen(
            (b) => b
              ..profile = 'shell'
              ..cols = 80
              ..rows = 24
              ..clientEphemeralKeyHash = 'A' * 43,
          ),
        );
      } on Object catch (e) {
        error = e;
      }
      final problem = ApiProblem.of(error!);
      expect(
        (problem?.code, problem?.reason),
        ('terminal_denied', 'session_limit'),
      );
    },
  );
}

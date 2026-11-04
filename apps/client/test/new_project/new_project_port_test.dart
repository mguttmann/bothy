// SPDX-License-Identifier: Apache-2.0

// The errors of the new project port as the dialog shows them: a session without any step-up
// method is not a cancel, and a locked password step-up shows its reason (SEC-STEPUP-LOCKOUT).
// F-PROJEKT-BEREIT: the runner choice offers no chat agent as a runner, and the upload grant
// is write with upload for the runner on the forum.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:client/new_project/new_project_dialog.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/new_project_port.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' as d;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show openInMemoryForTests;
import 'package:sync_engine/sync_engine.dart' show Outbox, OutboxExecutors;

const String _forumId = '0192f4a0-7b1c-72d4-8bae-000000000002';
const String _runnerId = '0192f4a0-7b1c-72d4-8bae-000000000003';
const String _hermesId = '0192f4a0-7b1c-72d4-8bae-000000000004';
const String _offId = '0192f4a0-7b1c-72d4-8bae-000000000005';

Map<String, Object?> _agent(
  String id,
  String name,
  String kind, {
  String status = 'active',
}) => {
  'id': id,
  'handle': name.toLowerCase().replaceAll(' ', '-'),
  'display_name': name,
  'status': status,
  'adapter_kind': kind,
  'description': '',
  'billing_mode': 'organization',
  'tool_class': 'chat',
  'presence': {'agent_id': id, 'state': 'online'},
  'operator_config': <String, Object?>{},
  'version': 1,
};

/// Answers every request from [routes] (`METHOD path` to status and body) and records it.
final class _Adapter implements HttpClientAdapter {
  _Adapter(this.routes);

  final Map<String, (int, Object?)> routes;
  final List<(String, Object?, Map<String, dynamic>)> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final key = '${options.method} ${options.uri.path}';
    final bytes = requestStream == null
        ? <int>[]
        : [for (final chunk in await requestStream.toList()) ...chunk];
    seen.add((
      key,
      bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)),
      options.headers,
    ));
    final (status, body) = routes[key] ?? (404, {'code': 'not_found'});
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// The port over [adapter] with an in-memory outbox, which these tests never use; the
/// database is closed at the end of the test.
ApiNewProjectPort _port(_Adapter adapter) {
  final db = openInMemoryForTests();
  addTearDown(db.close);
  return ApiNewProjectPort(
    api: ApiClient(
      dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com/v1'))
        ..httpClientAdapter = adapter,
      interceptors: const [],
    ),
    outbox: Outbox(
      db: db,
      executors: OutboxExecutors(),
      config: d.AppConfig.defaults(),
      ids: d.Uuid7Generator(nowMillis: () => 0, random: (bits) => 0),
    ),
    dao: db.outboxDao,
    purpose: (name) => 'Create $name',
    reason: 'New project from the app',
  );
}

DioException _wrapped(Object error) => DioException(
  requestOptions: RequestOptions(path: '/channels'),
  error: error,
);

DioException _answered(ApiProblem problem) {
  final options = RequestOptions(path: '/channels');
  return DioException(
    requestOptions: options,
    response: Response<Object?>(
      requestOptions: options,
      statusCode: problem.status,
    ),
    error: problem,
  );
}

void main() {
  final l = lookupAppLocalizations(const Locale('en'));

  test('a cancelled step-up and a lost session read as a cancel', () {
    for (final e in [
      const StepUpCancelled(),
      _wrapped(const StepUpCancelled()),
      _wrapped(const NotSignedIn()),
    ]) {
      expect(
        newProjectFailureOf(e)?.kind,
        NewProjectFailureKind.stepUpCancelled,
      );
    }
  });

  test('a session without a step-up method is not a cancel', () {
    for (final e in [
      const StepUpUnavailable(),
      _wrapped(const StepUpUnavailable()),
    ]) {
      final failure = newProjectFailureOf(e)!;
      expect(
        (failure.kind, failure.code, failure.reason),
        (NewProjectFailureKind.problem, 'forbidden', 'step_up_not_possible'),
      );
      expect(
        newProjectFailureText(l, NewProjectStep.createForum, failure),
        l.problemForbiddenStepUpNotPossible,
      );
    }
  });

  test('a locked password step-up shows its reason', () {
    final failure = newProjectFailureOf(
      _answered(
        const ApiProblem(
          status: 401,
          code: 'step_up_required',
          reason: 'password_locked',
        ),
      ),
    )!;
    expect(
      newProjectFailureText(l, NewProjectStep.bindProject, failure),
      l.problemStepUpRequiredPasswordLocked,
    );
  });

  test(
    'the runner choice offers no chat agent as a runner (09 18.16)',
    () async {
      final adapter = _Adapter({
        'GET /v1/agents': (
          200,
          {
            'items': [
              _agent(_hermesId, 'Hermes', 'hermes'),
              _agent(_runnerId, 'Claude Code', 'code_runner'),
              _agent(_offId, 'Old runner', 'code_runner', status: 'disabled'),
            ],
          },
        ),
      });
      final runners = await _port(adapter).runners();
      expect(
        [for (final r in runners) (r.name, r.canServe)],
        [('Claude Code', true), ('Hermes', false)],
      );
      expect(runners.where((r) => r.canServe).map((r) => r.id.value), [
        _runnerId,
      ]);
    },
  );

  test(
    'the upload grant is write with upload for the runner on the forum',
    () async {
      final adapter = _Adapter({
        'POST /v1/grants': (
          201,
          {
            'id': '0192f4a0-7b1c-72d4-8bae-000000000009',
            'subject_kind': 'agent',
            'subject_id': _runnerId,
            'target_kind': 'channel',
            'target_id': _forumId,
            'level': 'write',
            'capabilities': ['upload'],
            'reason': 'New project from the app',
            'granted_by': _hermesId,
            'granted_at': '2026-11-04T09:00:00Z',
          },
        ),
      });
      await _port(adapter).grantUpload(
        d.ChannelId.parse(_forumId),
        d.PrincipalId.parse(_runnerId),
        idempotencyKey: 'grant-key',
      );
      final (key, body, headers) = adapter.seen.single;
      expect(key, 'POST /v1/grants');
      expect(body, {
        'subject_kind': 'agent',
        'subject_id': _runnerId,
        'target_kind': 'channel',
        'target_id': _forumId,
        'level': 'write',
        'capabilities': ['upload'],
        'reason': 'New project from the app',
      });
      expect(headers['Idempotency-Key'], 'grant-key');
    },
  );

  test('no answer is offline and other errors pass', () {
    expect(
      newProjectFailureOf(_wrapped(Exception('socket')))?.kind,
      NewProjectFailureKind.offline,
    );
    expect(newProjectFailureOf(StateError('bug')), isNull);
  });
}

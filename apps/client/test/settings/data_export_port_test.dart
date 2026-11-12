// SPDX-License-Identifier: Apache-2.0

// The API port of the export of the own data (docs/decisions.d/AUD-client-settings-01.md;
// 20 5.1) over the client the app builds (`createApiClient` with its interceptors and the
// generated decoder) and a scripted network that answers what the server of this tree
// answered in `services/server/test/privacy` (`TestExport_AsTheAppAsksForIt`): the newest
// export among the requests about the caller, filing one, the open request a second one
// meets, the archive written into the chosen file, and the answers before the archive
// exists and after `privacy.export_ttl`.

import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart' as rest;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:client/settings/data_export.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'data_export_support.dart';

void main() {
  late Directory out;
  setUp(() async => out = await Directory.systemTemp.createTemp('data-export'));
  tearDown(() => out.delete(recursive: true));

  Future<Object?> failure(Future<Object?> call) =>
      call.then<Object?>((_) => null, onError: (Object e) => e);

  test('the newest export among the requests about the caller is read; a '
      'request of another kind is none', () async {
    final (:port, :network) = exportPort();
    network.answers
      ..add(Answer.json(200, RealAnswers.getMe))
      ..add(Answer.json(200, RealAnswers.listBefore))
      ..add(Answer.json(200, RealAnswers.listApproved));

    expect(await port.latest(), isNull);
    expect(
      await port.latest(),
      DataExport(
        id: RealAnswers.export,
        phase: DataExportPhase.preparing,
        requestedAt: RealAnswers.requestedAt,
      ),
    );
    // The caller is asked for once; the list names the caller, because an owner would get
    // the requests of everyone otherwise.
    expect(network.lines, [
      'GET /api/v1/me',
      'GET /api/v1/gdpr-requests?subject_user_id=${RealAnswers.alex}',
      'GET /api/v1/gdpr-requests?subject_user_id=${RealAnswers.alex}',
    ]);
  });

  test('a completed export is ready until its archive expires', () async {
    final justBefore = RealAnswers.availableUntil.subtract(
      const Duration(milliseconds: 1),
    );
    final ready = exportPort(now: justBefore);
    ready.network.answers
      ..add(Answer.json(200, RealAnswers.getMe))
      ..add(Answer.json(200, RealAnswers.listCompleted));
    expect(
      await ready.port.latest(),
      DataExport(
        id: RealAnswers.export,
        phase: DataExportPhase.ready,
        requestedAt: RealAnswers.requestedAt,
        availableUntil: RealAnswers.availableUntil,
      ),
    );

    final expired = exportPort(now: RealAnswers.availableUntil);
    expired.network.answers
      ..add(Answer.json(200, RealAnswers.getMe))
      ..add(Answer.json(200, RealAnswers.listCompleted));
    expect(
      await expired.port.latest(),
      DataExport(
        id: RealAnswers.export,
        phase: DataExportPhase.expired,
        requestedAt: RealAnswers.requestedAt,
      ),
    );
  });

  test('the request names the kind, the reason, a key and the purpose of the '
      'step-up', () async {
    final (:port, :network) = exportPort();
    network.answers.add(Answer.json(201, RealAnswers.created));

    final export = await port.request(
      reason: 'Requested by the person in the app',
      purpose: 'Request the export of your data',
    );

    expect(export.id, RealAnswers.export);
    expect(export.phase, DataExportPhase.preparing);
    expect(export.open, isTrue);
    final sent = network.requests.single;
    expect('${sent.method} ${sent.uri.path}', 'POST /api/v1/gdpr-requests');
    expect(sent.data, {
      'kind': 'export',
      'reason': 'Requested by the person in the app',
    });
    expect(sent.headers['Idempotency-Key'], 'key-1');
    expect(sent.extra[stepUpPurposeExtra], 'Request the export of your data');
  });

  test('a request while one is open gives the open request', () async {
    final (:port, :network) = exportPort();
    network.answers
      ..add(Answer.json(409, RealAnswers.openConflict))
      ..add(Answer.json(200, RealAnswers.getMe))
      ..add(Answer.json(200, RealAnswers.listApproved));

    final export = await port.request(reason: 'r', purpose: 'p');

    expect(export.id, RealAnswers.export);
    expect(export.phase, DataExportPhase.preparing);
  });

  test('a refused request keeps the problem; no answer has none', () async {
    final (:port, :network) = exportPort();
    network.answers.add(
      Answer.json(
        403,
        jsonEncode({
          'type': '/api/v1/problems/forbidden',
          'title': 'Forbidden',
          'status': 403,
          'code': 'forbidden',
          'reason': 'step_up_not_possible',
        }),
      ),
    );
    final refused = await failure(port.request(reason: 'r', purpose: 'p'));
    expect(refused, isA<DataExportRefused>());
    final problem = (refused! as DataExportRefused).problem!;
    expect(
      (problem.status, problem.code, problem.reason),
      (403, 'forbidden', 'step_up_not_possible'),
    );

    network.down = true;
    final offline = await failure(port.latest());
    expect(offline, isA<DataExportRefused>());
    expect((offline! as DataExportRefused).problem, isNull);
  });

  test('the archive is written unchanged into the chosen file', () async {
    final (:port, :network) = exportPort();
    final first = List<int>.generate(70000, (i) => i % 251);
    final second = [0x50, 0x4b, 0x05, 0x06];
    network.answers.add(Answer.archive([first, second]));
    final export = readyExport();
    final path = '${out.path}/export.zip';

    final bytes = await port.download(
      export,
      path: path,
      purpose: 'Download the export of your data',
    );

    expect(bytes, first.length + second.length);
    expect(await File(path).readAsBytes(), [...first, ...second]);
    final sent = network.requests.single;
    expect(
      '${sent.method} ${sent.uri.path}',
      'GET /api/v1/gdpr-requests/${RealAnswers.export}/export',
    );
    expect(sent.extra[stepUpPurposeExtra], 'Download the export of your data');
    // The request is signed like a generated one, although it runs on the plain client.
    expect(sent.extra['secure'], isNotEmpty);
  });

  test('before the job finished and after the archive expired the server '
      'refuses, and no file stays', () async {
    final export = readyExport();
    for (final (status, body, reason) in [
      (409, RealAnswers.notReady, 'export_not_ready'),
      (410, RealAnswers.gone, null),
    ]) {
      final (:port, :network) = exportPort();
      network.answers.add(Answer.json(status, body));
      final path = '${out.path}/export-$status.zip';

      final refused = await failure(
        port.download(export, path: path, purpose: 'p'),
      );

      expect(refused, isA<DataExportRefused>());
      final problem = (refused! as DataExportRefused).problem!;
      expect((problem.status, problem.reason), (status, reason));
      expect(File(path).existsSync(), isFalse);
    }
  });

  test('a stream that breaks off or is cancelled leaves no file', () async {
    final export = readyExport();
    final broken = exportPort();
    broken.network.answers.add(
      Answer.archive([
        [1, 2, 3],
      ], breaks: true),
    );
    final path = '${out.path}/broken.zip';
    expect(
      await failure(broken.port.download(export, path: path, purpose: 'p')),
      isA<DataExportBrokeOff>(),
    );
    expect(File(path).existsSync(), isFalse);

    final cancelled = exportPort();
    final cancel = CancelToken();
    cancelled.network.answers.add(
      Answer.archive([
        [1, 2, 3],
        [4, 5, 6],
      ]),
    );
    cancel.cancel();
    final other = '${out.path}/cancelled.zip';
    expect(
      await failure(
        cancelled.port.download(
          export,
          path: other,
          purpose: 'p',
          cancel: cancel,
        ),
      ),
      anyOf(isA<DataExportBrokeOff>(), isA<DataExportRefused>()),
    );
    expect(File(other).existsSync(), isFalse);
  });

  test('every state of a request has its phase', () {
    rest.GdprRequest request(rest.GdprRequestState state, {DateTime? until}) =>
        rest.GdprRequest(
          (b) => b
            ..id = 'r1'
            ..subjectUserId = 'u1'
            ..kind = rest.GdprRequestKind.export_
            ..state = state
            ..requestedBy = 'u1'
            ..reason = 'r'
            ..dueAt = _epoch
            ..createdAt = _epoch
            ..resultExpiresAt = until,
        );
    final now = DateTime.utc(2026, 11, 12);
    DataExportPhase phase(rest.GdprRequestState state, {DateTime? until}) =>
        DataExport.of(request(state, until: until), now: now).phase;

    expect(phase(rest.GdprRequestState.requested), DataExportPhase.waiting);
    expect(phase(rest.GdprRequestState.approved), DataExportPhase.preparing);
    expect(phase(rest.GdprRequestState.running), DataExportPhase.preparing);
    expect(
      phase(
        rest.GdprRequestState.completed,
        until: now.add(const Duration(seconds: 1)),
      ),
      DataExportPhase.ready,
    );
    expect(
      phase(rest.GdprRequestState.completed, until: now),
      DataExportPhase.expired,
    );
    expect(phase(rest.GdprRequestState.completed), DataExportPhase.expired);
    expect(phase(rest.GdprRequestState.rejected), DataExportPhase.rejected);
    expect(phase(rest.GdprRequestState.cancelled), DataExportPhase.cancelled);
    expect(phase(rest.GdprRequestState.failed), DataExportPhase.failed);
    expect(
      phase(rest.GdprRequestState.unknownDefaultOpenApi),
      DataExportPhase.failed,
    );
  });
}

final DateTime _epoch = DateTime.utc(2026, 11, 12);

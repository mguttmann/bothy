// SPDX-License-Identifier: Apache-2.0

// What the tests of the export of the own data share: the answers of the real server, a
// scripted network and the API port over the client the app builds.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/settings/data_export.dart';
import 'package:dio/dio.dart';

/// The answers of the server of this tree to the calls of the app, as
/// `TestExport_AsTheAppAsksForIt` logged them (`go test ./test/privacy -run
/// TestExport_AsTheAppAsksForIt -v` in `services/server`). Alex is the caller; an owner filed
/// the deletion, a request of another kind about Alex.
abstract final class RealAnswers {
  static const String alex = '01a1cf9a-20fa-7adf-87b5-b50901131de1';
  static const String export = '01a1cf9a-27d8-76a3-825a-94506175c5e9';

  static const String getMe =
      '{"device_id":"01a1cf9a-2109-7381-92d1-9ea86f7bd903","email":"alex@example.com","is_owner":false,"locale":"de","permission_version":16,"principal":{"display_name":"Alex","handle":"alex","id":"01a1cf9a-20fa-7adf-87b5-b50901131de1","kind":"user"},"session_id":"01a1cf9a-2115-7bbe-bf9f-39c7f5a6697b","time_zone":"UTC"}';

  static const String _deletion =
      '{"created_at":"2026-11-12T11:52:23.567Z","due_at":"2026-12-12T11:52:23.567Z","id":"01a1cf9a-279c-7128-b716-6d1bc209d742","kind":"deletion","reason":"Asked by mail","requested_by":"01a1cf9a-1b9c-799b-9c7d-5cc9800eab35","state":"requested","subject_user_id":"01a1cf9a-20fa-7adf-87b5-b50901131de1"}';

  /// `GET /gdpr-requests?subject_user_id=<Alex>` before the export was requested.
  static const String listBefore = '{"items":[$_deletion]}';

  /// `POST /gdpr-requests` with `{"kind":"export","reason":...}`: `201`.
  static const String created =
      '{"created_at":"2026-11-12T11:52:23.567Z","due_at":"2026-12-12T11:52:23.567Z","id":"01a1cf9a-27d8-76a3-825a-94506175c5e9","kind":"export","reason":"Requested by the person in the app","requested_by":"01a1cf9a-20fa-7adf-87b5-b50901131de1","state":"approved","subject_user_id":"01a1cf9a-20fa-7adf-87b5-b50901131de1"}';

  /// The list after the request: the newest request first.
  static const String listApproved = '{"items":[$created,$_deletion]}';

  /// `POST /gdpr-requests` while the export is open: `409`.
  static const String openConflict =
      '{"type":"/api/v1/problems/conflict","title":"Conflict","status":409,"code":"conflict","correlation_id":"dea66be0-e7e2-fe01-3530-8de4beda449e","detail":"an open request of this kind exists: 01a1cf9a-27d8-76a3-825a-94506175c5e9","instance":"/api/v1/gdpr-requests","current":{"created_at":"2026-11-12T11:52:23.567Z","due_at":"2026-12-12T11:52:23.567Z","id":"01a1cf9a-27d8-76a3-825a-94506175c5e9","kind":"export","progress":{},"reason":"Requested by the person in the app","requested_by":"01a1cf9a-20fa-7adf-87b5-b50901131de1","state":"approved","subject_user_id":"01a1cf9a-20fa-7adf-87b5-b50901131de1"},"reason":"request_open"}';

  /// `GET /gdpr-requests/<id>/export` before the job finished: `409`.
  static const String notReady =
      '{"type":"/api/v1/problems/conflict","title":"Conflict","status":409,"code":"conflict","correlation_id":"6d6a6943-e391-52b9-6db0-03391a380d2e","detail":"the request has no export package","instance":"/api/v1/gdpr-requests/01a1cf9a-27d8-76a3-825a-94506175c5e9/export","reason":"export_not_ready"}';

  /// The list after the job; it reads the same after `privacy.export_ttl` (one hour in that
  /// test), when the archive is gone: `result_expires_at` stays and lies in the past.
  static const String listCompleted =
      '{"items":[{"completed_at":"2026-11-12T11:52:23.567Z","created_at":"2026-11-12T11:52:23.567Z","due_at":"2026-12-12T11:52:23.567Z","id":"01a1cf9a-27d8-76a3-825a-94506175c5e9","kind":"export","progress":{"bytes":4867},"reason":"Requested by the person in the app","requested_by":"01a1cf9a-20fa-7adf-87b5-b50901131de1","result_expires_at":"2026-11-12T12:52:23.567Z","state":"completed","subject_user_id":"01a1cf9a-20fa-7adf-87b5-b50901131de1"},$_deletion]}';

  /// `GET /gdpr-requests/<id>/export` after `privacy.export_ttl`: `410`.
  static const String gone =
      '{"type":"/api/v1/problems/gone","title":"Gone","status":410,"code":"gone","correlation_id":"23598285-10f0-fa02-b0b6-e74c8dc9efcc","detail":"the export package has expired","instance":"/api/v1/gdpr-requests/01a1cf9a-27d8-76a3-825a-94506175c5e9/export"}';

  /// When the export was requested and until when its archive was kept.
  static final DateTime requestedAt = DateTime.utc(
    2026,
    11,
    12,
    11,
    52,
    23,
    567,
  );
  static final DateTime availableUntil = requestedAt.add(
    const Duration(hours: 1),
  );
}

/// One scripted answer of the network: a body, or a stream that ends with an error.
final class Answer {
  Answer.json(this.status, String body)
    : chunks = [utf8.encode(body)],
      contentType = status < 400
          ? Headers.jsonContentType
          : 'application/problem+json',
      breaks = false;

  Answer.archive(this.chunks, {this.breaks = false})
    : status = 200,
      contentType = 'application/zip';

  final int status;
  final List<List<int>> chunks;
  final String contentType;

  /// Whether the stream ends with an error after its chunks.
  final bool breaks;
}

/// A network that answers the requests in order from [answers], or fails every request with
/// a connection error when [down] is set.
class ScriptedNetwork implements HttpClientAdapter {
  final List<Answer> answers = [];
  final List<RequestOptions> requests = [];
  bool down = false;

  /// `METHOD path?query` of every request so far.
  List<String> get lines => [
    for (final r in requests) '${r.method} ${r.uri.path}${_query(r.uri)}',
  ];

  static String _query(Uri uri) => uri.hasQuery ? '?${uri.query}' : '';

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (down) {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'no route',
      );
    }
    final a = answers.removeAt(0);
    Stream<Uint8List> body() async* {
      for (final chunk in a.chunks) {
        yield Uint8List.fromList(chunk);
      }
      if (a.breaks) throw const SocketException('connection reset');
    }

    return ResponseBody(
      body(),
      a.status,
      headers: {
        Headers.contentTypeHeader: [a.contentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// The port over the client of the app (the interceptors and the decoder of
/// `createApiClient`; one attempt per read) at the time [now].
({ApiDataExportPort port, ScriptedNetwork network}) exportPort({
  DateTime? now,
}) {
  final network = ScriptedNetwork();
  final api = rest.createApiClient(
    baseUrl: Uri.parse('https://chat.example.com'),
    traceContext: () => rest.TraceContext(
      traceId: '0af7651916cd43dd8448eb211c80319c',
      sampled: false,
    ),
    readRetry: rest.ReadRetryPolicy(
      base: Duration.zero,
      cap: Duration.zero,
      maxAttempts: 1,
    ),
    adapter: network,
  );
  var n = 0;
  return (
    port: ApiDataExportPort(
      api,
      newIdempotencyKey: () => 'key-${++n}',
      clock: () => now ?? RealAnswers.requestedAt,
    ),
    network: network,
  );
}

/// The completed export of [RealAnswers], as the row hands it to the download.
DataExport readyExport() => DataExport(
  id: RealAnswers.export,
  phase: DataExportPhase.ready,
  requestedAt: RealAnswers.requestedAt,
  availableUntil: RealAnswers.availableUntil,
);

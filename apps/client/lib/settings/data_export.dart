// SPDX-License-Identifier: Apache-2.0

/// The export of the own data from the settings (docs/spec/01-product.md 5.1 D8 "[Export]";
/// docs/spec/20-privacy.md 5.1 and 5.2; openapi `listGdprRequests`, `createGdprRequest`,
/// `downloadGdprExport`): the newest export request of the signed-in person, filing one, and
/// the archive of a completed one streamed unchanged into the file the person chose.
///
/// Filing and downloading need a step-up (05 `gdpr.request.create`, `gdpr.export.download`):
/// the requests carry their purpose text and the client's `StepUpInterceptor` asks when the
/// server answers `401 step_up_required` (15 1). Nothing of the archive stays in the app, and
/// a broken or cancelled download deletes its partial file
/// (docs/decisions.d/AUD-client-settings-01.md).
library;

import 'dart:async';
import 'dart:io' as io;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show AuthException, stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../export/channel_export.dart' show ChannelExporter;

part 'data_export.g.dart';

/// Where an export request stands, as the row of the settings tells it.
enum DataExportPhase {
  /// Filed; an owner has to approve it (`privacy.export_requires_approval`, 20 5.1 item 2).
  waiting,

  /// Approved; the job has not finished the archive yet.
  preparing,

  /// Completed; the archive can be downloaded until [DataExport.availableUntil].
  ready,

  /// Completed, and the archive was deleted after `privacy.export_ttl` (20 5.1 item 5).
  expired,

  /// An owner rejected the request.
  rejected,

  /// The request was cancelled.
  cancelled,

  /// The job failed.
  failed,
}

/// An export request of the signed-in person.
final class DataExport {
  /// Creates the request.
  const DataExport({
    required this.id,
    required this.phase,
    required this.requestedAt,
    this.availableUntil,
  });

  /// Maps the API request [r] at the time [now]: a completed request whose
  /// `result_expires_at` is absent or has passed is [DataExportPhase.expired].
  factory DataExport.of(GdprRequest r, {required DateTime now}) {
    final until = r.resultExpiresAt;
    final phase = switch (r.state) {
      GdprRequestState.requested => DataExportPhase.waiting,
      GdprRequestState.approved ||
      GdprRequestState.running => DataExportPhase.preparing,
      GdprRequestState.completed =>
        until != null && now.isBefore(until)
            ? DataExportPhase.ready
            : DataExportPhase.expired,
      GdprRequestState.rejected => DataExportPhase.rejected,
      GdprRequestState.cancelled => DataExportPhase.cancelled,
      // `failed`, and a state a newer server added: nothing to wait for or download.
      _ => DataExportPhase.failed,
    };
    return DataExport(
      id: r.id,
      phase: phase,
      requestedAt: r.createdAt,
      availableUntil: phase == DataExportPhase.ready ? until : null,
    );
  }

  /// The ID of the request.
  final String id;

  /// Where the request stands.
  final DataExportPhase phase;

  /// When the request was filed.
  final DateTime requestedAt;

  /// Until when the archive is kept; only for [DataExportPhase.ready].
  final DateTime? availableUntil;

  /// Whether the request is still open: a new one would be refused (20 5.1, one open
  /// request per person and kind).
  bool get open =>
      phase == DataExportPhase.waiting || phase == DataExportPhase.preparing;

  @override
  bool operator ==(Object other) =>
      other is DataExport &&
      other.id == id &&
      other.phase == phase &&
      other.requestedAt == requestedAt &&
      other.availableUntil == availableUntil;

  @override
  int get hashCode => Object.hash(id, phase, requestedAt, availableUntil);
}

/// The instance refused a step of the export or did not answer.
final class DataExportRefused implements Exception {
  /// Creates the failure.
  const DataExportRefused(this.problem);

  /// The server's problem, or null without an answer.
  final ApiProblem? problem;
}

/// The download ended before the archive was complete; its partial file is deleted.
final class DataExportBrokeOff implements Exception {
  /// Creates the failure.
  const DataExportBrokeOff();
}

/// The export of the own data as the settings use it.
///
/// Every method throws [DataExportRefused] for a refusal or a missing answer; an error of
/// the login layer (the step-up cancelled, a lost session) passes as it is.
abstract interface class DataExportPort {
  /// The newest export request of the signed-in person, or null when there is none.
  Future<DataExport?> latest();

  /// Files the request for the export with [reason] (stored with the request and in the
  /// audit log) and returns it; [purpose] is the text of the step-up. While a request is
  /// open already (`409 conflict`, reason `request_open`), that request is returned.
  Future<DataExport> request({required String reason, required String purpose});

  /// Writes the archive of [export] byte for byte into the file at [path] and returns the
  /// number of bytes; [purpose] is the text of the step-up and [cancel] aborts the stream.
  ///
  /// Throws [DataExportBrokeOff] when the stream ended with an error or was cancelled. In
  /// every failure the file at [path] is deleted.
  Future<int> download(
    DataExport export, {
    required String path,
    required String purpose,
    CancelToken? cancel,
  });
}

/// [DataExportPort] over the client of the signed-in session (DPoP, refresh and the step-up
/// interceptor apply).
///
/// Example:
/// ```dart
/// final port = ApiDataExportPort(session.api, newIdempotencyKey: ids.next, clock: clock.now);
/// final export = await port.request(reason: l.settingsDataExportReason, purpose: purpose);
/// ```
final class ApiDataExportPort implements DataExportPort {
  /// Creates the port; [newIdempotencyKey] returns a fresh UUIDv7 per request and [clock]
  /// the time a completed request is compared with.
  ApiDataExportPort(
    this._api, {
    required String Function() newIdempotencyKey,
    required DateTime Function() clock,
  }) : _newKey = newIdempotencyKey,
       _now = clock;

  final ApiClient _api;
  final String Function() _newKey;
  final DateTime Function() _now;
  String? _me;

  PrivacyApi get _privacy => _api.getPrivacyApi();

  /// The ID of the signed-in person. An owner may read the requests of everyone, so the list
  /// is asked for this person by name.
  Future<String> _self() async =>
      _me ??= (await _api.getMeApi().getMe()).data!.principal.id;

  @override
  Future<DataExport?> latest() => _call(() async {
    // Newest first; the requests of the other kinds (deletion, rectification) are skipped.
    final page = (await _privacy.listGdprRequests(subjectUserId: await _self()))
        .data!;
    final newest = page.items
        .where((r) => r.kind == GdprRequestKind.export_)
        .firstOrNull;
    return newest == null ? null : DataExport.of(newest, now: _now());
  });

  @override
  Future<DataExport> request({
    required String reason,
    required String purpose,
  }) => _call(() async {
    try {
      final created = (await _privacy.createGdprRequest(
        gdprRequestCreate: GdprRequestCreate(
          (b) => b
            ..kind = GdprRequestKind.export_
            ..reason = reason,
        ),
        idempotencyKey: _newKey(),
        extra: {stepUpPurposeExtra: purpose},
      )).data!;
      return DataExport.of(created, now: _now());
    } on DioException catch (e) {
      final problem = ApiProblem.of(e);
      if (problem?.status != 409 || problem?.reason != 'request_open') rethrow;
      // Another device or an owner filed it meanwhile: that request is the state.
      final open = await latest();
      if (open == null) rethrow;
      return open;
    }
  });

  @override
  Future<int> download(
    DataExport export, {
    required String path,
    required String purpose,
    CancelToken? cancel,
  }) async {
    final response = await _call(
      () => _api.dio.get<ResponseBody>(
        '/gdpr-requests/${export.id}/export',
        options: Options(
          responseType: ResponseType.stream,
          extra: {...ChannelExporter.secured, stepUpPurposeExtra: purpose},
        ),
        cancelToken: cancel,
      ),
    );
    final file = io.File(path);
    final sink = file.openWrite();
    var written = 0;
    try {
      await for (final chunk in response.data!.stream) {
        if (cancel?.isCancelled ?? false) throw const DataExportBrokeOff();
        sink.add(chunk);
        written += chunk.length;
      }
      await sink.flush();
      await sink.close();
      return written;
    } on Object {
      try {
        await sink.close();
      } on Object {
        // The file is deleted next; a failed close changes nothing.
      }
      if (file.existsSync()) await file.delete();
      throw const DataExportBrokeOff();
    }
  }

  /// Runs [call] and turns its errors into [DataExportRefused]; an error of the login layer
  /// passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw DataExportRefused(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}

/// The export of the own data of the signed-in instance, or null before the login.
@Riverpod(keepAlive: true)
DataExportPort? dataExportPort(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return ApiDataExportPort(
    services.auth.api,
    newIdempotencyKey: services.ids.next,
    clock: ref.watch(appPortsProvider).clock.now,
  );
}

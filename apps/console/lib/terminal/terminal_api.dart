// SPDX-License-Identifier: Apache-2.0

/// The REST operations of the terminal view over the console's API client (brief M6-18 Scope
/// "Client"; docs/api/openapi.yaml `getHost`, `openTerminalSession`, `attachTerminalSession`,
/// `closeTerminalSession`).
library;

import 'package:api_client/api_client.dart';
import 'package:terminal_ui/terminal_ui.dart' show TerminalServerApi;

import '../admin/step_up.dart' show consoleNoStepUpExtra;

/// The reason the console gives `closeTerminalSession` when the person ends a session ("End").
const String consoleTerminalCloseReason = 'Ended in the web console';

/// [TerminalServerApi] over [api], the console's client: the cookie of the same origin, the
/// `X-Console-Request` header on every changing request and no token (08 7.5).
///
/// Opening and attaching carry [consoleNoStepUpExtra], so the console's action step-up never
/// answers their `401 step_up_required`: the terminal controller runs the passkey step-up
/// itself and follows the server (`reason: passkey_required`, 24 6.2), and the console's other
/// step-up ways (password, OIDC) never confirm a terminal (ADR 0070 item 1). Errors reach the
/// controller unchanged as `DioException`s with their problem.
///
/// The console's session REST answer has no device ID, but `identify` needs the device of the
/// session token; so the API records the `session.device_id` of every grant by its
/// `stream_path` ([deviceIdFor]), which `ConsoleTerminalTransport` reads.
///
/// Example:
/// ```dart
/// final api = ConsoleTerminalApi(controller.api, newIdempotencyKey: ids.next);
/// final grant = await api.openSession(hostId, request);
/// api.deviceIdFor(grant.streamPath); // the device of the console session
/// ```
final class ConsoleTerminalApi implements TerminalServerApi {
  /// Creates the API over [api]; [newIdempotencyKey] returns a fresh UUIDv7 for every close.
  ConsoleTerminalApi(this.api, {required this.newIdempotencyKey});

  /// The console's generated client.
  final ApiClient api;

  /// A fresh `Idempotency-Key` per logical operation (06 3).
  final String Function() newIdempotencyKey;

  final Map<String, String> _devices = {};

  static const Map<String, Object?> _ownStepUp = {consoleNoStepUpExtra: true};

  /// The `session.device_id` of the last grant with [streamPath], or null before one.
  String? deviceIdFor(String streamPath) => _devices[streamPath];

  @override
  Future<Host> getHost(String hostId) async =>
      (await api.getHostsApi().getHost(hostId: hostId)).data!;

  @override
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  ) async => _record(
    (await api.getTerminalApi().openTerminalSession(
      hostId: hostId,
      terminalSessionOpen: request,
      extra: _ownStepUp,
    )).data!,
  );

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async => _record(
    (await api.getTerminalApi().attachTerminalSession(
      sessionId: sessionId,
      terminalSessionAttach: request,
      extra: _ownStepUp,
    )).data!,
  );

  @override
  Future<TerminalSession> closeSession(String sessionId) async =>
      (await api.getTerminalApi().closeTerminalSession(
        sessionId: sessionId,
        reasonBody: ReasonBody((b) => b.reason = consoleTerminalCloseReason),
        idempotencyKey: newIdempotencyKey(),
      )).data!;

  @override
  Future<TerminalSession> getSession(String sessionId) async =>
      (await api.getTerminalApi().getTerminalSession(sessionId: sessionId))
          .data!;

  TerminalSessionGrant _record(TerminalSessionGrant grant) {
    _devices[grant.streamPath] = grant.session.deviceId;
    return grant;
  }
}

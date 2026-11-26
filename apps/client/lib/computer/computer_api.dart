// SPDX-License-Identifier: Apache-2.0

/// The REST side of computer use in the apps (docs/spec/27-computer-use.md 11, 12.2; brief
/// CU-07 Scope): the three operations an owner's client calls. There is no operation that turns
/// an enablement on (27 4 item 1) and none that reads an image (R-CL-18): the port offers
/// neither, so no view can ask for one.
library;

import 'package:api_client/api_client.dart' show ApiClient, ComputerSession;

/// The operations of computer use an owner's client calls.
abstract interface class ComputerApi {
  /// The owner's own computer sessions, newest first (`listComputerSessions`,
  /// `computer.session.read`). Throws the `DioException` of a refusal.
  Future<List<ComputerSession>> listSessions();

  /// Stops the session [sessionId] (`stopComputerSession`, `computer.stop`) and returns it as
  /// the server answers, closed. Throws the `DioException` of a refusal.
  Future<ComputerSession> stop(String sessionId);

  /// Switches the enablement of host [hostId] off (`disableComputerDevice`,
  /// `computer.device.disable`). The caller reads the host again through its list; the event
  /// `host.updated` follows as well. Throws the `DioException` of a refusal.
  Future<void> disableDevice(String hostId);
}

/// [ComputerApi] over the generated client.
///
/// [api] is the client of the signed-in session (DPoP and refresh apply; the three actions
/// have no step-up, 07 2.15 item 4). [newIdempotencyKey] returns a fresh UUIDv7 for every
/// write, so a repeated stop or switch-off behaves like the first.
///
/// Example:
/// ```dart
/// final computer = ApiComputerApi(services.auth.api, newIdempotencyKey: services.ids.next);
/// await computer.stop(sessionId);
/// ```
final class ApiComputerApi implements ComputerApi {
  /// Creates the adapter over [api].
  ApiComputerApi(this._api, {required this.newIdempotencyKey});

  final ApiClient _api;

  /// A fresh `Idempotency-Key` per write (06 3).
  final String Function() newIdempotencyKey;

  @override
  Future<List<ComputerSession>> listSessions() async =>
      (await _api.getTerminalApi().listComputerSessions()).data!.items.toList();

  @override
  Future<ComputerSession> stop(String sessionId) async =>
      (await _api.getTerminalApi().stopComputerSession(
        sessionId: sessionId,
        idempotencyKey: newIdempotencyKey(),
      )).data!;

  @override
  Future<void> disableDevice(String hostId) async {
    await _api.getTerminalApi().disableComputerDevice(
      hostId: hostId,
      idempotencyKey: newIdempotencyKey(),
    );
  }
}

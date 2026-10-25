// SPDX-License-Identifier: Apache-2.0

/// The REST side of the terminal view over `api_client` (docs/spec/15-client-architecture.md
/// 23.1; brief M4-36 Scope): the port `TerminalServerApi` of `terminal_ui`, plus the two lists
/// the group "Hosts" of the sidebar reads.
library;

import 'package:api_client/api_client.dart'
    show
        ApiClient,
        Host,
        ReasonBody,
        TerminalSession,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionOpen;
import 'package:terminal_ui/terminal_ui.dart' show TerminalServerApi;

/// The REST operations of the terminal of the app: the port of `terminal_ui` plus the lists the
/// group "Hosts" and the tabs read.
abstract interface class TerminalHostsApi implements TerminalServerApi {
  /// Every host the person sees (`listHosts`), in the server's order (by name).
  Future<List<Host>> listHosts();

  /// Every terminal session the server lists for the person (`listTerminalSessions`), on
  /// [hostId] when given; owners get the sessions of everyone, so the caller keeps the own
  /// ones.
  Future<List<TerminalSession>> listSessions({String? hostId});
}

/// [TerminalHostsApi] over the generated client.
///
/// [api] must be the terminal's client of the session (`InstanceServices.terminalApi`): it
/// carries DPoP and the refresh, but no interceptor answers `401 step_up_required` itself, so
/// the controller runs the step-up after its warnings (D-M4-35-8). Errors come back unchanged
/// (a `DioException` with the `ApiProblem`), as the port asks.
///
/// Example:
/// ```dart
/// final api = ApiTerminalServerApi(services.terminalApi, closeReason: 'Ended in the app');
/// final hosts = await api.listHosts();
/// ```
final class ApiTerminalServerApi implements TerminalHostsApi {
  /// Creates the adapter over [api]; [closeReason] is the audit reason of
  /// `closeTerminalSession`.
  ApiTerminalServerApi(this._api, {required this.closeReason});

  final ApiClient _api;

  /// The `reason` that `closeTerminalSession` sends (05 audit; never terminal content).
  final String closeReason;

  @override
  Future<Host> getHost(String hostId) async =>
      (await _api.getHostsApi().getHost(hostId: hostId)).data!;

  @override
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  ) async => (await _api.getTerminalApi().openTerminalSession(
    hostId: hostId,
    terminalSessionOpen: request,
  )).data!;

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async => (await _api.getTerminalApi().attachTerminalSession(
    sessionId: sessionId,
    terminalSessionAttach: request,
  )).data!;

  @override
  Future<TerminalSession> closeSession(String sessionId) async =>
      (await _api.getTerminalApi().closeTerminalSession(
        sessionId: sessionId,
        reasonBody: ReasonBody((b) => b.reason = closeReason),
      )).data!;

  @override
  Future<List<Host>> listHosts() async {
    final hosts = <Host>[];
    String? cursor;
    do {
      final page = (await _api.getHostsApi().listHosts(cursor: cursor)).data!;
      hosts.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    return hosts;
  }

  @override
  Future<List<TerminalSession>> listSessions({String? hostId}) async {
    final sessions = <TerminalSession>[];
    String? cursor;
    do {
      final page = (await _api.getTerminalApi().listTerminalSessions(
        cursor: cursor,
        hostId: hostId,
      )).data!;
      sessions.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    return sessions;
  }
}

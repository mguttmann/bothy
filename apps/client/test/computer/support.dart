// SPDX-License-Identifier: Apache-2.0

// Fakes of computer use for the app's tests (brief CU-07 Tests): an in-memory [ComputerApi]
// that records every call, and builders of `ComputerSession`, `Host` and the event.

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:client/computer/computer_api.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/sync_engine.dart' show ComputerSessionChanged;

/// The host of the desktop "Desk".
const String deskHost = '01900000-0000-7000-8000-000000000601';

/// The run that controls it.
const String deskRun = '01900000-0000-7000-8000-000000000701';

/// The owner Alex.
const String alexUser = '01900000-0000-7000-8000-000000000001';

/// Session [n].
String computerSessionId(int n) =>
    '01900000-0000-7000-8000-${(0x900 + n).toRadixString(16).padLeft(12, '0')}';

/// A `ComputerSession` of Alex on "Desk".
ComputerSession computerSession(
  int n, {
  ComputerSessionState state = ComputerSessionState.active,
  int actions = 4,
  String? frontApp = 'com.example.editor',
  ComputerCloseReason? closeReason,
  int version = 2,
  String runId = deskRun,
}) => ComputerSession(
  (b) => b
    ..id = computerSessionId(n)
    ..hostId = deskHost
    ..desktop = 'Desk'
    ..userId = alexUser
    ..agentId = '01900000-0000-7000-8000-000000000801'
    ..runId = runId
    ..state = state
    ..pauseReason = state == ComputerSessionState.paused
        ? ComputerSessionPauseReasonEnum.locked
        : null
    ..generation = 3
    ..keyGeneration = 1
    ..openedAt = DateTime.utc(2026, 11, 26, 10)
    ..pausedMs = 0
    ..actionCount = actions
    ..frontApp = frontApp
    ..closeReason = closeReason
    ..version = version,
);

/// The event of [n] in [state].
ComputerSessionChanged computerEvent(
  int n, {
  String state = 'active',
  int actions = 4,
  String? frontApp = 'com.example.editor',
  String? closeReason,
  int version = 3,
  String runId = deskRun,
}) => ComputerSessionChanged(
  sessionId: computerSessionId(n),
  hostId: deskHost,
  runId: runId,
  userId: alexUser,
  desktop: 'Desk',
  state: state,
  version: version,
  actionCount: actions,
  pauseReason: state == 'paused' ? 'locked' : null,
  frontApp: frontApp,
  closeReason: closeReason,
);

/// The `HostComputer` object a host reports.
HostComputer hostComputer({
  bool enabled = true,
  String helper = 'connected',
  bool locked = false,
  HostComputerPermissionsCaptureEnum? capture,
}) => HostComputer(
  (b) => b
    ..helper = HostComputerHelperEnum.valueOf(helper)
    ..enabled = enabled
    ..generation = 3
    ..osAccount = 'alex'
    ..permissions.capture =
        capture ?? HostComputerPermissionsCaptureEnum.granted
    ..permissions.input = HostComputerPermissionsInputEnum.granted
    ..helperVersion = '1.0.0'
    ..signature = HostComputerSignatureEnum.verified
    ..locked = locked,
);

/// A `DioException` with the problem [code] and [status].
DioException problem(int status, String code, {String? reason}) {
  final options = RequestOptions(path: '/api/v1/computer');
  return DioException(
    requestOptions: options,
    response: Response<Object?>(
      requestOptions: options,
      statusCode: status,
      data: {'code': code, 'status': status, 'reason': ?reason},
    ),
    type: DioExceptionType.badResponse,
  );
}

/// An in-memory [ComputerApi]: [sessions] answer the list, [stop] closes one with
/// `stopped_by_user`; [failure] makes the next write throw.
final class FakeComputerApi implements ComputerApi {
  /// The sessions of the list.
  List<ComputerSession> sessions = [];

  /// Thrown by the next write, then cleared.
  Object? failure;

  /// Every call as `list`, `stop <id>` or `disable <host>`.
  final List<String> calls = [];

  /// Completes the next list call when set; the list waits for it.
  Completer<void>? holdList;

  @override
  Future<List<ComputerSession>> listSessions() async {
    calls.add('list');
    await holdList?.future;
    return List.of(sessions);
  }

  @override
  Future<ComputerSession> stop(String sessionId) async {
    calls.add('stop $sessionId');
    if (failure case final f?) {
      failure = null;
      throw f;
    }
    final s = sessions.firstWhere((s) => s.id == sessionId);
    final stopped = s.rebuild(
      (b) => b
        ..state = ComputerSessionState.closed
        ..closeReason = ComputerCloseReason.stoppedByUser
        ..version = s.version + 10,
    );
    sessions = [
      for (final x in sessions)
        if (x.id == sessionId) stopped else x,
    ];
    return stopped;
  }

  @override
  Future<void> disableDevice(String hostId) async {
    calls.add('disable $hostId');
    if (failure case final f?) {
      failure = null;
      throw f;
    }
  }
}

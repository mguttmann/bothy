// SPDX-License-Identifier: Apache-2.0

/// The state of the owner's computer sessions as the views show it (docs/spec/
/// 27-computer-use.md 8, 11; brief CU-07 Scope): metadata only, held in memory, never stored
/// and never an image (R-CL-18).
library;

import 'package:api_client/api_client.dart'
    show ComputerSession, ComputerSessionPauseReasonEnum, ComputerSessionState;
import 'package:flutter/foundation.dart';
import 'package:sync_engine/sync_engine.dart' show ComputerSessionChanged;

/// The state of one computer session (27 8.1).
enum ComputerPhase {
  /// Created, waiting for both sides and the handshake.
  opening,

  /// The agent controls.
  active,

  /// The screen is locked; the session waits (27 8.2).
  paused,

  /// Ended; [ComputerStatus.closeReason] says why.
  closed,
}

/// One computer session of the owner as the chip, the desktop list and the tray show it.
///
/// [frontApp] and [desktop] are personal data of the owner's desktop: [toString] leaves them
/// out, so a log line or a crash report never carries them (R-CL-18).
@immutable
final class ComputerStatus {
  /// Creates the status.
  const ComputerStatus({
    required this.sessionId,
    required this.hostId,
    required this.runId,
    required this.desktop,
    required this.phase,
    required this.actionCount,
    required this.version,
    this.frontApp,
    this.closeReason,
  });

  /// The status of the REST [session] (`listComputerSessions`, `stopComputerSession`).
  factory ComputerStatus.fromSession(ComputerSession session) => ComputerStatus(
    sessionId: session.id,
    hostId: session.hostId,
    runId: session.runId,
    desktop: session.desktop,
    phase: switch (session.state) {
      ComputerSessionState.opening => ComputerPhase.opening,
      ComputerSessionState.paused => ComputerPhase.paused,
      ComputerSessionState.closed => ComputerPhase.closed,
      // A locked pause reported without the state still waits.
      _ when session.pauseReason == ComputerSessionPauseReasonEnum.locked =>
        ComputerPhase.paused,
      _ => ComputerPhase.active,
    },
    actionCount: session.actionCount,
    version: session.version,
    frontApp: _nonEmpty(session.frontApp),
    closeReason: switch (session.closeReason) {
      final r? => _snake(r.name),
      null => null,
    },
  );

  /// The status of the event [e] (`computer_session.changed`, 27 12.4).
  factory ComputerStatus.fromEvent(ComputerSessionChanged e) => ComputerStatus(
    sessionId: e.sessionId,
    hostId: e.hostId,
    runId: e.runId,
    desktop: e.desktop,
    phase: switch (e.state) {
      'opening' => ComputerPhase.opening,
      'paused' => ComputerPhase.paused,
      'closed' => ComputerPhase.closed,
      _ => ComputerPhase.active,
    },
    actionCount: e.actionCount,
    version: e.version,
    frontApp: _nonEmpty(e.frontApp),
    closeReason: e.closeReason,
  );

  /// The session.
  final String sessionId;

  /// The desktop's host.
  final String hostId;

  /// The run that controls the desktop.
  final String runId;

  /// The desktop's name (`hosts.name`).
  final String desktop;

  /// The state.
  final ComputerPhase phase;

  /// The count of actions so far.
  final int actionCount;

  /// The session's version; a lower or equal one never replaces a known status.
  final int version;

  /// The identity of the frontmost app, or null before the helper named one.
  final String? frontApp;

  /// The `close_reason` code of a closed session (27 8.3), else null.
  final String? closeReason;

  /// Whether the session ended.
  bool get closed => phase == ComputerPhase.closed;

  static String? _nonEmpty(String? s) => s == null || s.isEmpty ? null : s;

  /// The wire code of a generated enum's Dart name: `stoppedByUser` is `stopped_by_user`
  /// (the generator turns every code of 27 8.3 into lower camel case).
  static String _snake(String name) =>
      name.replaceAllMapped(RegExp('[A-Z]'), (m) => '_${m[0]!.toLowerCase()}');

  @override
  bool operator ==(Object other) =>
      other is ComputerStatus &&
      other.sessionId == sessionId &&
      other.hostId == hostId &&
      other.runId == runId &&
      other.desktop == desktop &&
      other.phase == phase &&
      other.actionCount == actionCount &&
      other.version == version &&
      other.frontApp == frontApp &&
      other.closeReason == closeReason;

  @override
  int get hashCode => Object.hash(
    sessionId,
    hostId,
    runId,
    desktop,
    phase,
    actionCount,
    version,
    frontApp,
    closeReason,
  );

  // Neither the desktop's name nor the frontmost app (R-CL-18).
  @override
  String toString() =>
      'ComputerStatus($sessionId, ${phase.name}, v$version, $closeReason)';
}

/// The owner's computer sessions: the [live] ones (not closed) and the [ended] ones this
/// device saw end, each shown once until the owner dismisses it (27 11 item 1).
@immutable
final class ComputerSessionsState {
  /// Creates the state.
  const ComputerSessionsState({this.live = const {}, this.ended = const {}});

  /// Nothing: before the login, for a person who is no owner, or before the first load.
  static const ComputerSessionsState none = ComputerSessionsState();

  /// The sessions that are not closed, by session ID.
  final Map<String, ComputerStatus> live;

  /// The sessions that ended while this device knew them live, by session ID, until dismissed.
  final Map<String, ComputerStatus> ended;

  /// The status of the run [runId]: its live session, else its ended one, else null.
  ComputerStatus? ofRun(String runId) =>
      live.values.where((s) => s.runId == runId).firstOrNull ??
      ended.values.where((s) => s.runId == runId).firstOrNull;

  /// The live session on host [hostId], or null (one controller per host, 27 8.5).
  ComputerStatus? liveOn(String hostId) =>
      live.values.where((s) => s.hostId == hostId).firstOrNull;

  /// The known status of [sessionId], live or ended.
  ComputerStatus? known(String sessionId) =>
      live[sessionId] ?? ended[sessionId];

  @override
  bool operator ==(Object other) =>
      other is ComputerSessionsState &&
      mapEquals(other.live, live) &&
      mapEquals(other.ended, ended);

  @override
  int get hashCode => Object.hash(
    Object.hashAllUnordered(
      live.entries.map((e) => Object.hash(e.key, e.value)),
    ),
    Object.hashAllUnordered(
      ended.entries.map((e) => Object.hash(e.key, e.value)),
    ),
  );
}

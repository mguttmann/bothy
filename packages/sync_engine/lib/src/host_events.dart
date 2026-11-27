// SPDX-License-Identifier: Apache-2.0

/// The changes of hosts and of the person's terminal and computer sessions (docs/spec/
/// 04-sync-protocol.md 2.1 and 8.2; docs/spec/15-client-architecture.md 23.1; docs/spec/
/// 27-computer-use.md 12.4; briefs M4-36 and CU-07 Scope): the dispatcher turns `host.updated`,
/// `terminal_session.changed` and `computer_session.changed` (durable), `host.presence_changed`
/// (ephemeral) and the `access.changed` entries of kind `host` into [HostEvent]s of the
/// [HostEventsHub]. Nothing of them is stored: the app reloads hosts and sessions through REST
/// (D-M4-36). The terminal stream itself never runs through the engine (15 8).
library;

import 'dart:async';

/// One change of a host or of a terminal session, without content: IDs, versions and states.
sealed class HostEvent {
  const HostEvent();

  /// The host the change belongs to.
  String get hostId;
}

/// `host.updated`: the host changed (state, keys, profiles, assignments, connection).
final class HostChanged extends HostEvent {
  /// Creates the change.
  const HostChanged({
    required this.hostId,
    required this.version,
    required this.state,
    required this.connected,
    this.keyChanged = false,
  });

  @override
  final String hostId;

  /// `resource.version` of the event (the host's `version`).
  final int version;

  /// The host's `state` (`pairing`, `confirming`, `active`, `disabled`, `retired`).
  final String state;

  /// The host's `connected`.
  final bool connected;

  /// `key_changed`: true after a re-pairing (24 3.3).
  final bool keyChanged;

  @override
  bool operator ==(Object other) =>
      other is HostChanged &&
      other.hostId == hostId &&
      other.version == version &&
      other.state == state &&
      other.connected == connected &&
      other.keyChanged == keyChanged;

  @override
  int get hashCode =>
      Object.hash(hostId, version, state, connected, keyChanged);

  @override
  String toString() =>
      'HostChanged($hostId, v$version, $state, connected: $connected, '
      'keyChanged: $keyChanged)';
}

/// `host.presence_changed`: the control connection of the host opened or closed.
final class HostPresenceChanged extends HostEvent {
  /// Creates the change.
  const HostPresenceChanged({required this.hostId, required this.connected});

  @override
  final String hostId;

  /// Whether the host is connected now.
  final bool connected;

  @override
  bool operator ==(Object other) =>
      other is HostPresenceChanged &&
      other.hostId == hostId &&
      other.connected == connected;

  @override
  int get hashCode => Object.hash(hostId, connected);

  @override
  String toString() => 'HostPresenceChanged($hostId, connected: $connected)';
}

/// `terminal_session.changed`: a terminal session of the person changed (24 10.2).
final class TerminalSessionChanged extends HostEvent {
  /// Creates the change.
  const TerminalSessionChanged({
    required this.sessionId,
    required this.hostId,
    required this.userId,
    required this.deviceId,
    required this.state,
    required this.version,
    this.closeReason,
  });

  /// The session.
  final String sessionId;

  @override
  final String hostId;

  /// The person of the session.
  final String userId;

  /// The device the session is bound to now.
  final String deviceId;

  /// The session's `state` (`opening`, `active`, `detached`, `closed`).
  final String state;

  /// `resource.version` of the event.
  final int version;

  /// The `close_reason` of a closed session (24 10.3), else null.
  final String? closeReason;

  /// Whether the session is closed.
  bool get closed => state == 'closed';

  @override
  bool operator ==(Object other) =>
      other is TerminalSessionChanged &&
      other.sessionId == sessionId &&
      other.hostId == hostId &&
      other.userId == userId &&
      other.deviceId == deviceId &&
      other.state == state &&
      other.version == version &&
      other.closeReason == closeReason;

  @override
  int get hashCode => Object.hash(
    sessionId,
    hostId,
    userId,
    deviceId,
    state,
    version,
    closeReason,
  );

  @override
  String toString() =>
      'TerminalSessionChanged($sessionId on $hostId, $state, v$version, '
      'reason: $closeReason)';
}

/// `computer_session.changed`: a computer session of the owner changed (docs/spec/
/// 27-computer-use.md 8, 12.4). Metadata only, never an image (R-CL-18).
///
/// [frontApp] names an app of the owner's desktop: it is personal data, so [toString] leaves
/// it out and nothing of this event goes into a log line, a crash report or a metric.
final class ComputerSessionChanged extends HostEvent {
  /// Creates the change.
  const ComputerSessionChanged({
    required this.sessionId,
    required this.hostId,
    required this.runId,
    required this.userId,
    required this.desktop,
    required this.state,
    required this.version,
    required this.actionCount,
    this.pauseReason,
    this.frontApp,
    this.closeReason,
    this.closeDetail,
  });

  /// The session.
  final String sessionId;

  @override
  final String hostId;

  /// The run that controls the desktop.
  final String runId;

  /// The owner who triggered the run.
  final String userId;

  /// The name of the desktop (`hosts.name`).
  final String desktop;

  /// The session's `state` (`opening`, `active`, `paused`, `closed`).
  final String state;

  /// `resource.version` of the event.
  final int version;

  /// The count of actions so far.
  final int actionCount;

  /// `locked` while paused, else null.
  final String? pauseReason;

  /// The identity of the frontmost app (bundle ID or executable name), or null.
  final String? frontApp;

  /// The `close_reason` of a closed session (27 8.3), else null.
  final String? closeReason;

  /// The `close_detail` of a closed session, else null.
  final String? closeDetail;

  /// Whether the session is closed.
  bool get closed => state == 'closed';

  @override
  bool operator ==(Object other) =>
      other is ComputerSessionChanged &&
      other.sessionId == sessionId &&
      other.hostId == hostId &&
      other.runId == runId &&
      other.userId == userId &&
      other.desktop == desktop &&
      other.state == state &&
      other.version == version &&
      other.actionCount == actionCount &&
      other.pauseReason == pauseReason &&
      other.frontApp == frontApp &&
      other.closeReason == closeReason &&
      other.closeDetail == closeDetail;

  @override
  int get hashCode => Object.hash(
    sessionId,
    hostId,
    runId,
    userId,
    desktop,
    state,
    version,
    actionCount,
    pauseReason,
    frontApp,
    closeReason,
    closeDetail,
  );

  // Neither the desktop's name nor the frontmost app: both are personal (R-CL-18).
  @override
  String toString() =>
      'ComputerSessionChanged($sessionId on $hostId, $state, v$version, '
      'reason: $closeReason)';
}

/// An `access.changed` entry of kind `host` in `removed`: the person lost the capability
/// `terminal` on the host; the server ends their sessions there (04 8.2, 24 8.3).
final class HostRemoved extends HostEvent {
  /// Creates the signal.
  const HostRemoved({required this.hostId});

  @override
  final String hostId;

  @override
  bool operator ==(Object other) =>
      other is HostRemoved && other.hostId == hostId;

  @override
  int get hashCode => hostId.hashCode;

  @override
  String toString() => 'HostRemoved($hostId)';
}

/// An `access.changed` entry of kind `host` in `added` or `changed`: the person may see or
/// use the host now; the app reloads its hosts.
final class HostAccessGranted extends HostEvent {
  /// Creates the signal.
  const HostAccessGranted({required this.hostId});

  @override
  final String hostId;

  @override
  bool operator ==(Object other) =>
      other is HostAccessGranted && other.hostId == hostId;

  @override
  int get hashCode => hostId.hashCode;

  @override
  String toString() => 'HostAccessGranted($hostId)';
}

/// The stream of [HostEvent]s of one engine.
///
/// The dispatcher adds a durable change after the commit of its page, so a page that rolls back
/// announces nothing; an ephemeral change comes at once. Events after [close] are dropped.
///
/// Example:
/// ```dart
/// engine.hostEvents.listen((e) {
///   if (e is TerminalSessionChanged && e.closed) showEnded(e.sessionId, e.closeReason);
/// });
/// ```
final class HostEventsHub {
  final StreamController<HostEvent> _events =
      StreamController<HostEvent>.broadcast(sync: true);

  /// Every change, in the order the server sent them.
  Stream<HostEvent> get events => _events.stream;

  /// Announces [event]; does nothing after [close].
  void add(HostEvent event) {
    if (!_events.isClosed) _events.add(event);
  }

  /// Closes the stream.
  Future<void> close() => _events.close();
}

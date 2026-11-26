// SPDX-License-Identifier: Apache-2.0

/// Appliers of the durable host events (04 2.1; briefs M4-36 and CU-07 Scope): `host.updated`,
/// `terminal_session.changed` and `computer_session.changed` write no table (the app reloads hosts and sessions through REST)
/// and announce their change on the [HostEventsHub] after the commit of the page, so a page
/// that rolls back announces nothing.
library;

import 'package:domain/domain.dart' show HostId;

import '../dispatcher.dart';
import '../host_events.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> hostAppliers = {
  'host.updated': _hostUpdated,
  'terminal_session.changed': _terminalSessionChanged,
  'computer_session.changed': _computerSessionChanged,
};

/// `computer_session.changed` (27 12.4): no table either; the client keeps the sessions of the
/// owner in memory (R-CL-18).
Future<void> _computerSessionChanged(ApplyContext c) async {
  final s = c.payload;
  final event = ComputerSessionChanged(
    sessionId: str(s, 'id'),
    hostId: HostId.parse(str(s, 'host_id')).value,
    runId: str(s, 'run_id'),
    userId: str(s, 'user_id'),
    desktop: str(s, 'desktop'),
    state: str(s, 'state'),
    version: c.resourceVersion!.value,
    actionCount: integer(s, 'action_count'),
    pauseReason: optStr(s, 'pause_reason'),
    frontApp: optStr(s, 'front_app'),
    closeReason: optStr(s, 'close_reason'),
    closeDetail: optStr(s, 'close_detail'),
  );
  c.after.steps.add(() => c.sync.hosts.add(event));
}

Future<void> _hostUpdated(ApplyContext c) async {
  final host = obj(c.payload, 'host');
  final event = HostChanged(
    hostId: HostId.parse(str(host, 'id')).value,
    // The catalogue marks the type versioned; the dispatcher skipped it without a version.
    version: c.resourceVersion!.value,
    state: str(host, 'state'),
    connected: flag(host, 'connected'),
    keyChanged: flag(c.payload, 'key_changed'),
  );
  c.after.steps.add(() => c.sync.hosts.add(event));
}

Future<void> _terminalSessionChanged(ApplyContext c) async {
  final s = c.payload;
  final event = TerminalSessionChanged(
    sessionId: str(s, 'id'),
    hostId: HostId.parse(str(s, 'host_id')).value,
    userId: str(s, 'user_id'),
    deviceId: str(s, 'device_id'),
    state: str(s, 'state'),
    version: c.resourceVersion!.value,
    closeReason: optStr(s, 'close_reason'),
  );
  c.after.steps.add(() => c.sync.hosts.add(event));
}

/// `host.presence_changed` (ephemeral): the hub hears it at once. Throws a [FormatException]
/// for a malformed payload; the dispatcher logs it without content.
void applyHostPresence(SyncContext sync, Json payload) {
  sync.hosts.add(
    HostPresenceChanged(
      hostId: HostId.parse(str(payload, 'host_id')).value,
      connected: flag(payload, 'connected'),
    ),
  );
}

/// The `access.changed` entries of kind `host` (04 8.2): a removed host becomes [HostRemoved],
/// an added or changed one [HostAccessGranted], announced after the commit of the page.
void announceHostAccess(ApplyContext c) {
  final p = c.payload;
  final events = <HostEvent>[
    for (final t in objList(p, 'removed'))
      if (str(t, 'kind') == 'host') HostRemoved(hostId: str(t, 'id')),
    for (final key in const ['added', 'changed'])
      for (final t in objList(p, key))
        if (str(t, 'kind') == 'host') HostAccessGranted(hostId: str(t, 'id')),
  ];
  if (events.isEmpty) return;
  c.after.steps.add(() => events.forEach(c.sync.hosts.add));
}

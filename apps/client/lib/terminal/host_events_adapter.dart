// SPDX-License-Identifier: Apache-2.0

/// The host events of the terminal from `sync_engine` (docs/spec/15-client-architecture.md
/// 23.1; brief M4-36 Scope): the port `HostEvents` of `terminal_ui` over
/// `SyncEngine.hostEvents`.
library;

import 'package:sync_engine/sync_engine.dart' as sync;
import 'package:terminal_ui/terminal_ui.dart';

/// [HostEvents] over the hub of the engine.
///
/// `host.updated`, `host.presence_changed` and a lost access to the host become
/// [HostUpdated]; `terminal_session.changed` becomes [TerminalSessionUpdated], whose `closed`
/// with the `close_reason` ends the open tab like `4100` (15 23.2 item 8). The stream of the
/// terminal never runs through the engine (15 8).
///
/// Example:
/// ```dart
/// final events = EngineHostEvents(engine.hostEvents);
/// events.watch(hostId).listen(print);
/// ```
final class EngineHostEvents implements HostEvents {
  /// Creates the port over [events].
  EngineHostEvents(this._events);

  final Stream<sync.HostEvent> _events;

  @override
  Stream<HostChange> watch(String hostId) => _events
      .where((e) => e.hostId == hostId)
      .map(
        (e) => switch (e) {
          sync.TerminalSessionChanged() => TerminalSessionUpdated(
            e.sessionId,
            closed: e.closed,
            closeReason: e.closeReason,
          ),
          _ => HostUpdated(e.hostId),
        },
      );
}

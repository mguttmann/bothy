// SPDX-License-Identifier: Apache-2.0

/// The changes of the status page (docs/spec/16-operations.md 12; brief M7-21 Design note 1):
/// the dispatcher turns the durable event `status.updated` into a [StatusUpdated] of the
/// [StatusEventsHub] after the commit of its page. Nothing of it is stored: the status view of
/// the app reads `getStatus` again on each change, because the payload names only the overall
/// state and the IDs of what changed.
library;

import 'dart:async';

/// `status.updated`: a component, its state or an incident of the status page changed.
final class StatusUpdated {
  /// Creates the change.
  const StatusUpdated({required this.overallState});

  /// `overall_state` of the payload (`operational`, `maintenance`, `degraded`,
  /// `partial_outage`, `major_outage`), as the server sent it.
  final String overallState;

  @override
  bool operator ==(Object other) =>
      other is StatusUpdated && other.overallState == overallState;

  @override
  int get hashCode => overallState.hashCode;

  @override
  String toString() => 'StatusUpdated($overallState)';
}

/// The stream of [StatusUpdated]s of one engine.
///
/// The dispatcher adds a change after the commit of its page, so a page that rolls back
/// announces nothing. Changes after [close] are dropped.
///
/// Example:
/// ```dart
/// engine.statusUpdates.listen((_) => reloadStatus());
/// ```
final class StatusEventsHub {
  final StreamController<StatusUpdated> _events =
      StreamController<StatusUpdated>.broadcast(sync: true);

  /// Every change, in the order the server sent them.
  Stream<StatusUpdated> get events => _events.stream;

  /// Announces [event]; does nothing after [close].
  void add(StatusUpdated event) {
    if (!_events.isClosed) _events.add(event);
  }

  /// Closes the stream.
  Future<void> close() => _events.close();
}

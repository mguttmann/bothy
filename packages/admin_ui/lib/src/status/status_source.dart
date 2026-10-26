// SPDX-License-Identifier: Apache-2.0

/// Where the status view gets its page and its changes from (brief M7-21 Design note 1,
/// docs/decisions.d/M7-21.md D-M7-21-1): the [StatusSource] port keeps the widgets the same on
/// every surface. The apps implement it with `getStatus`, the event `status.updated` of the
/// sync engine and the engine's connection state ([LiveStatusSource]); the console, which uses
/// neither the sync engine nor a local database (15 3.1), with `getStatus` on an interval
/// ([PollingStatusSource]).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;

import '../data/self_repository.dart' show SelfActionFailed;

/// The port of the status view.
///
/// Example:
/// ```dart
/// final source = PollingStatusSource(loader: repository.status, interval: config.statusPollInterval);
/// ```
abstract interface class StatusSource {
  /// Reads the page (`getStatus` or `getPublicStatus`).
  ///
  /// Errors: [SelfActionFailed]; without a problem when no answer came, which the view shows
  /// as "Server unreachable".
  Future<api.StatusPage> load();

  /// One element whenever the page may have changed; the view loads it again.
  Stream<void> get changes;

  /// Whether the connection state reports the server unreachable (true) or reachable again
  /// (false); a source without a connection state never emits.
  Stream<bool> get unreachable;
}

/// A [StatusSource] of the apps: [load] reads `getStatus`, [changes] are the `status.updated`
/// events of the sync engine, [unreachable] follows the engine's connection state.
///
/// Example:
/// ```dart
/// LiveStatusSource(
///   loader: repository.status,
///   changes: engine.statusUpdates,
///   unreachable: engine.states.map(serverUnreachable),
/// );
/// ```
final class LiveStatusSource implements StatusSource {
  /// Creates the source.
  LiveStatusSource({
    required this.loader,
    required this.changes,
    required this.unreachable,
  });

  /// Reads the page.
  final Future<api.StatusPage> Function() loader;

  @override
  Future<api.StatusPage> load() => loader();

  @override
  final Stream<void> changes;

  @override
  final Stream<bool> unreachable;
}

/// A [StatusSource] that asks again every [interval]: the console's (brief M7-21 requirement
/// 12), where [interval] is the named client value `client.status_poll_interval` of
/// `AppConfig` (D-W-115). It reports no connection state; a [load] without an answer counts
/// as unreachable in the view.
final class PollingStatusSource implements StatusSource {
  /// Creates the source.
  PollingStatusSource({required this.loader, required this.interval});

  /// Reads the page.
  final Future<api.StatusPage> Function() loader;

  /// The time between two reads.
  final Duration interval;

  @override
  Future<api.StatusPage> load() => loader();

  @override
  Stream<void> get changes => Stream<void>.periodic(interval);

  @override
  Stream<bool> get unreachable => const Stream.empty();
}

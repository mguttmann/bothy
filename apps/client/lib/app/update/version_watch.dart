// SPDX-License-Identifier: Apache-2.0

/// The version gate of one instance (docs/spec/18-release.md 9 item 2, R-RL-08; brief M7-13
/// Design note 1).
library;

import 'dart:async';

import 'package:domain/domain.dart'
    show ClientVersionAllowed, ClientVersionBelowMinimum, ClientVersionDecision;

import '../discovery.dart';

/// Holds the last decision of the version gate for the instance at [address] and asks
/// `getDiscovery` again before every gateway attempt ([admit], the `ConnectGate` of the sync
/// engine).
///
/// The decision of the start (E1's discovery) is the [initial] one; every answer of [admit]
/// replaces it and is published on [changes] when it differs. Without a usable answer the last
/// decision stays in force, so an app the gate admitted connects and the server stays the
/// second line with `4007`.
///
/// Example: `SyncEngine.create(..., connectGate: watch.admit)`.
final class VersionWatch {
  /// Creates the watch.
  VersionWatch({
    required this._discovery,
    required this.address,
    ClientVersionDecision initial = const ClientVersionAllowed(),
  }) : _current = initial;

  final DiscoveryClient _discovery;

  /// The origin of the instance.
  final Uri address;

  ClientVersionDecision _current;
  final StreamController<ClientVersionDecision> _changes =
      StreamController<ClientVersionDecision>.broadcast(sync: true);

  /// The last decision.
  ClientVersionDecision get current => _current;

  /// Every new decision (broadcast).
  Stream<ClientVersionDecision> get changes => _changes.stream;

  /// Asks `getDiscovery` and returns whether the app may connect: false only when this build
  /// is below `client.min_version.<platform>`.
  Future<bool> admit() async {
    final decision = await _discovery.checkVersion(address);
    if (decision != null && decision != _current) {
      _current = decision;
      if (!_changes.isClosed) _changes.add(decision);
    }
    return _current is! ClientVersionBelowMinimum;
  }

  /// Closes [changes].
  Future<void> dispose() => _changes.close();
}

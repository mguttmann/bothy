// SPDX-License-Identifier: Apache-2.0

/// The network status of the desktop app without a plugin (docs/DECISIONS.md D-M4-18-8).
library;

import 'dart:async';

import 'package:sync_engine/sync_engine.dart';

/// [NetworkStatus] derived from the app lifecycle instead of `connectivity_plus`.
///
/// The gateway connection already notices a lost network by its failed connects and its
/// heartbeat and retries with backoff (04 6.5); what a network plugin adds is the hint "the
/// network may be back, retry now". On the desktop that hint comes from the return to the
/// foreground (a laptop that wakes up, a window that comes back), so [online] stays true and
/// [changes] emits `true` on every return to the foreground.
final class LifecycleNetworkStatus implements NetworkStatus {
  /// Creates the status over [lifecycle].
  LifecycleNetworkStatus(AppLifecycle lifecycle) {
    _subscription = lifecycle.changes.listen((activity) {
      if (activity == AppActivity.foreground) _changes.add(true);
    });
  }

  late final StreamSubscription<AppActivity> _subscription;
  final StreamController<bool> _changes = StreamController<bool>.broadcast();

  @override
  Stream<bool> get changes => _changes.stream;

  @override
  bool get online => true;

  /// Stops listening.
  Future<void> dispose() async {
    await _subscription.cancel();
    await _changes.close();
  }
}

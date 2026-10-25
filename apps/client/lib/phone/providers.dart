// SPDX-License-Identifier: Apache-2.0

/// The providers of the phone shell (brief M5-02 Design note 2).
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';

part 'providers.g.dart';

/// The number of outbox entries that are not final, for the offline banner (01 5.3 M7:
/// "Offline. 1 message waiting"); 0 while signed out.
@Riverpod(keepAlive: true)
Stream<int> outboxWaiting(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? Stream.value(0) : engine.outbox.depth;
}

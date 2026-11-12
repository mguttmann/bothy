// SPDX-License-Identifier: Apache-2.0

/// The providers of the phone shell (brief M5-02 Design note 2).
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../files/providers.dart';
import 'compose/media_picker.dart';

part 'providers.g.dart';

/// The number of outbox entries that are not final, for the offline banner (01 5.3 M7:
/// "Offline. 1 message waiting"); 0 while signed out.
@Riverpod(keepAlive: true)
Stream<int> outboxWaiting(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? Stream.value(0) : engine.outbox.depth;
}

/// Deletes the photos of a pick during which Android ended the app (brief M5-07 requirement
/// 16; `MediaPicker.discardLost`), once per start of an Android build: the phone shell reads it
/// when it first shows. Every other build completes without asking.
@Riverpod(keepAlive: true)
Future<void> lostPhotosDiscarded(Ref ref) async {
  if (ref.watch(appPortsProvider).platform != 'android') return;
  final files = ref.watch(filePortsProvider);
  await MediaPicker(
    port: files.media(),
    scratch: files.temporary,
  ).discardLost();
}

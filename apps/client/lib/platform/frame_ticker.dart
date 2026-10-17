// SPDX-License-Identifier: Apache-2.0

/// The frame pacing of the run documents (docs/spec/15-client-architektur.md 7.4).
library;

import 'package:flutter/scheduler.dart';
import 'package:sync_engine/sync_engine.dart';

/// [FrameTicker] on the [SchedulerBinding]: the run store applies queued patches once per
/// frame (04 11.3 item 4).
final class SchedulerFrameTicker implements FrameTicker {
  /// Creates the ticker; [binding] defaults to the app's binding.
  SchedulerFrameTicker({SchedulerBinding? binding})
    : _binding = binding; // ignore: prefer_initializing_formals

  final SchedulerBinding? _binding;

  @override
  void requestFrame(void Function() onFrame) {
    final binding = _binding ?? SchedulerBinding.instance;
    binding
      ..scheduleFrameCallback((_) => onFrame())
      ..scheduleFrame();
  }
}

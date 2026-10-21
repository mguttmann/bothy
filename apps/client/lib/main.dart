// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/scheduler.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/app.dart';
import 'app/bootstrap.dart';
import 'app/providers/ports.dart';
import 'telemetry/measurements.dart';

/// Starts the desktop app on macOS and Windows with the ports of the platform
/// ([productionPorts]); the integration tests start it from
/// `integration_test/support/test_app.dart` instead.
///
/// The first line starts the cold start of `client.cold_start.duration` (brief M4-31); the
/// frame timings of the engine count `client.frames` from the first frame on.
void main() {
  appStartWatch.elapsed;
  WidgetsFlutterBinding.ensureInitialized();
  final ports = productionPorts();
  ports.measurements.attachFrameTimings(SchedulerBinding.instance);
  runApp(
    ProviderScope(
      overrides: [appPortsProvider.overrideWithValue(ports)],
      child: const ClientApp(),
    ),
  );
}

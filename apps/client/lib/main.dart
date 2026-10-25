// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/scheduler.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'app/app.dart';
import 'app/bootstrap.dart';
import 'app/providers/ports.dart';
import 'platform/android_platform.dart';
import 'platform/notification_channels.dart';
import 'telemetry/measurements.dart';

/// Starts the app on macOS, Windows, iOS and Android with the ports of the platform
/// ([productionPorts]); the integration tests start it from
/// `integration_test/support/test_app.dart` instead.
///
/// The first line starts the cold start of `client.cold_start.duration` (brief M4-31); the
/// frame timings of the engine count `client.frames` from the first frame on. On Android the
/// app first reads the device's name and passkey support from the runner and registers the
/// notification channels in the device language, so they are in the system settings before
/// the first push (brief M5-03 Design note 4).
Future<void> main() async {
  appStartWatch.elapsed;
  WidgetsFlutterBinding.ensureInitialized();
  AndroidRunnerInfo? android;
  if (Platform.isAndroid) {
    final api = AndroidPlatformApi();
    android = await AndroidRunnerInfo.load(api);
    // A failed registration is retried at the next start; it never stops the app.
    unawaited(
      registerNotificationChannels(
        api,
        lookupAppLocalizations(
          resolveAppLocale(
            systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
          ),
        ),
      ).then<void>((_) {}, onError: (Object _) {}),
    );
  }
  final ports = productionPorts(android: android);
  ports.measurements.attachFrameTimings(SchedulerBinding.instance);
  runApp(
    ProviderScope(
      overrides: [appPortsProvider.overrideWithValue(ports)],
      child: const ClientApp(),
    ),
  );
}

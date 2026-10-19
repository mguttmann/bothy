// SPDX-License-Identifier: Apache-2.0

/// The telemetry of the app as providers (docs/spec/15-client-architektur.md 16 and 17; ADR
/// 0057; brief M4-31 Design notes 2 and 4).
///
/// [appMeasurementsProvider] hands the measurement points to the widgets, [metricsSenderProvider]
/// runs the sender while a person is signed in, and [appTelemetryProvider] keeps both in step
/// with `privacy.client_metrics` and `privacy.crash_reports`; the app widget watches it.
library;

import 'dart:async';
import 'dart:ui' show Brightness, PlatformDispatcher;

import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/providers/settings.dart';
import '../settings/settings_providers.dart';
import 'client_metrics.dart';
import 'crash_reports.dart';
import 'measurements.dart';
import 'metrics_sender.dart';

part 'telemetry_providers.g.dart';

/// The measurement points of the running app (one collector per process).
@Riverpod(keepAlive: true)
AppMeasurements appMeasurements(Ref ref) =>
    ref.watch(appPortsProvider).measurements;

/// The crash path of the build ([CrashReports] over the define `CRASH_DSN`).
@Riverpod(keepAlive: true)
CrashReports crashReports(Ref ref) => ref.watch(appPortsProvider).crashReports;

/// Whether the personal settings of the signed-in person are read from the local database.
/// Before that `personalSettingsProvider` holds the defaults, which would start crash reports
/// and send metrics of a person who turned them off (D-M4-31-5).
@Riverpod(keepAlive: true)
bool personalSettingsLoaded(Ref ref) {
  if (ref.watch(localDatabaseProvider) == null) return false;
  final stored = ref.watch(userSettingsProvider);
  return stored.hasValue && !stored.isLoading;
}

/// `privacy.client_metrics` of the signed-in person, false while it is not loaded.
@Riverpod(keepAlive: true)
bool clientMetricsOn(Ref ref) =>
    ref.watch(personalSettingsLoadedProvider) &&
    ref.watch(personalSettingsProvider.select((s) => s.clientMetrics));

/// `privacy.crash_reports` of the signed-in person, false while it is not loaded: crash
/// reports start only after the login, with the person's own choice (D-M4-31-5).
@Riverpod(keepAlive: true)
bool crashReportsOn(Ref ref) =>
    ref.watch(personalSettingsLoadedProvider) &&
    ref.watch(personalSettingsProvider.select((s) => s.crashReports));

/// The server port of the sender: the REST client of the signed-in session, or null before
/// the login.
@Riverpod(keepAlive: true)
ClientMetricsApi? clientMetricsApi(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : RestClientMetricsApi(services.auth.api);
}

/// The sender of the signed-in session, started; null before the login. A sign-out disposes it,
/// which sends what is left while the session still exists.
@Riverpod(keepAlive: true)
MetricsSender? metricsSender(Ref ref) {
  final api = ref.watch(clientMetricsApiProvider);
  if (api == null) return null;
  final ports = ref.watch(appPortsProvider);
  // Kept current by a listener, because the last flush runs while this provider is disposed,
  // where a read is not allowed.
  late PersonalSettings settings;
  late bool on;
  ref
    ..listen(
      personalSettingsProvider,
      (_, next) => settings = next,
      fireImmediately: true,
    )
    ..listen(
      clientMetricsOnProvider,
      (_, next) => on = next,
      fireImmediately: true,
    );
  final sender = MetricsSender(
    metrics: ref.watch(appMeasurementsProvider).metrics,
    api: api,
    interval: ports.config.metricsFlushInterval,
    enabled: () => on,
    attributes: () {
      return MetricsAttributes(
        platform: ports.platform,
        appVersion: ports.branding.appVersion,
        density: settings.density,
        theme: switch (settings.theme) {
          'light' || 'dark' => settings.theme,
          _ =>
            PlatformDispatcher.instance.platformBrightness == Brightness.dark
                ? 'dark'
                : 'light',
        },
      );
    },
  )..start();
  ref.onDispose(() => unawaited(sender.stop()));
  return sender;
}

/// Keeps the telemetry in step with the settings: the sender runs while signed in and resumes
/// after a change of `privacy.client_metrics`, the crash path starts or stops with
/// `privacy.crash_reports` and tags reports with the hashed instance ID.
@Riverpod(keepAlive: true)
void appTelemetry(Ref ref) {
  ref.watch(metricsSenderProvider);
  final crashes = ref.watch(crashReportsProvider);
  ref
    ..listen(
      clientMetricsOnProvider,
      (_, _) => ref.read(metricsSenderProvider)?.settingChanged(),
    )
    ..listen(
      crashReportsOnProvider,
      (_, on) => unawaited(crashes.apply(enabled: on)),
      fireImmediately: true,
    )
    ..listen(currentInstanceProvider, (_, instance) {
      if (instance != null) crashes.setInstance(instance.instanceId);
    }, fireImmediately: true);
}

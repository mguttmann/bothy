// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'telemetry_providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The measurement points of the running app (one collector per process).

@ProviderFor(appMeasurements)
final appMeasurementsProvider = AppMeasurementsProvider._();

/// The measurement points of the running app (one collector per process).

final class AppMeasurementsProvider
    extends
        $FunctionalProvider<AppMeasurements, AppMeasurements, AppMeasurements>
    with $Provider<AppMeasurements> {
  /// The measurement points of the running app (one collector per process).
  AppMeasurementsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appMeasurementsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appMeasurementsHash();

  @$internal
  @override
  $ProviderElement<AppMeasurements> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AppMeasurements create(Ref ref) {
    return appMeasurements(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AppMeasurements value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AppMeasurements>(value),
    );
  }
}

String _$appMeasurementsHash() => r'5871c3fabcb5aa24856a6e692858ebee8d4b1f90';

/// The crash path of the build ([CrashReports] over the define `CRASH_DSN`).

@ProviderFor(crashReports)
final crashReportsProvider = CrashReportsProvider._();

/// The crash path of the build ([CrashReports] over the define `CRASH_DSN`).

final class CrashReportsProvider
    extends $FunctionalProvider<CrashReports, CrashReports, CrashReports>
    with $Provider<CrashReports> {
  /// The crash path of the build ([CrashReports] over the define `CRASH_DSN`).
  CrashReportsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'crashReportsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$crashReportsHash();

  @$internal
  @override
  $ProviderElement<CrashReports> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  CrashReports create(Ref ref) {
    return crashReports(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CrashReports value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CrashReports>(value),
    );
  }
}

String _$crashReportsHash() => r'd72119bc6fd2f4684417e851ffb437db4428690f';

/// The server port of the sender: the REST client of the signed-in session, or null before
/// the login.

@ProviderFor(clientMetricsApi)
final clientMetricsApiProvider = ClientMetricsApiProvider._();

/// The server port of the sender: the REST client of the signed-in session, or null before
/// the login.

final class ClientMetricsApiProvider
    extends
        $FunctionalProvider<
          ClientMetricsApi?,
          ClientMetricsApi?,
          ClientMetricsApi?
        >
    with $Provider<ClientMetricsApi?> {
  /// The server port of the sender: the REST client of the signed-in session, or null before
  /// the login.
  ClientMetricsApiProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'clientMetricsApiProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$clientMetricsApiHash();

  @$internal
  @override
  $ProviderElement<ClientMetricsApi?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ClientMetricsApi? create(Ref ref) {
    return clientMetricsApi(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ClientMetricsApi? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ClientMetricsApi?>(value),
    );
  }
}

String _$clientMetricsApiHash() => r'8df35d7206b4ed00fe8b2443cfa8cac1c6fadac0';

/// The sender of the signed-in session, started; null before the login. A sign-out disposes it,
/// which sends what is left while the session still exists.

@ProviderFor(metricsSender)
final metricsSenderProvider = MetricsSenderProvider._();

/// The sender of the signed-in session, started; null before the login. A sign-out disposes it,
/// which sends what is left while the session still exists.

final class MetricsSenderProvider
    extends $FunctionalProvider<MetricsSender?, MetricsSender?, MetricsSender?>
    with $Provider<MetricsSender?> {
  /// The sender of the signed-in session, started; null before the login. A sign-out disposes it,
  /// which sends what is left while the session still exists.
  MetricsSenderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'metricsSenderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$metricsSenderHash();

  @$internal
  @override
  $ProviderElement<MetricsSender?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  MetricsSender? create(Ref ref) {
    return metricsSender(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MetricsSender? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MetricsSender?>(value),
    );
  }
}

String _$metricsSenderHash() => r'09b55bec8fe64ab8e0cafbff2b69a7bfee44048e';

/// Keeps the telemetry in step with the settings: the sender runs while signed in and resumes
/// after a change of `privacy.client_metrics`, the crash path starts or stops with
/// `privacy.crash_reports` and tags reports with the hashed instance ID.

@ProviderFor(appTelemetry)
final appTelemetryProvider = AppTelemetryProvider._();

/// Keeps the telemetry in step with the settings: the sender runs while signed in and resumes
/// after a change of `privacy.client_metrics`, the crash path starts or stops with
/// `privacy.crash_reports` and tags reports with the hashed instance ID.

final class AppTelemetryProvider extends $FunctionalProvider<void, void, void>
    with $Provider<void> {
  /// Keeps the telemetry in step with the settings: the sender runs while signed in and resumes
  /// after a change of `privacy.client_metrics`, the crash path starts or stops with
  /// `privacy.crash_reports` and tags reports with the hashed instance ID.
  AppTelemetryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appTelemetryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appTelemetryHash();

  @$internal
  @override
  $ProviderElement<void> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  void create(Ref ref) {
    return appTelemetry(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$appTelemetryHash() => r'ee28ad74fcd09fa73f1637d7b42ae2f0a36f1e62';

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

/// Whether the personal settings of the signed-in person are read from the local database.
/// Before that `personalSettingsProvider` holds the defaults, which would start crash reports
/// and send metrics of a person who turned them off (D-M4-31-5).

@ProviderFor(personalSettingsLoaded)
final personalSettingsLoadedProvider = PersonalSettingsLoadedProvider._();

/// Whether the personal settings of the signed-in person are read from the local database.
/// Before that `personalSettingsProvider` holds the defaults, which would start crash reports
/// and send metrics of a person who turned them off (D-M4-31-5).

final class PersonalSettingsLoadedProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// Whether the personal settings of the signed-in person are read from the local database.
  /// Before that `personalSettingsProvider` holds the defaults, which would start crash reports
  /// and send metrics of a person who turned them off (D-M4-31-5).
  PersonalSettingsLoadedProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'personalSettingsLoadedProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$personalSettingsLoadedHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return personalSettingsLoaded(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$personalSettingsLoadedHash() =>
    r'edd77cc8b2bf833f336e727a1ad174b162321e14';

/// `privacy.client_metrics` of the signed-in person, false while it is not loaded.

@ProviderFor(clientMetricsOn)
final clientMetricsOnProvider = ClientMetricsOnProvider._();

/// `privacy.client_metrics` of the signed-in person, false while it is not loaded.

final class ClientMetricsOnProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// `privacy.client_metrics` of the signed-in person, false while it is not loaded.
  ClientMetricsOnProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'clientMetricsOnProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$clientMetricsOnHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return clientMetricsOn(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$clientMetricsOnHash() => r'a556e5d3afc38cc7b27f8fbb37b370d843e0a150';

/// `privacy.crash_reports` of the signed-in person, false while it is not loaded: crash
/// reports start only after the login, with the person's own choice (D-M4-31-5).

@ProviderFor(crashReportsOn)
final crashReportsOnProvider = CrashReportsOnProvider._();

/// `privacy.crash_reports` of the signed-in person, false while it is not loaded: crash
/// reports start only after the login, with the person's own choice (D-M4-31-5).

final class CrashReportsOnProvider extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// `privacy.crash_reports` of the signed-in person, false while it is not loaded: crash
  /// reports start only after the login, with the person's own choice (D-M4-31-5).
  CrashReportsOnProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'crashReportsOnProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$crashReportsOnHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return crashReportsOn(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$crashReportsOnHash() => r'26ddac8a12d4d340c8d8ed23829362f845166ede';

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

String _$metricsSenderHash() => r'8a16bff4d6fdaecbd966c0f553aa06a1f3fd64ba';

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

String _$appTelemetryHash() => r'ee0d3f44f7cfdf2babf114b7a085e8cb73b1f42b';

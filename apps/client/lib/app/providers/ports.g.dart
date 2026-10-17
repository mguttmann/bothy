// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ports.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The ports of the running app; `main.dart` and every test override it.
///
/// Throws an [UnimplementedError] without an override.

@ProviderFor(appPorts)
final appPortsProvider = AppPortsProvider._();

/// The ports of the running app; `main.dart` and every test override it.
///
/// Throws an [UnimplementedError] without an override.

final class AppPortsProvider
    extends $FunctionalProvider<AppPorts, AppPorts, AppPorts>
    with $Provider<AppPorts> {
  /// The ports of the running app; `main.dart` and every test override it.
  ///
  /// Throws an [UnimplementedError] without an override.
  AppPortsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appPortsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appPortsHash();

  @$internal
  @override
  $ProviderElement<AppPorts> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AppPorts create(Ref ref) {
    return appPorts(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AppPorts value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AppPorts>(value),
    );
  }
}

String _$appPortsHash() => r'272d53b188cc92398779167ed2423cd8678fbd23';

/// The client values of D-W-115 and D-W-116.

@ProviderFor(appConfig)
final appConfigProvider = AppConfigProvider._();

/// The client values of D-W-115 and D-W-116.

final class AppConfigProvider
    extends $FunctionalProvider<AppConfig, AppConfig, AppConfig>
    with $Provider<AppConfig> {
  /// The client values of D-W-115 and D-W-116.
  AppConfigProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appConfigProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appConfigHash();

  @$internal
  @override
  $ProviderElement<AppConfig> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AppConfig create(Ref ref) {
    return appConfig(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AppConfig value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AppConfig>(value),
    );
  }
}

String _$appConfigHash() => r'afc8b76e8b65889a1228b488482bc84e20b81319';

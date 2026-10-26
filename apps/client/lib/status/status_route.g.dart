// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_route.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
/// engine's `status.updated` as the changes and its connection state as the reachability; null
/// while signed out.

@ProviderFor(appStatusSource)
final appStatusSourceProvider = AppStatusSourceProvider._();

/// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
/// engine's `status.updated` as the changes and its connection state as the reachability; null
/// while signed out.

final class AppStatusSourceProvider
    extends $FunctionalProvider<StatusSource?, StatusSource?, StatusSource?>
    with $Provider<StatusSource?> {
  /// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
  /// engine's `status.updated` as the changes and its connection state as the reachability; null
  /// while signed out.
  AppStatusSourceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appStatusSourceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appStatusSourceHash();

  @$internal
  @override
  $ProviderElement<StatusSource?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StatusSource? create(Ref ref) {
    return appStatusSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StatusSource? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StatusSource?>(value),
    );
  }
}

String _$appStatusSourceHash() => r'562d81c69883e9649adb7a43cd9d31dd8a7741e1';

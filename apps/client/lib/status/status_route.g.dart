// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'status_route.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
/// engine's `status.updated` and a read every `client.status_poll_interval` as the changes, and
/// its connection state as the reachability; null while signed out. The timed read is needed
/// because a change of agent components alone sends no `status.updated` (D-M7-09-5), and the
/// page must not keep showing a state it no longer knows (AUD-PKG-P01/7).

@ProviderFor(appStatusSource)
final appStatusSourceProvider = AppStatusSourceProvider._();

/// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
/// engine's `status.updated` and a read every `client.status_poll_interval` as the changes, and
/// its connection state as the reachability; null while signed out. The timed read is needed
/// because a change of agent components alone sends no `status.updated` (D-M7-09-5), and the
/// page must not keep showing a state it no longer knows (AUD-PKG-P01/7).

final class AppStatusSourceProvider
    extends $FunctionalProvider<StatusSource?, StatusSource?, StatusSource?>
    with $Provider<StatusSource?> {
  /// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
  /// engine's `status.updated` and a read every `client.status_poll_interval` as the changes, and
  /// its connection state as the reachability; null while signed out. The timed read is needed
  /// because a change of agent components alone sends no `status.updated` (D-M7-09-5), and the
  /// page must not keep showing a state it no longer knows (AUD-PKG-P01/7).
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

String _$appStatusSourceHash() => r'a8817108bc0685c72fcc0f57efbd358626ecf262';

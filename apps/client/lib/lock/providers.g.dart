// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The app lock of the signed-in instance on a phone, or null on the desktop (a later ticket
/// brings the lock there) and while nobody is signed in.
///
/// The policy is the cached `app_lock` of the instance row, so the lock works offline and at a
/// cold start before the network; a start that resumed a stored session locks at once when the
/// lock applies (`OnboardingState.restored`), a fresh login does not.

@ProviderFor(appLock)
final appLockProvider = AppLockProvider._();

/// The app lock of the signed-in instance on a phone, or null on the desktop (a later ticket
/// brings the lock there) and while nobody is signed in.
///
/// The policy is the cached `app_lock` of the instance row, so the lock works offline and at a
/// cold start before the network; a start that resumed a stored session locks at once when the
/// lock applies (`OnboardingState.restored`), a fresh login does not.

final class AppLockProvider
    extends
        $FunctionalProvider<
          AppLockController?,
          AppLockController?,
          AppLockController?
        >
    with $Provider<AppLockController?> {
  /// The app lock of the signed-in instance on a phone, or null on the desktop (a later ticket
  /// brings the lock there) and while nobody is signed in.
  ///
  /// The policy is the cached `app_lock` of the instance row, so the lock works offline and at a
  /// cold start before the network; a start that resumed a stored session locks at once when the
  /// lock applies (`OnboardingState.restored`), a fresh login does not.
  AppLockProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appLockProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appLockHash();

  @$internal
  @override
  $ProviderElement<AppLockController?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AppLockController? create(Ref ref) {
    return appLock(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AppLockController? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AppLockController?>(value),
    );
  }
}

String _$appLockHash() => r'20f478619db709ff2de4f2c90ffd7a5e844805b3';

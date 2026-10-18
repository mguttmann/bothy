// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'admin_route.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The REST source of the administration over the client of the signed-in session (DPoP,
/// refresh and the step-up interceptor apply), or null before the login.

@ProviderFor(adminRepository)
final adminRepositoryProvider = AdminRepositoryProvider._();

/// The REST source of the administration over the client of the signed-in session (DPoP,
/// refresh and the step-up interceptor apply), or null before the login.

final class AdminRepositoryProvider
    extends
        $FunctionalProvider<
          AdminRepository?,
          AdminRepository?,
          AdminRepository?
        >
    with $Provider<AdminRepository?> {
  /// The REST source of the administration over the client of the signed-in session (DPoP,
  /// refresh and the step-up interceptor apply), or null before the login.
  AdminRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'adminRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$adminRepositoryHash();

  @$internal
  @override
  $ProviderElement<AdminRepository?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AdminRepository? create(Ref ref) {
    return adminRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AdminRepository? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AdminRepository?>(value),
    );
  }
}

String _$adminRepositoryHash() => r'e62badcdfa34cdd7d7301d3a390e345c5df905e4';

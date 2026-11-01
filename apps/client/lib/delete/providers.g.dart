// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The [DeletePort] of the signed-in session, or null without one.

@ProviderFor(deletePort)
final deletePortProvider = DeletePortProvider._();

/// The [DeletePort] of the signed-in session, or null without one.

final class DeletePortProvider
    extends $FunctionalProvider<DeletePort?, DeletePort?, DeletePort?>
    with $Provider<DeletePort?> {
  /// The [DeletePort] of the signed-in session, or null without one.
  DeletePortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'deletePortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$deletePortHash();

  @$internal
  @override
  $ProviderElement<DeletePort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DeletePort? create(Ref ref) {
    return deletePort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DeletePort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DeletePort?>(value),
    );
  }
}

String _$deletePortHash() => r'3939392adb56b981f3a2aa9b85275abfefc4e65a';

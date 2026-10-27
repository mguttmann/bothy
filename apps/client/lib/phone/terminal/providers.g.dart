// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The coordinator of the signed-in phone, or null on the desktop and before the login; the
/// terminal screen M8 registers its tabs with it.

@ProviderFor(phoneTerminalCoordinator)
final phoneTerminalCoordinatorProvider = PhoneTerminalCoordinatorProvider._();

/// The coordinator of the signed-in phone, or null on the desktop and before the login; the
/// terminal screen M8 registers its tabs with it.

final class PhoneTerminalCoordinatorProvider
    extends
        $FunctionalProvider<
          PhoneTerminalCoordinator?,
          PhoneTerminalCoordinator?,
          PhoneTerminalCoordinator?
        >
    with $Provider<PhoneTerminalCoordinator?> {
  /// The coordinator of the signed-in phone, or null on the desktop and before the login; the
  /// terminal screen M8 registers its tabs with it.
  PhoneTerminalCoordinatorProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'phoneTerminalCoordinatorProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$phoneTerminalCoordinatorHash();

  @$internal
  @override
  $ProviderElement<PhoneTerminalCoordinator?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  PhoneTerminalCoordinator? create(Ref ref) {
    return phoneTerminalCoordinator(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PhoneTerminalCoordinator? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PhoneTerminalCoordinator?>(value),
    );
  }
}

String _$phoneTerminalCoordinatorHash() =>
    r'c7aa6c53f90f5c4fbeba0ba7fe53e853ea41dee1';

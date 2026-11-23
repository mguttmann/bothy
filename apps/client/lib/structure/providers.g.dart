// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The creates of the signed-in session, or null before the login; tests override it.

@ProviderFor(structurePort)
final structurePortProvider = StructurePortProvider._();

/// The creates of the signed-in session, or null before the login; tests override it.

final class StructurePortProvider
    extends $FunctionalProvider<StructurePort?, StructurePort?, StructurePort?>
    with $Provider<StructurePort?> {
  /// The creates of the signed-in session, or null before the login; tests override it.
  StructurePortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'structurePortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$structurePortHash();

  @$internal
  @override
  $ProviderElement<StructurePort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StructurePort? create(Ref ref) {
    return structurePort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StructurePort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StructurePort?>(value),
    );
  }
}

String _$structurePortHash() => r'b4edb550e690ce6ac700ffd0b494c4913638637d';

/// Makes an idempotency key for a create; tests override it.

@ProviderFor(structureKeys)
final structureKeysProvider = StructureKeysProvider._();

/// Makes an idempotency key for a create; tests override it.

final class StructureKeysProvider
    extends
        $FunctionalProvider<
          String Function(),
          String Function(),
          String Function()
        >
    with $Provider<String Function()> {
  /// Makes an idempotency key for a create; tests override it.
  StructureKeysProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'structureKeysProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$structureKeysHash();

  @$internal
  @override
  $ProviderElement<String Function()> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  String Function() create(Ref ref) {
    return structureKeys(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String Function() value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String Function()>(value),
    );
  }
}

String _$structureKeysHash() => r'69ea0507b1f2e01343f6e96d7b0f832097affac1';

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The search assistant of the signed-in instance (`assistSearch`), or null while signed out.

@ProviderFor(assistPort)
final assistPortProvider = AssistPortProvider._();

/// The search assistant of the signed-in instance (`assistSearch`), or null while signed out.

final class AssistPortProvider
    extends $FunctionalProvider<AssistPort?, AssistPort?, AssistPort?>
    with $Provider<AssistPort?> {
  /// The search assistant of the signed-in instance (`assistSearch`), or null while signed out.
  AssistPortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'assistPortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$assistPortHash();

  @$internal
  @override
  $ProviderElement<AssistPort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AssistPort? create(Ref ref) {
    return assistPort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AssistPort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AssistPort?>(value),
    );
  }
}

String _$assistPortHash() => r'101e94f872990dcdc95ee2dc623375fe7ccda25a';

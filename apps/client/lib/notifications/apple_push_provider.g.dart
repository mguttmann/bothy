// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'apple_push_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [ApplePush] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension never sees the default preview mode of a person who
/// chose `none`; later changes of `notify.preview_mode` follow.

@ProviderFor(applePush)
final applePushProvider = ApplePushProvider._();

/// [ApplePush] of the build kept in step with the signed-in account, or null where the build
/// has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension never sees the default preview mode of a person who
/// chose `none`; later changes of `notify.preview_mode` follow.

final class ApplePushProvider
    extends $FunctionalProvider<ApplePush?, ApplePush?, ApplePush?>
    with $Provider<ApplePush?> {
  /// [ApplePush] of the build kept in step with the signed-in account, or null where the build
  /// has none (Windows, tests).
  ///
  /// The account is handed over once per signed-in services and only after the personal
  /// settings are read, so the extension never sees the default preview mode of a person who
  /// chose `none`; later changes of `notify.preview_mode` follow.
  ApplePushProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'applePushProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$applePushHash();

  @$internal
  @override
  $ProviderElement<ApplePush?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ApplePush? create(Ref ref) {
    return applePush(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ApplePush? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ApplePush?>(value),
    );
  }
}

String _$applePushHash() => r'3bf10a79b9e187a0a303468e1678b654c375005a';

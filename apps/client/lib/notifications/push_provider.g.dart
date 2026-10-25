// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The [PushPlatform] of the build kept in step with the signed-in account, or null where the
/// build has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension or the push service never sees the default preview mode
/// of a person who chose `none`; later changes of `notify.preview_mode` follow.
/// Whether the build receives pushes on this device; null where it has no push platform
/// (Windows, tests). D9 says so when it is false (brief M5-05 Design note 4).

@ProviderFor(pushAvailable)
final pushAvailableProvider = PushAvailableProvider._();

/// The [PushPlatform] of the build kept in step with the signed-in account, or null where the
/// build has none (Windows, tests).
///
/// The account is handed over once per signed-in services and only after the personal
/// settings are read, so the extension or the push service never sees the default preview mode
/// of a person who chose `none`; later changes of `notify.preview_mode` follow.
/// Whether the build receives pushes on this device; null where it has no push platform
/// (Windows, tests). D9 says so when it is false (brief M5-05 Design note 4).

final class PushAvailableProvider
    extends $FunctionalProvider<AsyncValue<bool?>, bool?, FutureOr<bool?>>
    with $FutureModifier<bool?>, $FutureProvider<bool?> {
  /// The [PushPlatform] of the build kept in step with the signed-in account, or null where the
  /// build has none (Windows, tests).
  ///
  /// The account is handed over once per signed-in services and only after the personal
  /// settings are read, so the extension or the push service never sees the default preview mode
  /// of a person who chose `none`; later changes of `notify.preview_mode` follow.
  /// Whether the build receives pushes on this device; null where it has no push platform
  /// (Windows, tests). D9 says so when it is false (brief M5-05 Design note 4).
  PushAvailableProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'pushAvailableProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$pushAvailableHash();

  @$internal
  @override
  $FutureProviderElement<bool?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool?> create(Ref ref) {
    return pushAvailable(ref);
  }
}

String _$pushAvailableHash() => r'bfbce18387bfdb81b38045d7f69ba1ea4e7c0613';

@ProviderFor(pushPlatform)
final pushPlatformProvider = PushPlatformProvider._();

final class PushPlatformProvider
    extends $FunctionalProvider<PushPlatform?, PushPlatform?, PushPlatform?>
    with $Provider<PushPlatform?> {
  PushPlatformProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'pushPlatformProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$pushPlatformHash();

  @$internal
  @override
  $ProviderElement<PushPlatform?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  PushPlatform? create(Ref ref) {
    return pushPlatform(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PushPlatform? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PushPlatform?>(value),
    );
  }
}

String _$pushPlatformHash() => r'1a4e0784eafb453d56ee6a205c1cfb3946d2db22';

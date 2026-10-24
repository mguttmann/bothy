// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The source of [MentionsController]: `listMyMentions` of the signed-in instance, or null
/// while signed out.

@ProviderFor(mentionsSource)
final mentionsSourceProvider = MentionsSourceProvider._();

/// The source of [MentionsController]: `listMyMentions` of the signed-in instance, or null
/// while signed out.

final class MentionsSourceProvider
    extends
        $FunctionalProvider<MentionsSource?, MentionsSource?, MentionsSource?>
    with $Provider<MentionsSource?> {
  /// The source of [MentionsController]: `listMyMentions` of the signed-in instance, or null
  /// while signed out.
  MentionsSourceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'mentionsSourceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$mentionsSourceHash();

  @$internal
  @override
  $ProviderElement<MentionsSource?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  MentionsSource? create(Ref ref) {
    return mentionsSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MentionsSource? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MentionsSource?>(value),
    );
  }
}

String _$mentionsSourceHash() => r'74bc0f2a372c28dcaf117e670320f0ce8c2353e4';

/// The pages of the mentions view; [loadMore] appends the next page.

@ProviderFor(MentionsController)
final mentionsControllerProvider = MentionsControllerProvider._();

/// The pages of the mentions view; [loadMore] appends the next page.
final class MentionsControllerProvider
    extends $AsyncNotifierProvider<MentionsController, MentionsPage> {
  /// The pages of the mentions view; [loadMore] appends the next page.
  MentionsControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'mentionsControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$mentionsControllerHash();

  @$internal
  @override
  MentionsController create() => MentionsController();
}

String _$mentionsControllerHash() =>
    r'd8fb31a62ff6bb08371e59978f59be13a2b2b9de';

/// The pages of the mentions view; [loadMore] appends the next page.

abstract class _$MentionsController extends $AsyncNotifier<MentionsPage> {
  FutureOr<MentionsPage> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<AsyncValue<MentionsPage>, MentionsPage>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<MentionsPage>, MentionsPage>,
              AsyncValue<MentionsPage>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

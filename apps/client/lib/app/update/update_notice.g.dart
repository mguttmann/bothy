// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'update_notice.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The version gate's decision for the signed-in instance (18 9 item 2): the decision of the
/// start, then every new one before a reconnect; allowed while no instance is signed in.

@ProviderFor(clientVersion)
final clientVersionProvider = ClientVersionProvider._();

/// The version gate's decision for the signed-in instance (18 9 item 2): the decision of the
/// start, then every new one before a reconnect; allowed while no instance is signed in.

final class ClientVersionProvider
    extends
        $FunctionalProvider<
          AsyncValue<ClientVersionDecision>,
          ClientVersionDecision,
          Stream<ClientVersionDecision>
        >
    with
        $FutureModifier<ClientVersionDecision>,
        $StreamProvider<ClientVersionDecision> {
  /// The version gate's decision for the signed-in instance (18 9 item 2): the decision of the
  /// start, then every new one before a reconnect; allowed while no instance is signed in.
  ClientVersionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'clientVersionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$clientVersionHash();

  @$internal
  @override
  $StreamProviderElement<ClientVersionDecision> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<ClientVersionDecision> create(Ref ref) {
    return clientVersion(ref);
  }
}

String _$clientVersionHash() => r'9a15486b91f0895c530d71163f7b9e5a5aeec7b5';

/// The recommended version whose notice the person dismissed, kept in the app preferences.

@ProviderFor(UpdateNoticeDismissal)
final updateNoticeDismissalProvider = UpdateNoticeDismissalProvider._();

/// The recommended version whose notice the person dismissed, kept in the app preferences.
final class UpdateNoticeDismissalProvider
    extends $AsyncNotifierProvider<UpdateNoticeDismissal, String?> {
  /// The recommended version whose notice the person dismissed, kept in the app preferences.
  UpdateNoticeDismissalProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'updateNoticeDismissalProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$updateNoticeDismissalHash();

  @$internal
  @override
  UpdateNoticeDismissal create() => UpdateNoticeDismissal();
}

String _$updateNoticeDismissalHash() =>
    r'150554bd45ba23d7946d7a949fa32a9b896b4cd7';

/// The recommended version whose notice the person dismissed, kept in the app preferences.

abstract class _$UpdateNoticeDismissal extends $AsyncNotifier<String?> {
  FutureOr<String?> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<AsyncValue<String?>, String?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<String?>, String?>,
              AsyncValue<String?>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The version the notice "Update recommended" names, or null when none shows: the app is
/// below `client.recommended_version.<platform>` and the person has not dismissed that version.

@ProviderFor(recommendedUpdate)
final recommendedUpdateProvider = RecommendedUpdateProvider._();

/// The version the notice "Update recommended" names, or null when none shows: the app is
/// below `client.recommended_version.<platform>` and the person has not dismissed that version.

final class RecommendedUpdateProvider
    extends $FunctionalProvider<String?, String?, String?>
    with $Provider<String?> {
  /// The version the notice "Update recommended" names, or null when none shows: the app is
  /// below `client.recommended_version.<platform>` and the person has not dismissed that version.
  RecommendedUpdateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'recommendedUpdateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$recommendedUpdateHash();

  @$internal
  @override
  $ProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String? create(Ref ref) {
    return recommendedUpdate(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$recommendedUpdateHash() => r'e1aa753934c27818e670d9aa305acf5018729a38';

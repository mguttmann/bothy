// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'new_agent_entry.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
/// out, not an owner, an older server or no runner that hosts agents (26 8.1).

@ProviderFor(newAgentOffer)
final newAgentOfferProvider = NewAgentOfferProvider._();

/// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
/// out, not an owner, an older server or no runner that hosts agents (26 8.1).

final class NewAgentOfferProvider
    extends
        $FunctionalProvider<
          AsyncValue<NewAgentOffer?>,
          NewAgentOffer?,
          FutureOr<NewAgentOffer?>
        >
    with $FutureModifier<NewAgentOffer?>, $FutureProvider<NewAgentOffer?> {
  /// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
  /// out, not an owner, an older server or no runner that hosts agents (26 8.1).
  NewAgentOfferProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'newAgentOfferProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$newAgentOfferHash();

  @$internal
  @override
  $FutureProviderElement<NewAgentOffer?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<NewAgentOffer?> create(Ref ref) {
    return newAgentOffer(ref);
  }
}

String _$newAgentOfferHash() => r'603e97913e0617fc58d779c114b57e6107117fcd';

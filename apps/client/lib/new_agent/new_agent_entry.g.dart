// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'new_agent_entry.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
/// out, not an owner, an older server or no runner that hosts agents (26 8.1). No event carries
/// `hosts_agents` (09 7, 26 8.1), so it is read again when the stored agent cards change (a new
/// runner), when the presence of an agent changes (the snapshot of `ready` included), when the
/// app returns to the foreground, and on every opening of the dialog (AUD-PKG-P01/5).

@ProviderFor(newAgentOffer)
final newAgentOfferProvider = NewAgentOfferProvider._();

/// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
/// out, not an owner, an older server or no runner that hosts agents (26 8.1). No event carries
/// `hosts_agents` (09 7, 26 8.1), so it is read again when the stored agent cards change (a new
/// runner), when the presence of an agent changes (the snapshot of `ready` included), when the
/// app returns to the foreground, and on every opening of the dialog (AUD-PKG-P01/5).

final class NewAgentOfferProvider
    extends
        $FunctionalProvider<
          AsyncValue<NewAgentOffer?>,
          NewAgentOffer?,
          FutureOr<NewAgentOffer?>
        >
    with $FutureModifier<NewAgentOffer?>, $FutureProvider<NewAgentOffer?> {
  /// The offer of "New agent" for the signed-in person, or null when the entry stays hidden: signed
  /// out, not an owner, an older server or no runner that hosts agents (26 8.1). No event carries
  /// `hosts_agents` (09 7, 26 8.1), so it is read again when the stored agent cards change (a new
  /// runner), when the presence of an agent changes (the snapshot of `ready` included), when the
  /// app returns to the foreground, and on every opening of the dialog (AUD-PKG-P01/5).
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

String _$newAgentOfferHash() => r'220a6cc86b5d74e49d4251d25c2768deeeea5a16';

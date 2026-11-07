// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'autonomous_runs_dialog.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
/// version; null when the answer carries no switch (a server before stage AR) or the read
/// failed. The sync snapshot and the channel events of a server of stage AR leave the member
/// out, so this read is what fills it (AR-08).

@ProviderFor(autonomousRunsOfServer)
final autonomousRunsOfServerProvider = AutonomousRunsOfServerFamily._();

/// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
/// version; null when the answer carries no switch (a server before stage AR) or the read
/// failed. The sync snapshot and the channel events of a server of stage AR leave the member
/// out, so this read is what fills it (AR-08).

final class AutonomousRunsOfServerProvider
    extends $FunctionalProvider<AsyncValue<bool?>, bool?, FutureOr<bool?>>
    with $FutureModifier<bool?>, $FutureProvider<bool?> {
  /// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
  /// version; null when the answer carries no switch (a server before stage AR) or the read
  /// failed. The sync snapshot and the channel events of a server of stage AR leave the member
  /// out, so this read is what fills it (AR-08).
  AutonomousRunsOfServerProvider._({
    required AutonomousRunsOfServerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'autonomousRunsOfServerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$autonomousRunsOfServerHash();

  @override
  String toString() {
    return r'autonomousRunsOfServerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<bool?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool?> create(Ref ref) {
    final argument = this.argument as String;
    return autonomousRunsOfServer(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AutonomousRunsOfServerProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$autonomousRunsOfServerHash() =>
    r'b2f551dd70ed9bf715ce6103dbaf001fa6117b83';

/// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
/// version; null when the answer carries no switch (a server before stage AR) or the read
/// failed. The sync snapshot and the channel events of a server of stage AR leave the member
/// out, so this read is what fills it (AR-08).

final class AutonomousRunsOfServerFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<bool?>, String> {
  AutonomousRunsOfServerFamily._()
    : super(
        retry: null,
        name: r'autonomousRunsOfServerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// `allow_autonomous_runs` of [channelId] as `getChannel` reports it, stored with the channel's
  /// version; null when the answer carries no switch (a server before stage AR) or the read
  /// failed. The sync snapshot and the channel events of a server of stage AR leave the member
  /// out, so this read is what fills it (AR-08).

  AutonomousRunsOfServerProvider call(String channelId) =>
      AutonomousRunsOfServerProvider._(argument: channelId, from: this);

  @override
  String toString() => r'autonomousRunsOfServerProvider';
}

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The team operations of the signed-in session, or null before the login; tests override it.

@ProviderFor(teamPort)
final teamPortProvider = TeamPortProvider._();

/// The team operations of the signed-in session, or null before the login; tests override it.

final class TeamPortProvider
    extends $FunctionalProvider<TeamPort?, TeamPort?, TeamPort?>
    with $Provider<TeamPort?> {
  /// The team operations of the signed-in session, or null before the login; tests override it.
  TeamPortProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'teamPortProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$teamPortHash();

  @$internal
  @override
  $ProviderElement<TeamPort?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  TeamPort? create(Ref ref) {
    return teamPort(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TeamPort? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TeamPort?>(value),
    );
  }
}

String _$teamPortHash() => r'fa676dcd08b626997d339e0cc5e205fe9669986b';

/// Makes an idempotency key for a team operation; tests override it.

@ProviderFor(teamKeys)
final teamKeysProvider = TeamKeysProvider._();

/// Makes an idempotency key for a team operation; tests override it.

final class TeamKeysProvider
    extends
        $FunctionalProvider<
          String Function(),
          String Function(),
          String Function()
        >
    with $Provider<String Function()> {
  /// Makes an idempotency key for a team operation; tests override it.
  TeamKeysProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'teamKeysProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$teamKeysHash();

  @$internal
  @override
  $ProviderElement<String Function()> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  String Function() create(Ref ref) {
    return teamKeys(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String Function() value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String Function()>(value),
    );
  }
}

String _$teamKeysHash() => r'2920f62b593b27680d061037a7547e566c8e1c66';

/// The team of [channelId], or null when it is no team channel, the person is no owner, the
/// channel is no text channel or the read failed (the header then stays hidden).

@ProviderFor(channelTeam)
final channelTeamProvider = ChannelTeamFamily._();

/// The team of [channelId], or null when it is no team channel, the person is no owner, the
/// channel is no text channel or the read failed (the header then stays hidden).

final class ChannelTeamProvider
    extends
        $FunctionalProvider<
          AsyncValue<TeamView?>,
          TeamView?,
          FutureOr<TeamView?>
        >
    with $FutureModifier<TeamView?>, $FutureProvider<TeamView?> {
  /// The team of [channelId], or null when it is no team channel, the person is no owner, the
  /// channel is no text channel or the read failed (the header then stays hidden).
  ChannelTeamProvider._({
    required ChannelTeamFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelTeamProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelTeamHash();

  @override
  String toString() {
    return r'channelTeamProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<TeamView?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<TeamView?> create(Ref ref) {
    final argument = this.argument as String;
    return channelTeam(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelTeamProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelTeamHash() => r'83f30bfb2072ed2873978f983af4081d4b5d01fd';

/// The team of [channelId], or null when it is no team channel, the person is no owner, the
/// channel is no text channel or the read failed (the header then stays hidden).

final class ChannelTeamFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<TeamView?>, String> {
  ChannelTeamFamily._()
    : super(
        retry: null,
        name: r'channelTeamProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The team of [channelId], or null when it is no team channel, the person is no owner, the
  /// channel is no text channel or the read failed (the header then stays hidden).

  ChannelTeamProvider call(String channelId) =>
      ChannelTeamProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelTeamProvider';
}

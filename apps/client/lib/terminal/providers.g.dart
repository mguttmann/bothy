// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The terminal ports of the signed-in instance, or null before the login.

@ProviderFor(terminalServices)
final terminalServicesProvider = TerminalServicesProvider._();

/// The terminal ports of the signed-in instance, or null before the login.

final class TerminalServicesProvider
    extends
        $FunctionalProvider<
          TerminalServices?,
          TerminalServices?,
          TerminalServices?
        >
    with $Provider<TerminalServices?> {
  /// The terminal ports of the signed-in instance, or null before the login.
  TerminalServicesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'terminalServicesProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$terminalServicesHash();

  @$internal
  @override
  $ProviderElement<TerminalServices?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  TerminalServices? create(Ref ref) {
    return terminalServices(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TerminalServices? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TerminalServices?>(value),
    );
  }
}

String _$terminalServicesHash() => r'37cdf6d5e7eb73c8d6d4d570d802809db392a217';

/// The person's own user ID as the engine's last snapshot named it; null before it.

@ProviderFor(terminalSelfUserId)
final terminalSelfUserIdProvider = TerminalSelfUserIdProvider._();

/// The person's own user ID as the engine's last snapshot named it; null before it.

final class TerminalSelfUserIdProvider
    extends $FunctionalProvider<String?, String?, String?>
    with $Provider<String?> {
  /// The person's own user ID as the engine's last snapshot named it; null before it.
  TerminalSelfUserIdProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'terminalSelfUserIdProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$terminalSelfUserIdHash();

  @$internal
  @override
  $ProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String? create(Ref ref) {
    return terminalSelfUserId(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$terminalSelfUserIdHash() =>
    r'e627e8aa381cba626d95eeca11648d0cd440e7f5';

/// The hosts of the person, reloaded after every host event (D-M4-36-4); null before the
/// login.

@ProviderFor(HostsList)
final hostsListProvider = HostsListProvider._();

/// The hosts of the person, reloaded after every host event (D-M4-36-4); null before the
/// login.
final class HostsListProvider
    extends $AsyncNotifierProvider<HostsList, HostsOverview?> {
  /// The hosts of the person, reloaded after every host event (D-M4-36-4); null before the
  /// login.
  HostsListProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'hostsListProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$hostsListHash();

  @$internal
  @override
  HostsList create() => HostsList();
}

String _$hostsListHash() => r'a0fbc498ad9e77065a774da43ffc4b95c02a340d';

/// The hosts of the person, reloaded after every host event (D-M4-36-4); null before the
/// login.

abstract class _$HostsList extends $AsyncNotifier<HostsOverview?> {
  FutureOr<HostsOverview?> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<AsyncValue<HostsOverview?>, HostsOverview?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<HostsOverview?>, HostsOverview?>,
              AsyncValue<HostsOverview?>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The tabs of host [hostId], kept for the login; null before the login or before the engine
/// named the person.

@ProviderFor(terminalTabs)
final terminalTabsProvider = TerminalTabsFamily._();

/// The tabs of host [hostId], kept for the login; null before the login or before the engine
/// named the person.

final class TerminalTabsProvider
    extends $FunctionalProvider<TerminalTabs?, TerminalTabs?, TerminalTabs?>
    with $Provider<TerminalTabs?> {
  /// The tabs of host [hostId], kept for the login; null before the login or before the engine
  /// named the person.
  TerminalTabsProvider._({
    required TerminalTabsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'terminalTabsProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$terminalTabsHash();

  @override
  String toString() {
    return r'terminalTabsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<TerminalTabs?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  TerminalTabs? create(Ref ref) {
    final argument = this.argument as String;
    return terminalTabs(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TerminalTabs? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TerminalTabs?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is TerminalTabsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$terminalTabsHash() => r'd4ad188074d706f92a1c0772fefa0d6ed7e85e65';

/// The tabs of host [hostId], kept for the login; null before the login or before the engine
/// named the person.

final class TerminalTabsFamily extends $Family
    with $FunctionalFamilyOverride<TerminalTabs?, String> {
  TerminalTabsFamily._()
    : super(
        retry: null,
        name: r'terminalTabsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// The tabs of host [hostId], kept for the login; null before the login or before the engine
  /// named the person.

  TerminalTabsProvider call(String hostId) =>
      TerminalTabsProvider._(argument: hostId, from: this);

  @override
  String toString() => r'terminalTabsProvider';
}

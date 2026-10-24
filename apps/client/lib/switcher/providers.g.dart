// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The recent targets of this device, or null without a database; keeps and lists at most
/// `ui.quick_switch_max_results` targets, timed by the app's clock port.

@ProviderFor(recentTargets)
final recentTargetsProvider = RecentTargetsProvider._();

/// The recent targets of this device, or null without a database; keeps and lists at most
/// `ui.quick_switch_max_results` targets, timed by the app's clock port.

final class RecentTargetsProvider
    extends $FunctionalProvider<RecentTargets?, RecentTargets?, RecentTargets?>
    with $Provider<RecentTargets?> {
  /// The recent targets of this device, or null without a database; keeps and lists at most
  /// `ui.quick_switch_max_results` targets, timed by the app's clock port.
  RecentTargetsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'recentTargetsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$recentTargetsHash();

  @$internal
  @override
  $ProviderElement<RecentTargets?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  RecentTargets? create(Ref ref) {
    return recentTargets(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RecentTargets? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RecentTargets?>(value),
    );
  }
}

String _$recentTargetsHash() => r'8baa667d87f2d945ba87138b911cac8af1e553ea';

/// The recent targets, newest first.

@ProviderFor(recentTargetList)
final recentTargetListProvider = RecentTargetListProvider._();

/// The recent targets, newest first.

final class RecentTargetListProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RecentTarget>>,
          List<RecentTarget>,
          Stream<List<RecentTarget>>
        >
    with
        $FutureModifier<List<RecentTarget>>,
        $StreamProvider<List<RecentTarget>> {
  /// The recent targets, newest first.
  RecentTargetListProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'recentTargetListProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$recentTargetListHash();

  @$internal
  @override
  $StreamProviderElement<List<RecentTarget>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RecentTarget>> create(Ref ref) {
    return recentTargetList(ref);
  }
}

String _$recentTargetListHash() => r'1058103b739209e3618f4a8144a9c09760453c18';

/// Records a visit of [path] (path and query of the router) once; the shell watches it with
/// the location it shows, so every new route is recorded when it appears. Returns whether the
/// path named a space, channel or thread.

@ProviderFor(recordVisit)
final recordVisitProvider = RecordVisitFamily._();

/// Records a visit of [path] (path and query of the router) once; the shell watches it with
/// the location it shows, so every new route is recorded when it appears. Returns whether the
/// path named a space, channel or thread.

final class RecordVisitProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// Records a visit of [path] (path and query of the router) once; the shell watches it with
  /// the location it shows, so every new route is recorded when it appears. Returns whether the
  /// path named a space, channel or thread.
  RecordVisitProvider._({
    required RecordVisitFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'recordVisitProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$recordVisitHash();

  @override
  String toString() {
    return r'recordVisitProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    final argument = this.argument as String;
    return recordVisit(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is RecordVisitProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$recordVisitHash() => r'e349460445eeebdbf438739c5376f21a64cec177';

/// Records a visit of [path] (path and query of the router) once; the shell watches it with
/// the location it shows, so every new route is recorded when it appears. Returns whether the
/// path named a space, channel or thread.

final class RecordVisitFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<bool>, String> {
  RecordVisitFamily._()
    : super(
        retry: null,
        name: r'recordVisitProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Records a visit of [path] (path and query of the router) once; the shell watches it with
  /// the location it shows, so every new route is recorded when it appears. Returns whether the
  /// path named a space, channel or thread.

  RecordVisitProvider call(String path) =>
      RecordVisitProvider._(argument: path, from: this);

  @override
  String toString() => r'recordVisitProvider';
}

/// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
/// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
/// channel.

@ProviderFor(switcherCurrentChannel)
final switcherCurrentChannelProvider = SwitcherCurrentChannelFamily._();

/// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
/// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
/// channel.

final class SwitcherCurrentChannelProvider
    extends $FunctionalProvider<AsyncValue<String?>, String?, FutureOr<String?>>
    with $FutureModifier<String?>, $FutureProvider<String?> {
  /// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
  /// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
  /// channel.
  SwitcherCurrentChannelProvider._({
    required SwitcherCurrentChannelFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'switcherCurrentChannelProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$switcherCurrentChannelHash();

  @override
  String toString() {
    return r'switcherCurrentChannelProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<String?> create(Ref ref) {
    final argument = this.argument as String;
    return switcherCurrentChannel(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is SwitcherCurrentChannelProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$switcherCurrentChannelHash() =>
    r'40536888b28c6c88f4f91539926ad58ea3c65bb0';

/// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
/// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
/// channel.

final class SwitcherCurrentChannelFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<String?>, String> {
  SwitcherCurrentChannelFamily._()
    : super(
        retry: null,
        name: r'switcherCurrentChannelProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
  /// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
  /// channel.

  SwitcherCurrentChannelProvider call(String path) =>
      SwitcherCurrentChannelProvider._(argument: path, from: this);

  @override
  String toString() => r'switcherCurrentChannelProvider';
}

/// The stored spaces, on every change.

@ProviderFor(switcherSpaces)
final switcherSpacesProvider = SwitcherSpacesProvider._();

/// The stored spaces, on every change.

final class SwitcherSpacesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Space>>,
          List<Space>,
          Stream<List<Space>>
        >
    with $FutureModifier<List<Space>>, $StreamProvider<List<Space>> {
  /// The stored spaces, on every change.
  SwitcherSpacesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'switcherSpacesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$switcherSpacesHash();

  @$internal
  @override
  $StreamProviderElement<List<Space>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Space>> create(Ref ref) {
    return switcherSpaces(ref);
  }
}

String _$switcherSpacesHash() => r'509574fb48718cc54f6a6d771a14778cd0ef1143';

/// The stored channels of [spaceId], or the direct messages for null, on every change.

@ProviderFor(switcherChannels)
final switcherChannelsProvider = SwitcherChannelsFamily._();

/// The stored channels of [spaceId], or the direct messages for null, on every change.

final class SwitcherChannelsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Channel>>,
          List<Channel>,
          Stream<List<Channel>>
        >
    with $FutureModifier<List<Channel>>, $StreamProvider<List<Channel>> {
  /// The stored channels of [spaceId], or the direct messages for null, on every change.
  SwitcherChannelsProvider._({
    required SwitcherChannelsFamily super.from,
    required String? super.argument,
  }) : super(
         retry: null,
         name: r'switcherChannelsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$switcherChannelsHash();

  @override
  String toString() {
    return r'switcherChannelsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Channel>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Channel>> create(Ref ref) {
    final argument = this.argument as String?;
    return switcherChannels(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is SwitcherChannelsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$switcherChannelsHash() => r'd4401176325ac034c4d5dde7465db61bcad4ec34';

/// The stored channels of [spaceId], or the direct messages for null, on every change.

final class SwitcherChannelsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Channel>>, String?> {
  SwitcherChannelsFamily._()
    : super(
        retry: null,
        name: r'switcherChannelsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored channels of [spaceId], or the direct messages for null, on every change.

  SwitcherChannelsProvider call(String? spaceId) =>
      SwitcherChannelsProvider._(argument: spaceId, from: this);

  @override
  String toString() => r'switcherChannelsProvider';
}

/// The stored threads of [channelId], on every change.

@ProviderFor(switcherThreads)
final switcherThreadsProvider = SwitcherThreadsFamily._();

/// The stored threads of [channelId], on every change.

final class SwitcherThreadsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Thread>>,
          List<Thread>,
          Stream<List<Thread>>
        >
    with $FutureModifier<List<Thread>>, $StreamProvider<List<Thread>> {
  /// The stored threads of [channelId], on every change.
  SwitcherThreadsProvider._({
    required SwitcherThreadsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'switcherThreadsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$switcherThreadsHash();

  @override
  String toString() {
    return r'switcherThreadsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Thread>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Thread>> create(Ref ref) {
    final argument = this.argument as String;
    return switcherThreads(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is SwitcherThreadsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$switcherThreadsHash() => r'54b3b9702874646cb0c89608a6ec2fb47a48b573';

/// The stored threads of [channelId], on every change.

final class SwitcherThreadsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Thread>>, String> {
  SwitcherThreadsFamily._()
    : super(
        retry: null,
        name: r'switcherThreadsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored threads of [channelId], on every change.

  SwitcherThreadsProvider call(String channelId) =>
      SwitcherThreadsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'switcherThreadsProvider';
}

/// The stored principals, on every change.

@ProviderFor(switcherPrincipals)
final switcherPrincipalsProvider = SwitcherPrincipalsProvider._();

/// The stored principals, on every change.

final class SwitcherPrincipalsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Principal>>,
          List<Principal>,
          Stream<List<Principal>>
        >
    with $FutureModifier<List<Principal>>, $StreamProvider<List<Principal>> {
  /// The stored principals, on every change.
  SwitcherPrincipalsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'switcherPrincipalsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$switcherPrincipalsHash();

  @$internal
  @override
  $StreamProviderElement<List<Principal>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Principal>> create(Ref ref) {
    return switcherPrincipals(ref);
  }
}

String _$switcherPrincipalsHash() =>
    r'd547fb8b420ac2649efe9a729371adf2cd50499e';

/// The stored agent cards, on every change.

@ProviderFor(switcherAgents)
final switcherAgentsProvider = SwitcherAgentsProvider._();

/// The stored agent cards, on every change.

final class SwitcherAgentsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<AgentCard>>,
          List<AgentCard>,
          Stream<List<AgentCard>>
        >
    with $FutureModifier<List<AgentCard>>, $StreamProvider<List<AgentCard>> {
  /// The stored agent cards, on every change.
  SwitcherAgentsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'switcherAgentsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$switcherAgentsHash();

  @$internal
  @override
  $StreamProviderElement<List<AgentCard>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<AgentCard>> create(Ref ref) {
    return switcherAgents(ref);
  }
}

String _$switcherAgentsHash() => r'6cee64be523ebaf3d48a53952c88dfbc5b1fc4c9';

/// Every stored command, on every change.

@ProviderFor(switcherCommands)
final switcherCommandsProvider = SwitcherCommandsProvider._();

/// Every stored command, on every change.

final class SwitcherCommandsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Command>>,
          List<Command>,
          Stream<List<Command>>
        >
    with $FutureModifier<List<Command>>, $StreamProvider<List<Command>> {
  /// Every stored command, on every change.
  SwitcherCommandsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'switcherCommandsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$switcherCommandsHash();

  @$internal
  @override
  $StreamProviderElement<List<Command>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Command>> create(Ref ref) {
    return switcherCommands(ref);
  }
}

String _$switcherCommandsHash() => r'68339f00675186c0f01b7e492fb6d0fd4edb007e';

/// What the local database holds for the switcher while it is open: every stored space,
/// channel (direct messages included), thread, principal, agent card and command, the
/// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
/// changes of the database while the switcher is open.

@ProviderFor(switcherLocalData)
final switcherLocalDataProvider = SwitcherLocalDataFamily._();

/// What the local database holds for the switcher while it is open: every stored space,
/// channel (direct messages included), thread, principal, agent card and command, the
/// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
/// changes of the database while the switcher is open.

final class SwitcherLocalDataProvider
    extends
        $FunctionalProvider<
          SwitcherLocalData?,
          SwitcherLocalData?,
          SwitcherLocalData?
        >
    with $Provider<SwitcherLocalData?> {
  /// What the local database holds for the switcher while it is open: every stored space,
  /// channel (direct messages included), thread, principal, agent card and command, the
  /// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
  /// changes of the database while the switcher is open.
  SwitcherLocalDataProvider._({
    required SwitcherLocalDataFamily super.from,
    required String? super.argument,
  }) : super(
         retry: null,
         name: r'switcherLocalDataProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$switcherLocalDataHash();

  @override
  String toString() {
    return r'switcherLocalDataProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<SwitcherLocalData?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SwitcherLocalData? create(Ref ref) {
    final argument = this.argument as String?;
    return switcherLocalData(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SwitcherLocalData? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SwitcherLocalData?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SwitcherLocalDataProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$switcherLocalDataHash() => r'a47375aab0627e4767d05f08ca5a38e16d8e78d3';

/// What the local database holds for the switcher while it is open: every stored space,
/// channel (direct messages included), thread, principal, agent card and command, the
/// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
/// changes of the database while the switcher is open.

final class SwitcherLocalDataFamily extends $Family
    with $FunctionalFamilyOverride<SwitcherLocalData?, String?> {
  SwitcherLocalDataFamily._()
    : super(
        retry: null,
        name: r'switcherLocalDataProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// What the local database holds for the switcher while it is open: every stored space,
  /// channel (direct messages included), thread, principal, agent card and command, the
  /// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
  /// changes of the database while the switcher is open.

  SwitcherLocalDataProvider call(String? currentChannel) =>
      SwitcherLocalDataProvider._(argument: currentChannel, from: this);

  @override
  String toString() => r'switcherLocalDataProvider';
}

/// The server's prefix search over thread titles, or null before the login (tests override
/// it).

@ProviderFor(threadTitleSearch)
final threadTitleSearchProvider = ThreadTitleSearchProvider._();

/// The server's prefix search over thread titles, or null before the login (tests override
/// it).

final class ThreadTitleSearchProvider
    extends
        $FunctionalProvider<
          ThreadTitleSearch?,
          ThreadTitleSearch?,
          ThreadTitleSearch?
        >
    with $Provider<ThreadTitleSearch?> {
  /// The server's prefix search over thread titles, or null before the login (tests override
  /// it).
  ThreadTitleSearchProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'threadTitleSearchProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$threadTitleSearchHash();

  @$internal
  @override
  $ProviderElement<ThreadTitleSearch?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ThreadTitleSearch? create(Ref ref) {
    return threadTitleSearch(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ThreadTitleSearch? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ThreadTitleSearch?>(value),
    );
  }
}

String _$threadTitleSearchHash() => r'0ab4d5873f19ca09f86f12699f60f82497043240';

/// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
///
/// Asked only while the engine is connected; offline, before the login, for an empty query
/// and when the request fails the list is empty, so the switcher shows its local rows alone
/// (01 6.6: the app stays usable offline).

@ProviderFor(serverThreadTitles)
final serverThreadTitlesProvider = ServerThreadTitlesFamily._();

/// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
///
/// Asked only while the engine is connected; offline, before the login, for an empty query
/// and when the request fails the list is empty, so the switcher shows its local rows alone
/// (01 6.6: the app stays usable offline).

final class ServerThreadTitlesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<ThreadTitleHit>>,
          List<ThreadTitleHit>,
          FutureOr<List<ThreadTitleHit>>
        >
    with
        $FutureModifier<List<ThreadTitleHit>>,
        $FutureProvider<List<ThreadTitleHit>> {
  /// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
  ///
  /// Asked only while the engine is connected; offline, before the login, for an empty query
  /// and when the request fails the list is empty, so the switcher shows its local rows alone
  /// (01 6.6: the app stays usable offline).
  ServerThreadTitlesProvider._({
    required ServerThreadTitlesFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'serverThreadTitlesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$serverThreadTitlesHash();

  @override
  String toString() {
    return r'serverThreadTitlesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<ThreadTitleHit>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<ThreadTitleHit>> create(Ref ref) {
    final argument = this.argument as String;
    return serverThreadTitles(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ServerThreadTitlesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$serverThreadTitlesHash() =>
    r'677dc21e9dd3c67c108e0dd038e2fe02efbf092b';

/// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
///
/// Asked only while the engine is connected; offline, before the login, for an empty query
/// and when the request fails the list is empty, so the switcher shows its local rows alone
/// (01 6.6: the app stays usable offline).

final class ServerThreadTitlesFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<ThreadTitleHit>>, String> {
  ServerThreadTitlesFamily._()
    : super(
        retry: null,
        name: r'serverThreadTitlesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
  ///
  /// Asked only while the engine is connected; offline, before the login, for an empty query
  /// and when the request fails the list is empty, so the switcher shows its local rows alone
  /// (01 6.6: the app stays usable offline).

  ServerThreadTitlesProvider call(String query) =>
      ServerThreadTitlesProvider._(argument: query, from: this);

  @override
  String toString() => r'serverThreadTitlesProvider';
}

/// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
/// local rows and server titles ranked by `rankSwitcher`, at most
/// `ui.quick_switch_max_results`.

@ProviderFor(switcherRows)
final switcherRowsProvider = SwitcherRowsFamily._();

/// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
/// local rows and server titles ranked by `rankSwitcher`, at most
/// `ui.quick_switch_max_results`.

final class SwitcherRowsProvider
    extends
        $FunctionalProvider<
          List<SwitcherEntry>,
          List<SwitcherEntry>,
          List<SwitcherEntry>
        >
    with $Provider<List<SwitcherEntry>> {
  /// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
  /// local rows and server titles ranked by `rankSwitcher`, at most
  /// `ui.quick_switch_max_results`.
  SwitcherRowsProvider._({
    required SwitcherRowsFamily super.from,
    required (String, SwitcherFilter, String?) super.argument,
  }) : super(
         retry: null,
         name: r'switcherRowsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$switcherRowsHash();

  @override
  String toString() {
    return r'switcherRowsProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $ProviderElement<List<SwitcherEntry>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  List<SwitcherEntry> create(Ref ref) {
    final argument = this.argument as (String, SwitcherFilter, String?);
    return switcherRows(ref, argument.$1, argument.$2, argument.$3);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SwitcherEntry> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SwitcherEntry>>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SwitcherRowsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$switcherRowsHash() => r'd71a10a41e22f06248edae487c4072513f592cc1';

/// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
/// local rows and server titles ranked by `rankSwitcher`, at most
/// `ui.quick_switch_max_results`.

final class SwitcherRowsFamily extends $Family
    with
        $FunctionalFamilyOverride<
          List<SwitcherEntry>,
          (String, SwitcherFilter, String?)
        > {
  SwitcherRowsFamily._()
    : super(
        retry: null,
        name: r'switcherRowsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
  /// local rows and server titles ranked by `rankSwitcher`, at most
  /// `ui.quick_switch_max_results`.

  SwitcherRowsProvider call(
    String query,
    SwitcherFilter filter,
    String? currentChannel,
  ) => SwitcherRowsProvider._(
    argument: (query, filter, currentChannel),
    from: this,
  );

  @override
  String toString() => r'switcherRowsProvider';
}

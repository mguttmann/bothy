// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Every stored space.

@ProviderFor(storedSpaces)
final storedSpacesProvider = StoredSpacesProvider._();

/// Every stored space.

final class StoredSpacesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Space>>,
          List<Space>,
          Stream<List<Space>>
        >
    with $FutureModifier<List<Space>>, $StreamProvider<List<Space>> {
  /// Every stored space.
  StoredSpacesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storedSpacesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storedSpacesHash();

  @$internal
  @override
  $StreamProviderElement<List<Space>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Space>> create(Ref ref) {
    return storedSpaces(ref);
  }
}

String _$storedSpacesHash() => r'c050a0f7d1e6b133b78a9120b311e97e836d756e';

/// The person's sidebar state of every space (03 `user_space_states`).

@ProviderFor(userSpaceStates)
final userSpaceStatesProvider = UserSpaceStatesProvider._();

/// The person's sidebar state of every space (03 `user_space_states`).

final class UserSpaceStatesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<UserSpaceState>>,
          List<UserSpaceState>,
          Stream<List<UserSpaceState>>
        >
    with
        $FutureModifier<List<UserSpaceState>>,
        $StreamProvider<List<UserSpaceState>> {
  /// The person's sidebar state of every space (03 `user_space_states`).
  UserSpaceStatesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'userSpaceStatesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$userSpaceStatesHash();

  @$internal
  @override
  $StreamProviderElement<List<UserSpaceState>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<UserSpaceState>> create(Ref ref) {
    return userSpaceStates(ref);
  }
}

String _$userSpaceStatesHash() => r'a7cdcbaadafb3be305edf77bb68511f845078a19';

/// The spaces with the person's state, in the personal order with the global order as the
/// fallback (`orderSpaces`); empty while either source loads.

@ProviderFor(spacesWithState)
final spacesWithStateProvider = SpacesWithStateProvider._();

/// The spaces with the person's state, in the personal order with the global order as the
/// fallback (`orderSpaces`); empty while either source loads.

final class SpacesWithStateProvider
    extends
        $FunctionalProvider<
          List<SidebarSpace>,
          List<SidebarSpace>,
          List<SidebarSpace>
        >
    with $Provider<List<SidebarSpace>> {
  /// The spaces with the person's state, in the personal order with the global order as the
  /// fallback (`orderSpaces`); empty while either source loads.
  SpacesWithStateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'spacesWithStateProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$spacesWithStateHash();

  @$internal
  @override
  $ProviderElement<List<SidebarSpace>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  List<SidebarSpace> create(Ref ref) {
    return spacesWithState(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SidebarSpace> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SidebarSpace>>(value),
    );
  }
}

String _$spacesWithStateHash() => r'74a16bfa966b0826709b7365223854eba5b4433b';

/// The stored channels of [spaceId] in their order.

@ProviderFor(spaceChannels)
final spaceChannelsProvider = SpaceChannelsFamily._();

/// The stored channels of [spaceId] in their order.

final class SpaceChannelsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Channel>>,
          List<Channel>,
          Stream<List<Channel>>
        >
    with $FutureModifier<List<Channel>>, $StreamProvider<List<Channel>> {
  /// The stored channels of [spaceId] in their order.
  SpaceChannelsProvider._({
    required SpaceChannelsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'spaceChannelsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$spaceChannelsHash();

  @override
  String toString() {
    return r'spaceChannelsProvider'
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
    final argument = this.argument as String;
    return spaceChannels(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is SpaceChannelsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$spaceChannelsHash() => r'8e4f983012bc5015b1d20303947dcefc5c59e29e';

/// The stored channels of [spaceId] in their order.

final class SpaceChannelsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Channel>>, String> {
  SpaceChannelsFamily._()
    : super(
        retry: null,
        name: r'spaceChannelsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored channels of [spaceId] in their order.

  SpaceChannelsProvider call(String spaceId) =>
      SpaceChannelsProvider._(argument: spaceId, from: this);

  @override
  String toString() => r'spaceChannelsProvider';
}

/// Every stored read state (the server's counters per thread, 04 14).

@ProviderFor(allReadStates)
final allReadStatesProvider = AllReadStatesProvider._();

/// Every stored read state (the server's counters per thread, 04 14).

final class AllReadStatesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<ReadState>>,
          List<ReadState>,
          Stream<List<ReadState>>
        >
    with $FutureModifier<List<ReadState>>, $StreamProvider<List<ReadState>> {
  /// Every stored read state (the server's counters per thread, 04 14).
  AllReadStatesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'allReadStatesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$allReadStatesHash();

  @$internal
  @override
  $StreamProviderElement<List<ReadState>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<ReadState>> create(Ref ref) {
    return allReadStates(ref);
  }
}

String _$allReadStatesHash() => r'62bb422139c4950628df807c203e9c7c97428fdf';

/// Every notification preference of the person.

@ProviderFor(notificationPreferences)
final notificationPreferencesProvider = NotificationPreferencesProvider._();

/// Every notification preference of the person.

final class NotificationPreferencesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<NotificationPreference>>,
          List<NotificationPreference>,
          Stream<List<NotificationPreference>>
        >
    with
        $FutureModifier<List<NotificationPreference>>,
        $StreamProvider<List<NotificationPreference>> {
  /// Every notification preference of the person.
  NotificationPreferencesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notificationPreferencesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notificationPreferencesHash();

  @$internal
  @override
  $StreamProviderElement<List<NotificationPreference>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<NotificationPreference>> create(Ref ref) {
    return notificationPreferences(ref);
  }
}

String _$notificationPreferencesHash() =>
    r'12c1cf81c00a1e9b7c728f49bb47e9e4abff2298';

/// The runs that have not ended.

@ProviderFor(activeRuns)
final activeRunsProvider = ActiveRunsProvider._();

/// The runs that have not ended.

final class ActiveRunsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RunSummary>>,
          List<RunSummary>,
          Stream<List<RunSummary>>
        >
    with $FutureModifier<List<RunSummary>>, $StreamProvider<List<RunSummary>> {
  /// The runs that have not ended.
  ActiveRunsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'activeRunsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$activeRunsHash();

  @$internal
  @override
  $StreamProviderElement<List<RunSummary>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RunSummary>> create(Ref ref) {
    return activeRuns(ref);
  }
}

String _$activeRunsHash() => r'5c397c2d62f62d4fd18e2ca8a8d6d6309678d39c';

/// A number that changes whenever the engine's counters change (04 14: its own counting of
/// `message.*` between two server values); the counter providers watch it.

@ProviderFor(counterTicks)
final counterTicksProvider = CounterTicksProvider._();

/// A number that changes whenever the engine's counters change (04 14: its own counting of
/// `message.*` between two server values); the counter providers watch it.

final class CounterTicksProvider
    extends $FunctionalProvider<AsyncValue<int>, int, Stream<int>>
    with $FutureModifier<int>, $StreamProvider<int> {
  /// A number that changes whenever the engine's counters change (04 14: its own counting of
  /// `message.*` between two server values); the counter providers watch it.
  CounterTicksProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'counterTicksProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$counterTicksHash();

  @$internal
  @override
  $StreamProviderElement<int> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<int> create(Ref ref) {
    return counterTicks(ref);
  }
}

String _$counterTicksHash() => r'b36dd1e9b6ef39efea0c849f849d5d266e5f5b96';

/// The counters of the threads of [channelId]: the engine's values, else the stored read
/// state (`unread_count` where delivered, `mention_count`).

@ProviderFor(threadCountsOf)
final threadCountsOfProvider = ThreadCountsOfFamily._();

/// The counters of the threads of [channelId]: the engine's values, else the stored read
/// state (`unread_count` where delivered, `mention_count`).

final class ThreadCountsOfProvider
    extends
        $FunctionalProvider<
          Map<ThreadId, ThreadCounts>,
          Map<ThreadId, ThreadCounts>,
          Map<ThreadId, ThreadCounts>
        >
    with $Provider<Map<ThreadId, ThreadCounts>> {
  /// The counters of the threads of [channelId]: the engine's values, else the stored read
  /// state (`unread_count` where delivered, `mention_count`).
  ThreadCountsOfProvider._({
    required ThreadCountsOfFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadCountsOfProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadCountsOfHash();

  @override
  String toString() {
    return r'threadCountsOfProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<Map<ThreadId, ThreadCounts>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  Map<ThreadId, ThreadCounts> create(Ref ref) {
    final argument = this.argument as String;
    return threadCountsOf(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Map<ThreadId, ThreadCounts> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Map<ThreadId, ThreadCounts>>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadCountsOfProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadCountsOfHash() => r'c8a188d73823a5d31e98ed9f4306e151ed625c7b';

/// The counters of the threads of [channelId]: the engine's values, else the stored read
/// state (`unread_count` where delivered, `mention_count`).

final class ThreadCountsOfFamily extends $Family
    with $FunctionalFamilyOverride<Map<ThreadId, ThreadCounts>, String> {
  ThreadCountsOfFamily._()
    : super(
        retry: null,
        name: r'threadCountsOfProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The counters of the threads of [channelId]: the engine's values, else the stored read
  /// state (`unread_count` where delivered, `mention_count`).

  ThreadCountsOfProvider call(String channelId) =>
      ThreadCountsOfProvider._(argument: channelId, from: this);

  @override
  String toString() => r'threadCountsOfProvider';
}

/// The channels whose thread group the person collapsed on this device (01 6.8 item 7; kept in
/// `channel_view_states`, never synchronized, D-SB-6).

@ProviderFor(collapsedGroups)
final collapsedGroupsProvider = CollapsedGroupsProvider._();

/// The channels whose thread group the person collapsed on this device (01 6.8 item 7; kept in
/// `channel_view_states`, never synchronized, D-SB-6).

final class CollapsedGroupsProvider
    extends
        $FunctionalProvider<
          AsyncValue<Set<ChannelId>>,
          Set<ChannelId>,
          Stream<Set<ChannelId>>
        >
    with $FutureModifier<Set<ChannelId>>, $StreamProvider<Set<ChannelId>> {
  /// The channels whose thread group the person collapsed on this device (01 6.8 item 7; kept in
  /// `channel_view_states`, never synchronized, D-SB-6).
  CollapsedGroupsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'collapsedGroupsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$collapsedGroupsHash();

  @$internal
  @override
  $StreamProviderElement<Set<ChannelId>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Set<ChannelId>> create(Ref ref) {
    return collapsedGroups(ref);
  }
}

String _$collapsedGroupsHash() => r'255ff66e0bbc04c5fcae3486af799c68a47d20ac';

/// Whether the pointer is over the sidebar or the keyboard focus is in it: while it holds, a
/// thread row that no longer qualifies stays in place (01 6.8 item 4; brief M4-40 Design note
/// 2). The sidebar sets it ([SidebarHold.set]).

@ProviderFor(SidebarHold)
final sidebarHoldProvider = SidebarHoldProvider._();

/// Whether the pointer is over the sidebar or the keyboard focus is in it: while it holds, a
/// thread row that no longer qualifies stays in place (01 6.8 item 4; brief M4-40 Design note
/// 2). The sidebar sets it ([SidebarHold.set]).
final class SidebarHoldProvider extends $NotifierProvider<SidebarHold, bool> {
  /// Whether the pointer is over the sidebar or the keyboard focus is in it: while it holds, a
  /// thread row that no longer qualifies stays in place (01 6.8 item 4; brief M4-40 Design note
  /// 2). The sidebar sets it ([SidebarHold.set]).
  SidebarHoldProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sidebarHoldProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sidebarHoldHash();

  @$internal
  @override
  SidebarHold create() => SidebarHold();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$sidebarHoldHash() => r'22354f24fab977e10f0a3df0450fadf675a40514';

/// Whether the pointer is over the sidebar or the keyboard focus is in it: while it holds, a
/// thread row that no longer qualifies stays in place (01 6.8 item 4; brief M4-40 Design note
/// 2). The sidebar sets it ([SidebarHold.set]).

abstract class _$SidebarHold extends $Notifier<bool> {
  bool build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<bool, bool>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<bool, bool>,
              bool,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
/// last stay (01 6.8 item 4); additions and changes apply at once.

@ProviderFor(ChannelThreads)
final channelThreadsProvider = ChannelThreadsFamily._();

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
/// last stay (01 6.8 item 4); additions and changes apply at once.
final class ChannelThreadsProvider
    extends $NotifierProvider<ChannelThreads, SidebarThreadGroup?> {
  /// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
  /// threads, the counters per thread, its runs, the selected thread of the route, the person,
  /// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
  /// last stay (01 6.8 item 4); additions and changes apply at once.
  ChannelThreadsProvider._({
    required ChannelThreadsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelThreadsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelThreadsHash();

  @override
  String toString() {
    return r'channelThreadsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ChannelThreads create() => ChannelThreads();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SidebarThreadGroup? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SidebarThreadGroup?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelThreadsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelThreadsHash() => r'94ac2c24601c0e5fa80a5c6b660648ccaf651249';

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
/// last stay (01 6.8 item 4); additions and changes apply at once.

final class ChannelThreadsFamily extends $Family
    with
        $ClassFamilyOverride<
          ChannelThreads,
          SidebarThreadGroup?,
          SidebarThreadGroup?,
          SidebarThreadGroup?,
          String
        > {
  ChannelThreadsFamily._()
    : super(
        retry: null,
        name: r'channelThreadsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
  /// threads, the counters per thread, its runs, the selected thread of the route, the person,
  /// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
  /// last stay (01 6.8 item 4); additions and changes apply at once.

  ChannelThreadsProvider call(String channelId) =>
      ChannelThreadsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelThreadsProvider';
}

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock and the limits of `AppConfig`. While [sidebarHoldProvider] holds, the rows shown
/// last stay (01 6.8 item 4); additions and changes apply at once.

abstract class _$ChannelThreads extends $Notifier<SidebarThreadGroup?> {
  late final _$args = ref.$arg as String;
  String get channelId => _$args;

  SidebarThreadGroup? build(String channelId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SidebarThreadGroup?, SidebarThreadGroup?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SidebarThreadGroup?, SidebarThreadGroup?>,
              SidebarThreadGroup?,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.

@ProviderFor(channelsOfSpace)
final channelsOfSpaceProvider = ChannelsOfSpaceFamily._();

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.

final class ChannelsOfSpaceProvider
    extends
        $FunctionalProvider<
          List<SidebarChannel>,
          List<SidebarChannel>,
          List<SidebarChannel>
        >
    with $Provider<List<SidebarChannel>> {
  /// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
  /// thread group with its collapse state on this device (01 6.8); direct messages and archived
  /// channels are not part of the tree.
  ChannelsOfSpaceProvider._({
    required ChannelsOfSpaceFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelsOfSpaceProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelsOfSpaceHash();

  @override
  String toString() {
    return r'channelsOfSpaceProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<List<SidebarChannel>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  List<SidebarChannel> create(Ref ref) {
    final argument = this.argument as String;
    return channelsOfSpace(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SidebarChannel> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SidebarChannel>>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelsOfSpaceProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelsOfSpaceHash() => r'b993f192e8ef260e2ba7711d53623f82cc7464ca';

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.

final class ChannelsOfSpaceFamily extends $Family
    with $FunctionalFamilyOverride<List<SidebarChannel>, String> {
  ChannelsOfSpaceFamily._()
    : super(
        retry: null,
        name: r'channelsOfSpaceProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
  /// thread group with its collapse state on this device (01 6.8); direct messages and archived
  /// channels are not part of the tree.

  ChannelsOfSpaceProvider call(String spaceId) =>
      ChannelsOfSpaceProvider._(argument: spaceId, from: this);

  @override
  String toString() => r'channelsOfSpaceProvider';
}

/// The whole tree: the spaces in the person's order with their channel rows.

@ProviderFor(sidebarTree)
final sidebarTreeProvider = SidebarTreeProvider._();

/// The whole tree: the spaces in the person's order with their channel rows.

final class SidebarTreeProvider
    extends
        $FunctionalProvider<
          List<SidebarSpaceNode>,
          List<SidebarSpaceNode>,
          List<SidebarSpaceNode>
        >
    with $Provider<List<SidebarSpaceNode>> {
  /// The whole tree: the spaces in the person's order with their channel rows.
  SidebarTreeProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sidebarTreeProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sidebarTreeHash();

  @$internal
  @override
  $ProviderElement<List<SidebarSpaceNode>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  List<SidebarSpaceNode> create(Ref ref) {
    return sidebarTree(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SidebarSpaceNode> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SidebarSpaceNode>>(value),
    );
  }
}

String _$sidebarTreeHash() => r'b68c524e27cfd43f5c0327212ee4a0d534cb5624';

/// The signed-in person as the engine's last snapshot named it, with name and avatar from
/// the local database; null before the first snapshot of this process.

@ProviderFor(ownPrincipal)
final ownPrincipalProvider = OwnPrincipalProvider._();

/// The signed-in person as the engine's last snapshot named it, with name and avatar from
/// the local database; null before the first snapshot of this process.

final class OwnPrincipalProvider
    extends
        $FunctionalProvider<
          AsyncValue<Principal?>,
          Principal?,
          FutureOr<Principal?>
        >
    with $FutureModifier<Principal?>, $FutureProvider<Principal?> {
  /// The signed-in person as the engine's last snapshot named it, with name and avatar from
  /// the local database; null before the first snapshot of this process.
  OwnPrincipalProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'ownPrincipalProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$ownPrincipalHash();

  @$internal
  @override
  $FutureProviderElement<Principal?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Principal?> create(Ref ref) {
    return ownPrincipal(ref);
  }
}

String _$ownPrincipalHash() => r'7183a1e3c3d47894757d243a5b3f419eaada4989';

/// The stored thread [threadId], emitted on every change; null while it is not stored.

@ProviderFor(storedThread)
final storedThreadProvider = StoredThreadFamily._();

/// The stored thread [threadId], emitted on every change; null while it is not stored.

final class StoredThreadProvider
    extends $FunctionalProvider<AsyncValue<Thread?>, Thread?, Stream<Thread?>>
    with $FutureModifier<Thread?>, $StreamProvider<Thread?> {
  /// The stored thread [threadId], emitted on every change; null while it is not stored.
  StoredThreadProvider._({
    required StoredThreadFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storedThreadProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storedThreadHash();

  @override
  String toString() {
    return r'storedThreadProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<Thread?> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<Thread?> create(Ref ref) {
    final argument = this.argument as String;
    return storedThread(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoredThreadProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storedThreadHash() => r'58376258ffa765211baca6d1a69fd6d5057b23e8';

/// The stored thread [threadId], emitted on every change; null while it is not stored.

final class StoredThreadFamily extends $Family
    with $FunctionalFamilyOverride<Stream<Thread?>, String> {
  StoredThreadFamily._()
    : super(
        retry: null,
        name: r'storedThreadProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored thread [threadId], emitted on every change; null while it is not stored.

  StoredThreadProvider call(String threadId) =>
      StoredThreadProvider._(argument: threadId, from: this);

  @override
  String toString() => r'storedThreadProvider';
}

/// The stored channel [channelId], emitted on every change; null while it is not stored.

@ProviderFor(storedChannelOfRoute)
final storedChannelOfRouteProvider = StoredChannelOfRouteFamily._();

/// The stored channel [channelId], emitted on every change; null while it is not stored.

final class StoredChannelOfRouteProvider
    extends
        $FunctionalProvider<AsyncValue<Channel?>, Channel?, Stream<Channel?>>
    with $FutureModifier<Channel?>, $StreamProvider<Channel?> {
  /// The stored channel [channelId], emitted on every change; null while it is not stored.
  StoredChannelOfRouteProvider._({
    required StoredChannelOfRouteFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storedChannelOfRouteProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storedChannelOfRouteHash();

  @override
  String toString() {
    return r'storedChannelOfRouteProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<Channel?> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<Channel?> create(Ref ref) {
    final argument = this.argument as String;
    return storedChannelOfRoute(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoredChannelOfRouteProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storedChannelOfRouteHash() =>
    r'a05505483ac2f2f7f5d6c3befc05af4428c112b9';

/// The stored channel [channelId], emitted on every change; null while it is not stored.

final class StoredChannelOfRouteFamily extends $Family
    with $FunctionalFamilyOverride<Stream<Channel?>, String> {
  StoredChannelOfRouteFamily._()
    : super(
        retry: null,
        name: r'storedChannelOfRouteProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored channel [channelId], emitted on every change; null while it is not stored.

  StoredChannelOfRouteProvider call(String channelId) =>
      StoredChannelOfRouteProvider._(argument: channelId, from: this);

  @override
  String toString() => r'storedChannelOfRouteProvider';
}

/// The last selection a route resolved to; the shell shows it while the selection of a new
/// route loads, so the post list does not leave and come back for a frame.

@ProviderFor(LastRouteSelection)
final lastRouteSelectionProvider = LastRouteSelectionProvider._();

/// The last selection a route resolved to; the shell shows it while the selection of a new
/// route loads, so the post list does not leave and come back for a frame.
final class LastRouteSelectionProvider
    extends $NotifierProvider<LastRouteSelection, RouteSelection> {
  /// The last selection a route resolved to; the shell shows it while the selection of a new
  /// route loads, so the post list does not leave and come back for a frame.
  LastRouteSelectionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'lastRouteSelectionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$lastRouteSelectionHash();

  @$internal
  @override
  LastRouteSelection create() => LastRouteSelection();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RouteSelection value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RouteSelection>(value),
    );
  }
}

String _$lastRouteSelectionHash() =>
    r'ee92503c6f7bedd423e295bc0a923dfd2a04910a';

/// The last selection a route resolved to; the shell shows it while the selection of a new
/// route loads, so the post list does not leave and come back for a frame.

abstract class _$LastRouteSelection extends $Notifier<RouteSelection> {
  RouteSelection build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<RouteSelection, RouteSelection>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<RouteSelection, RouteSelection>,
              RouteSelection,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The selection of [path] (path and query of the router), read from the local database; the
/// selected sidebar row and the post list follow it (brief M4-19 Design note 5).

@ProviderFor(routeSelection)
final routeSelectionProvider = RouteSelectionFamily._();

/// The selection of [path] (path and query of the router), read from the local database; the
/// selected sidebar row and the post list follow it (brief M4-19 Design note 5).

final class RouteSelectionProvider
    extends
        $FunctionalProvider<
          AsyncValue<RouteSelection>,
          RouteSelection,
          FutureOr<RouteSelection>
        >
    with $FutureModifier<RouteSelection>, $FutureProvider<RouteSelection> {
  /// The selection of [path] (path and query of the router), read from the local database; the
  /// selected sidebar row and the post list follow it (brief M4-19 Design note 5).
  RouteSelectionProvider._({
    required RouteSelectionFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'routeSelectionProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$routeSelectionHash();

  @override
  String toString() {
    return r'routeSelectionProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<RouteSelection> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<RouteSelection> create(Ref ref) {
    final argument = this.argument as String;
    return routeSelection(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is RouteSelectionProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$routeSelectionHash() => r'd440e99f2d13185f83e8d49cc223152a206da66d';

/// The selection of [path] (path and query of the router), read from the local database; the
/// selected sidebar row and the post list follow it (brief M4-19 Design note 5).

final class RouteSelectionFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<RouteSelection>, String> {
  RouteSelectionFamily._()
    : super(
        retry: null,
        name: r'routeSelectionProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The selection of [path] (path and query of the router), read from the local database; the
  /// selected sidebar row and the post list follow it (brief M4-19 Design note 5).

  RouteSelectionProvider call(String path) =>
      RouteSelectionProvider._(argument: path, from: this);

  @override
  String toString() => r'routeSelectionProvider';
}

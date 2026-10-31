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

/// The spaces collapsed in the sidebar on this device (01 6.7, R-PR-24; kept in
/// `space_view_states`, never synchronized).

@ProviderFor(collapsedSpaces)
final collapsedSpacesProvider = CollapsedSpacesProvider._();

/// The spaces collapsed in the sidebar on this device (01 6.7, R-PR-24; kept in
/// `space_view_states`, never synchronized).

final class CollapsedSpacesProvider
    extends
        $FunctionalProvider<
          AsyncValue<Set<SpaceId>>,
          Set<SpaceId>,
          Stream<Set<SpaceId>>
        >
    with $FutureModifier<Set<SpaceId>>, $StreamProvider<Set<SpaceId>> {
  /// The spaces collapsed in the sidebar on this device (01 6.7, R-PR-24; kept in
  /// `space_view_states`, never synchronized).
  CollapsedSpacesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'collapsedSpacesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$collapsedSpacesHash();

  @$internal
  @override
  $StreamProviderElement<Set<SpaceId>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Set<SpaceId>> create(Ref ref) {
    return collapsedSpaces(ref);
  }
}

String _$collapsedSpacesHash() => r'23495b56aa2aba7da077c5770e1a6d918df9fc5c';

/// The spaces with the person's state, collapsed as on this device; empty while either source
/// loads.
///
/// The order is the personal one with the global order as the fallback (`orderSpaces`), or, with
/// "Arrange spaces automatically" (01 6.16 item 4, D-M4-51-4), the newest activity of each
/// space's channels first with the personal order breaking ties; an automatic order holds while
/// [sidebarHoldProvider] holds, so no space moves under the pointer.

@ProviderFor(SpacesWithState)
final spacesWithStateProvider = SpacesWithStateProvider._();

/// The spaces with the person's state, collapsed as on this device; empty while either source
/// loads.
///
/// The order is the personal one with the global order as the fallback (`orderSpaces`), or, with
/// "Arrange spaces automatically" (01 6.16 item 4, D-M4-51-4), the newest activity of each
/// space's channels first with the personal order breaking ties; an automatic order holds while
/// [sidebarHoldProvider] holds, so no space moves under the pointer.
final class SpacesWithStateProvider
    extends $NotifierProvider<SpacesWithState, List<SidebarSpace>> {
  /// The spaces with the person's state, collapsed as on this device; empty while either source
  /// loads.
  ///
  /// The order is the personal one with the global order as the fallback (`orderSpaces`), or, with
  /// "Arrange spaces automatically" (01 6.16 item 4, D-M4-51-4), the newest activity of each
  /// space's channels first with the personal order breaking ties; an automatic order holds while
  /// [sidebarHoldProvider] holds, so no space moves under the pointer.
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
  SpacesWithState create() => SpacesWithState();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SidebarSpace> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SidebarSpace>>(value),
    );
  }
}

String _$spacesWithStateHash() => r'5f7a597389f63be12e6b7cb60b0f373575d7f939';

/// The spaces with the person's state, collapsed as on this device; empty while either source
/// loads.
///
/// The order is the personal one with the global order as the fallback (`orderSpaces`), or, with
/// "Arrange spaces automatically" (01 6.16 item 4, D-M4-51-4), the newest activity of each
/// space's channels first with the personal order breaking ties; an automatic order holds while
/// [sidebarHoldProvider] holds, so no space moves under the pointer.

abstract class _$SpacesWithState extends $Notifier<List<SidebarSpace>> {
  List<SidebarSpace> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<List<SidebarSpace>, List<SidebarSpace>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<List<SidebarSpace>, List<SidebarSpace>>,
              List<SidebarSpace>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
/// `last_activity_at` of its stored threads, the main history included, else its creation time;
/// the epoch while the channel is not stored.

@ProviderFor(channelActivityOf)
final channelActivityOfProvider = ChannelActivityOfFamily._();

/// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
/// `last_activity_at` of its stored threads, the main history included, else its creation time;
/// the epoch while the channel is not stored.

final class ChannelActivityOfProvider
    extends $FunctionalProvider<DateTime, DateTime, DateTime>
    with $Provider<DateTime> {
  /// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
  /// `last_activity_at` of its stored threads, the main history included, else its creation time;
  /// the epoch while the channel is not stored.
  ChannelActivityOfProvider._({
    required ChannelActivityOfFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelActivityOfProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelActivityOfHash();

  @override
  String toString() {
    return r'channelActivityOfProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<DateTime> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DateTime create(Ref ref) {
    final argument = this.argument as String;
    return channelActivityOf(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DateTime value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DateTime>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelActivityOfProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelActivityOfHash() => r'91ca55879a2e359fdbbbc06528f2c69586d28e58';

/// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
/// `last_activity_at` of its stored threads, the main history included, else its creation time;
/// the epoch while the channel is not stored.

final class ChannelActivityOfFamily extends $Family
    with $FunctionalFamilyOverride<DateTime, String> {
  ChannelActivityOfFamily._()
    : super(
        retry: null,
        name: r'channelActivityOfProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
  /// `last_activity_at` of its stored threads, the main history included, else its creation time;
  /// the epoch while the channel is not stored.

  ChannelActivityOfProvider call(String channelId) =>
      ChannelActivityOfProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelActivityOfProvider';
}

/// The person's sidebar state of every channel (03 `user_channel_states`): the manual order of
/// a forum's posts (01 6.16 item 3).

@ProviderFor(userChannelStates)
final userChannelStatesProvider = UserChannelStatesProvider._();

/// The person's sidebar state of every channel (03 `user_channel_states`): the manual order of
/// a forum's posts (01 6.16 item 3).

final class UserChannelStatesProvider
    extends
        $FunctionalProvider<
          AsyncValue<Map<ChannelId, UserChannelState>>,
          Map<ChannelId, UserChannelState>,
          Stream<Map<ChannelId, UserChannelState>>
        >
    with
        $FutureModifier<Map<ChannelId, UserChannelState>>,
        $StreamProvider<Map<ChannelId, UserChannelState>> {
  /// The person's sidebar state of every channel (03 `user_channel_states`): the manual order of
  /// a forum's posts (01 6.16 item 3).
  UserChannelStatesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'userChannelStatesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$userChannelStatesHash();

  @$internal
  @override
  $StreamProviderElement<Map<ChannelId, UserChannelState>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Map<ChannelId, UserChannelState>> create(Ref ref) {
    return userChannelStates(ref);
  }
}

String _$userChannelStatesHash() => r'90727bd3e8f04b4f3e7d39ab40cc9c2b7c55e32c';

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

/// The forums whose group lists every open post on this device ("Show more", 01 6.8 item 3;
/// kept in `channel_view_states`, never synchronized, D-M4-46-2).

@ProviderFor(postListsShownInFull)
final postListsShownInFullProvider = PostListsShownInFullProvider._();

/// The forums whose group lists every open post on this device ("Show more", 01 6.8 item 3;
/// kept in `channel_view_states`, never synchronized, D-M4-46-2).

final class PostListsShownInFullProvider
    extends
        $FunctionalProvider<
          AsyncValue<Set<ChannelId>>,
          Set<ChannelId>,
          Stream<Set<ChannelId>>
        >
    with $FutureModifier<Set<ChannelId>>, $StreamProvider<Set<ChannelId>> {
  /// The forums whose group lists every open post on this device ("Show more", 01 6.8 item 3;
  /// kept in `channel_view_states`, never synchronized, D-M4-46-2).
  PostListsShownInFullProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'postListsShownInFullProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$postListsShownInFullHash();

  @$internal
  @override
  $StreamProviderElement<Set<ChannelId>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Set<ChannelId>> create(Ref ref) {
    return postListsShownInFull(ref);
  }
}

String _$postListsShownInFullHash() =>
    r'e1dd49cc9929430ff66a7f47b28dfa6663c5a6cb';

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
/// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
/// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
/// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
/// once too: the person asked for them.

@ProviderFor(ChannelThreads)
final channelThreadsProvider = ChannelThreadsFamily._();

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
/// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
/// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
/// once too: the person asked for them.
final class ChannelThreadsProvider
    extends $NotifierProvider<ChannelThreads, SidebarThreadGroup?> {
  /// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
  /// threads, the counters per thread, its runs, the selected thread of the route, the person,
  /// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
  /// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
  /// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
  /// once too: the person asked for them.
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

String _$channelThreadsHash() => r'aef2524fee080a0c227b4de3388661d13a141574';

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
/// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
/// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
/// once too: the person asked for them.

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
  /// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
  /// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
  /// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
  /// once too: the person asked for them.

  ChannelThreadsProvider call(String channelId) =>
      ChannelThreadsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelThreadsProvider';
}

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
/// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
/// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
/// once too: the person asked for them.

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

/// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
/// draft row while the person's draft of a new post has text (the unsent write of this device,
/// else the stored draft) or the new chat view of the forum is open and writing, then the posts
/// sent from this device that the server has not created yet, newest first. Empty for other
/// channels.

@ProviderFor(newChatRows)
final newChatRowsProvider = NewChatRowsFamily._();

/// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
/// draft row while the person's draft of a new post has text (the unsent write of this device,
/// else the stored draft) or the new chat view of the forum is open and writing, then the posts
/// sent from this device that the server has not created yet, newest first. Empty for other
/// channels.

final class NewChatRowsProvider
    extends
        $FunctionalProvider<
          List<SidebarNewChatRow>,
          List<SidebarNewChatRow>,
          List<SidebarNewChatRow>
        >
    with $Provider<List<SidebarNewChatRow>> {
  /// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
  /// draft row while the person's draft of a new post has text (the unsent write of this device,
  /// else the stored draft) or the new chat view of the forum is open and writing, then the posts
  /// sent from this device that the server has not created yet, newest first. Empty for other
  /// channels.
  NewChatRowsProvider._({
    required NewChatRowsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'newChatRowsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$newChatRowsHash();

  @override
  String toString() {
    return r'newChatRowsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<List<SidebarNewChatRow>> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  List<SidebarNewChatRow> create(Ref ref) {
    final argument = this.argument as String;
    return newChatRows(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<SidebarNewChatRow> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<SidebarNewChatRow>>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is NewChatRowsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$newChatRowsHash() => r'ac5330fbdab927b8020b6e42088e740bcb141ee9';

/// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
/// draft row while the person's draft of a new post has text (the unsent write of this device,
/// else the stored draft) or the new chat view of the forum is open and writing, then the posts
/// sent from this device that the server has not created yet, newest first. Empty for other
/// channels.

final class NewChatRowsFamily extends $Family
    with $FunctionalFamilyOverride<List<SidebarNewChatRow>, String> {
  NewChatRowsFamily._()
    : super(
        retry: null,
        name: r'newChatRowsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
  /// draft row while the person's draft of a new post has text (the unsent write of this device,
  /// else the stored draft) or the new chat view of the forum is open and writing, then the posts
  /// sent from this device that the server has not created yet, newest first. Empty for other
  /// channels.

  NewChatRowsProvider call(String channelId) =>
      NewChatRowsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'newChatRowsProvider';
}

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.
///
/// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
/// item 3), or, with "Arrange channels and projects automatically", the newest activity first
/// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).

@ProviderFor(ChannelsOfSpace)
final channelsOfSpaceProvider = ChannelsOfSpaceFamily._();

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.
///
/// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
/// item 3), or, with "Arrange channels and projects automatically", the newest activity first
/// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).
final class ChannelsOfSpaceProvider
    extends $NotifierProvider<ChannelsOfSpace, List<SidebarChannel>> {
  /// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
  /// thread group with its collapse state on this device (01 6.8); direct messages and archived
  /// channels are not part of the tree.
  ///
  /// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
  /// item 3), or, with "Arrange channels and projects automatically", the newest activity first
  /// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).
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
  ChannelsOfSpace create() => ChannelsOfSpace();

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

String _$channelsOfSpaceHash() => r'680367bf20456d5e40c43d185f5e84820bf21817';

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.
///
/// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
/// item 3), or, with "Arrange channels and projects automatically", the newest activity first
/// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).

final class ChannelsOfSpaceFamily extends $Family
    with
        $ClassFamilyOverride<
          ChannelsOfSpace,
          List<SidebarChannel>,
          List<SidebarChannel>,
          List<SidebarChannel>,
          String
        > {
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
  ///
  /// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
  /// item 3), or, with "Arrange channels and projects automatically", the newest activity first
  /// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).

  ChannelsOfSpaceProvider call(String spaceId) =>
      ChannelsOfSpaceProvider._(argument: spaceId, from: this);

  @override
  String toString() => r'channelsOfSpaceProvider';
}

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state and the
/// thread group with its collapse state on this device (01 6.8); direct messages and archived
/// channels are not part of the tree.
///
/// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
/// item 3), or, with "Arrange channels and projects automatically", the newest activity first
/// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).

abstract class _$ChannelsOfSpace extends $Notifier<List<SidebarChannel>> {
  late final _$args = ref.$arg as String;
  String get spaceId => _$args;

  List<SidebarChannel> build(String spaceId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<List<SidebarChannel>, List<SidebarChannel>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<List<SidebarChannel>, List<SidebarChannel>>,
              List<SidebarChannel>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
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
    r'18e6db35eaf5092f8a572ab73bfa419e00df7c17';

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

String _$routeSelectionHash() => r'26babfa6b918152606bb5299d1cb167beb103cba';

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

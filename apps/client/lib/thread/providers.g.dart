// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The REST reads of the thread view; tests override it.

@ProviderFor(threadReads)
final threadReadsProvider = ThreadReadsProvider._();

/// The REST reads of the thread view; tests override it.

final class ThreadReadsProvider
    extends $FunctionalProvider<ThreadReads, ThreadReads, ThreadReads>
    with $Provider<ThreadReads> {
  /// The REST reads of the thread view; tests override it.
  ThreadReadsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'threadReadsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$threadReadsHash();

  @$internal
  @override
  $ProviderElement<ThreadReads> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ThreadReads create(Ref ref) {
    return threadReads(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ThreadReads value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ThreadReads>(value),
    );
  }
}

String _$threadReadsHash() => r'adcf2969c4dd6195c622c338d48310c7be6661aa';

/// The signed-in person, or null before the first snapshot.

@ProviderFor(myPrincipalId)
final myPrincipalIdProvider = MyPrincipalIdProvider._();

/// The signed-in person, or null before the first snapshot.

final class MyPrincipalIdProvider
    extends $FunctionalProvider<PrincipalId?, PrincipalId?, PrincipalId?>
    with $Provider<PrincipalId?> {
  /// The signed-in person, or null before the first snapshot.
  MyPrincipalIdProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'myPrincipalIdProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$myPrincipalIdHash();

  @$internal
  @override
  $ProviderElement<PrincipalId?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  PrincipalId? create(Ref ref) {
    return myPrincipalId(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PrincipalId? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PrincipalId?>(value),
    );
  }
}

String _$myPrincipalIdHash() => r'9cb638119adc08fa1c45d307152ad1fc47d21c23';

/// The caller's level and capabilities in [channelId] as the server reported them.

@ProviderFor(channelAccess)
final channelAccessProvider = ChannelAccessFamily._();

/// The caller's level and capabilities in [channelId] as the server reported them.

final class ChannelAccessProvider
    extends
        $FunctionalProvider<
          AsyncValue<AccessView?>,
          AccessView?,
          Stream<AccessView?>
        >
    with $FutureModifier<AccessView?>, $StreamProvider<AccessView?> {
  /// The caller's level and capabilities in [channelId] as the server reported them.
  ChannelAccessProvider._({
    required ChannelAccessFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelAccessProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelAccessHash();

  @override
  String toString() {
    return r'channelAccessProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<AccessView?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<AccessView?> create(Ref ref) {
    final argument = this.argument as String;
    return channelAccess(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelAccessProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelAccessHash() => r'3782ce2f11c41708c6db370891065887c09cca22';

/// The caller's level and capabilities in [channelId] as the server reported them.

final class ChannelAccessFamily extends $Family
    with $FunctionalFamilyOverride<Stream<AccessView?>, String> {
  ChannelAccessFamily._()
    : super(
        retry: null,
        name: r'channelAccessProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The caller's level and capabilities in [channelId] as the server reported them.

  ChannelAccessProvider call(String channelId) =>
      ChannelAccessProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelAccessProvider';
}

/// Every stored principal by ID (names, handles and the mention suggestions).

@ProviderFor(principalsById)
final principalsByIdProvider = PrincipalsByIdProvider._();

/// Every stored principal by ID (names, handles and the mention suggestions).

final class PrincipalsByIdProvider
    extends
        $FunctionalProvider<
          AsyncValue<Map<PrincipalId, Principal>>,
          Map<PrincipalId, Principal>,
          Stream<Map<PrincipalId, Principal>>
        >
    with
        $FutureModifier<Map<PrincipalId, Principal>>,
        $StreamProvider<Map<PrincipalId, Principal>> {
  /// Every stored principal by ID (names, handles and the mention suggestions).
  PrincipalsByIdProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'principalsByIdProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$principalsByIdHash();

  @$internal
  @override
  $StreamProviderElement<Map<PrincipalId, Principal>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Map<PrincipalId, Principal>> create(Ref ref) {
    return principalsById(ref);
  }
}

String _$principalsByIdHash() => r'71f2b90af722492b5afe471eb610d213cedf497c';

/// The limits of the instance (`getInstance`, D-W-115), or null while unknown.

@ProviderFor(instanceLimits)
final instanceLimitsProvider = InstanceLimitsProvider._();

/// The limits of the instance (`getInstance`, D-W-115), or null while unknown.

final class InstanceLimitsProvider
    extends
        $FunctionalProvider<
          AsyncValue<InstanceLimits?>,
          InstanceLimits?,
          Stream<InstanceLimits?>
        >
    with $FutureModifier<InstanceLimits?>, $StreamProvider<InstanceLimits?> {
  /// The limits of the instance (`getInstance`, D-W-115), or null while unknown.
  InstanceLimitsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'instanceLimitsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$instanceLimitsHash();

  @$internal
  @override
  $StreamProviderElement<InstanceLimits?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<InstanceLimits?> create(Ref ref) {
    return instanceLimits(ref);
  }
}

String _$instanceLimitsHash() => r'4c30ff09691846459e5eb9279d1ae828dfe62752';

/// How many messages the history of [threadId] shows: one page, more after paging up or a
/// jump; it lasts while the app runs, so coming back keeps the loaded range in view.

@ProviderFor(HistoryLimit)
final historyLimitProvider = HistoryLimitFamily._();

/// How many messages the history of [threadId] shows: one page, more after paging up or a
/// jump; it lasts while the app runs, so coming back keeps the loaded range in view.
final class HistoryLimitProvider extends $NotifierProvider<HistoryLimit, int> {
  /// How many messages the history of [threadId] shows: one page, more after paging up or a
  /// jump; it lasts while the app runs, so coming back keeps the loaded range in view.
  HistoryLimitProvider._({
    required HistoryLimitFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'historyLimitProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$historyLimitHash();

  @override
  String toString() {
    return r'historyLimitProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  HistoryLimit create() => HistoryLimit();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is HistoryLimitProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$historyLimitHash() => r'25be2b7229d93aa41f6ed1d26b8b6f71481cb572';

/// How many messages the history of [threadId] shows: one page, more after paging up or a
/// jump; it lasts while the app runs, so coming back keeps the loaded range in view.

final class HistoryLimitFamily extends $Family
    with $ClassFamilyOverride<HistoryLimit, int, int, int, String> {
  HistoryLimitFamily._()
    : super(
        retry: null,
        name: r'historyLimitProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// How many messages the history of [threadId] shows: one page, more after paging up or a
  /// jump; it lasts while the app runs, so coming back keeps the loaded range in view.

  HistoryLimitProvider call(String threadId) =>
      HistoryLimitProvider._(argument: threadId, from: this);

  @override
  String toString() => r'historyLimitProvider';
}

/// How many messages the history of [threadId] shows: one page, more after paging up or a
/// jump; it lasts while the app runs, so coming back keeps the loaded range in view.

abstract class _$HistoryLimit extends $Notifier<int> {
  late final _$args = ref.$arg as String;
  String get threadId => _$args;

  int build(String threadId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}

/// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
/// gaps before and after them.

@ProviderFor(threadWindow)
final threadWindowProvider = ThreadWindowFamily._();

/// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
/// gaps before and after them.

final class ThreadWindowProvider
    extends
        $FunctionalProvider<
          AsyncValue<ContiguousWindow>,
          ContiguousWindow,
          Stream<ContiguousWindow>
        >
    with $FutureModifier<ContiguousWindow>, $StreamProvider<ContiguousWindow> {
  /// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
  /// gaps before and after them.
  ThreadWindowProvider._({
    required ThreadWindowFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadWindowProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadWindowHash();

  @override
  String toString() {
    return r'threadWindowProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<ContiguousWindow> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<ContiguousWindow> create(Ref ref) {
    final argument = this.argument as String;
    return threadWindow(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadWindowProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadWindowHash() => r'1bf3981666cebca25f199ae79b34f7482be72684';

/// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
/// gaps before and after them.

final class ThreadWindowFamily extends $Family
    with $FunctionalFamilyOverride<Stream<ContiguousWindow>, String> {
  ThreadWindowFamily._()
    : super(
        retry: null,
        name: r'threadWindowProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
  /// gaps before and after them.

  ThreadWindowProvider call(String threadId) =>
      ThreadWindowProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadWindowProvider';
}

/// The outbox entries of [threadId] that are not confirmed, with their display state
/// (04 13.2 item 3).

@ProviderFor(threadOutbox)
final threadOutboxProvider = ThreadOutboxFamily._();

/// The outbox entries of [threadId] that are not confirmed, with their display state
/// (04 13.2 item 3).

final class ThreadOutboxProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<OutboxEntryView>>,
          List<OutboxEntryView>,
          Stream<List<OutboxEntryView>>
        >
    with
        $FutureModifier<List<OutboxEntryView>>,
        $StreamProvider<List<OutboxEntryView>> {
  /// The outbox entries of [threadId] that are not confirmed, with their display state
  /// (04 13.2 item 3).
  ThreadOutboxProvider._({
    required ThreadOutboxFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadOutboxProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadOutboxHash();

  @override
  String toString() {
    return r'threadOutboxProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<OutboxEntryView>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<OutboxEntryView>> create(Ref ref) {
    final argument = this.argument as String;
    return threadOutbox(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadOutboxProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadOutboxHash() => r'ec67b53030a083697215af42bcd7eb686bde2ba1';

/// The outbox entries of [threadId] that are not confirmed, with their display state
/// (04 13.2 item 3).

final class ThreadOutboxFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<OutboxEntryView>>, String> {
  ThreadOutboxFamily._()
    : super(
        retry: null,
        name: r'threadOutboxProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The outbox entries of [threadId] that are not confirmed, with their display state
  /// (04 13.2 item 3).

  ThreadOutboxProvider call(String threadId) =>
      ThreadOutboxProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadOutboxProvider';
}

/// The pinned messages of [threadId] (the section "Pinned" of the session column).

@ProviderFor(pinnedMessages)
final pinnedMessagesProvider = PinnedMessagesFamily._();

/// The pinned messages of [threadId] (the section "Pinned" of the session column).

final class PinnedMessagesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Message>>,
          List<Message>,
          Stream<List<Message>>
        >
    with $FutureModifier<List<Message>>, $StreamProvider<List<Message>> {
  /// The pinned messages of [threadId] (the section "Pinned" of the session column).
  PinnedMessagesProvider._({
    required PinnedMessagesFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'pinnedMessagesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$pinnedMessagesHash();

  @override
  String toString() {
    return r'pinnedMessagesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Message>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Message>> create(Ref ref) {
    final argument = this.argument as String;
    return pinnedMessages(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is PinnedMessagesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$pinnedMessagesHash() => r'14b8ab4e4d904a6777f5041c4eda2ef07825bf4c';

/// The pinned messages of [threadId] (the section "Pinned" of the session column).

final class PinnedMessagesFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Message>>, String> {
  PinnedMessagesFamily._()
    : super(
        retry: null,
        name: r'pinnedMessagesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The pinned messages of [threadId] (the section "Pinned" of the session column).

  PinnedMessagesProvider call(String threadId) =>
      PinnedMessagesProvider._(argument: threadId, from: this);

  @override
  String toString() => r'pinnedMessagesProvider';
}

/// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.

@ProviderFor(typingIn)
final typingInProvider = TypingInFamily._();

/// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.

final class TypingInProvider
    extends
        $FunctionalProvider<
          AsyncValue<Set<PrincipalId>>,
          Set<PrincipalId>,
          Stream<Set<PrincipalId>>
        >
    with $FutureModifier<Set<PrincipalId>>, $StreamProvider<Set<PrincipalId>> {
  /// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.
  TypingInProvider._({
    required TypingInFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'typingInProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$typingInHash();

  @override
  String toString() {
    return r'typingInProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<Set<PrincipalId>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<Set<PrincipalId>> create(Ref ref) {
    final argument = this.argument as String;
    return typingIn(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is TypingInProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$typingInHash() => r'e36f9134fc0da6943e978b6f49b864d4a50b289a';

/// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.

final class TypingInFamily extends $Family
    with $FunctionalFamilyOverride<Stream<Set<PrincipalId>>, String> {
  TypingInFamily._()
    : super(
        retry: null,
        name: r'typingInProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.

  TypingInProvider call(String threadId) =>
      TypingInProvider._(argument: threadId, from: this);

  @override
  String toString() => r'typingInProvider';
}

/// Whether the app window is in the foreground; the history marks read and focuses only then
/// (brief M4-20 Design note 2: `idle` while the window has no focus).

@ProviderFor(windowFocused)
final windowFocusedProvider = WindowFocusedProvider._();

/// Whether the app window is in the foreground; the history marks read and focuses only then
/// (brief M4-20 Design note 2: `idle` while the window has no focus).

final class WindowFocusedProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, Stream<bool>>
    with $FutureModifier<bool>, $StreamProvider<bool> {
  /// Whether the app window is in the foreground; the history marks read and focuses only then
  /// (brief M4-20 Design note 2: `idle` while the window has no focus).
  WindowFocusedProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'windowFocusedProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$windowFocusedHash();

  @$internal
  @override
  $StreamProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<bool> create(Ref ref) {
    return windowFocused(ref);
  }
}

String _$windowFocusedHash() => r'9a27c975bea15986a92e5e8ca0589082a38ebf4b';

/// The engine's notices to the app (removed targets, an overwritten draft).

@ProviderFor(syncNotices)
final syncNoticesProvider = SyncNoticesProvider._();

/// The engine's notices to the app (removed targets, an overwritten draft).

final class SyncNoticesProvider
    extends
        $FunctionalProvider<
          AsyncValue<SyncNotice>,
          SyncNotice,
          Stream<SyncNotice>
        >
    with $FutureModifier<SyncNotice>, $StreamProvider<SyncNotice> {
  /// The engine's notices to the app (removed targets, an overwritten draft).
  SyncNoticesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'syncNoticesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$syncNoticesHash();

  @$internal
  @override
  $StreamProviderElement<SyncNotice> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<SyncNotice> create(Ref ref) {
    return syncNotices(ref);
  }
}

String _$syncNoticesHash() => r'ac344e9a75159ce5be75ff7c0d0d459792cd3012';

/// The stored read state of [threadId], or null.

@ProviderFor(threadReadState)
final threadReadStateProvider = ThreadReadStateFamily._();

/// The stored read state of [threadId], or null.

final class ThreadReadStateProvider
    extends $FunctionalProvider<ReadState?, ReadState?, ReadState?>
    with $Provider<ReadState?> {
  /// The stored read state of [threadId], or null.
  ThreadReadStateProvider._({
    required ThreadReadStateFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadReadStateProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadReadStateHash();

  @override
  String toString() {
    return r'threadReadStateProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<ReadState?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ReadState? create(Ref ref) {
    final argument = this.argument as String;
    return threadReadState(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ReadState? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ReadState?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadReadStateProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadReadStateHash() => r'72464c5d70c7539a8360ef645c3cdde914197ab2';

/// The stored read state of [threadId], or null.

final class ThreadReadStateFamily extends $Family
    with $FunctionalFamilyOverride<ReadState?, String> {
  ThreadReadStateFamily._()
    : super(
        retry: null,
        name: r'threadReadStateProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored read state of [threadId], or null.

  ThreadReadStateProvider call(String threadId) =>
      ThreadReadStateProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadReadStateProvider';
}

/// The stored draft of [threadId] in [channelId], or null.

@ProviderFor(threadDraft)
final threadDraftProvider = ThreadDraftFamily._();

/// The stored draft of [threadId] in [channelId], or null.

final class ThreadDraftProvider
    extends $FunctionalProvider<AsyncValue<Draft?>, Draft?, Stream<Draft?>>
    with $FutureModifier<Draft?>, $StreamProvider<Draft?> {
  /// The stored draft of [threadId] in [channelId], or null.
  ThreadDraftProvider._({
    required ThreadDraftFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'threadDraftProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadDraftHash();

  @override
  String toString() {
    return r'threadDraftProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $StreamProviderElement<Draft?> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<Draft?> create(Ref ref) {
    final argument = this.argument as (String, String);
    return threadDraft(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadDraftProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadDraftHash() => r'23e61a6f5b22b2c7b0dabab76d53e2f02055f33d';

/// The stored draft of [threadId] in [channelId], or null.

final class ThreadDraftFamily extends $Family
    with $FunctionalFamilyOverride<Stream<Draft?>, (String, String)> {
  ThreadDraftFamily._()
    : super(
        retry: null,
        name: r'threadDraftProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored draft of [threadId] in [channelId], or null.

  ThreadDraftProvider call(String channelId, String threadId) =>
      ThreadDraftProvider._(argument: (channelId, threadId), from: this);

  @override
  String toString() => r'threadDraftProvider';
}

/// The personal templates and those of [channelId] (01 3.3), read when the menu opens.

@ProviderFor(templateLists)
final templateListsProvider = TemplateListsFamily._();

/// The personal templates and those of [channelId] (01 3.3), read when the menu opens.

final class TemplateListsProvider
    extends
        $FunctionalProvider<
          AsyncValue<TemplateLists>,
          TemplateLists,
          FutureOr<TemplateLists>
        >
    with $FutureModifier<TemplateLists>, $FutureProvider<TemplateLists> {
  /// The personal templates and those of [channelId] (01 3.3), read when the menu opens.
  TemplateListsProvider._({
    required TemplateListsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'templateListsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$templateListsHash();

  @override
  String toString() {
    return r'templateListsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<TemplateLists> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<TemplateLists> create(Ref ref) {
    final argument = this.argument as String;
    return templateLists(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is TemplateListsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$templateListsHash() => r'199aae7f130f4f7850a281e2c35f7a6851e91797';

/// The personal templates and those of [channelId] (01 3.3), read when the menu opens.

final class TemplateListsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<TemplateLists>, String> {
  TemplateListsFamily._()
    : super(
        retry: null,
        name: r'templateListsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The personal templates and those of [channelId] (01 3.3), read when the menu opens.

  TemplateListsProvider call(String channelId) =>
      TemplateListsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'templateListsProvider';
}

/// The drafts being edited on this device: the registry of the signed-in instance, or a
/// registry of its own without one (tests).

@ProviderFor(draftEditing)
final draftEditingProvider = DraftEditingProvider._();

/// The drafts being edited on this device: the registry of the signed-in instance, or a
/// registry of its own without one (tests).

final class DraftEditingProvider
    extends
        $FunctionalProvider<
          DraftEditingRegistry,
          DraftEditingRegistry,
          DraftEditingRegistry
        >
    with $Provider<DraftEditingRegistry> {
  /// The drafts being edited on this device: the registry of the signed-in instance, or a
  /// registry of its own without one (tests).
  DraftEditingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'draftEditingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$draftEditingHash();

  @$internal
  @override
  $ProviderElement<DraftEditingRegistry> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DraftEditingRegistry create(Ref ref) {
    return draftEditing(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DraftEditingRegistry value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DraftEditingRegistry>(value),
    );
  }
}

String _$draftEditingHash() => r'2b5ecf8e8bd0399c4574add5ad6bea7e392e0aad';

/// Whether the session column shows (the header button "Column", 01 5.1 D1); it lasts while
/// the app runs.

@ProviderFor(SessionColumnShown)
final sessionColumnShownProvider = SessionColumnShownProvider._();

/// Whether the session column shows (the header button "Column", 01 5.1 D1); it lasts while
/// the app runs.
final class SessionColumnShownProvider
    extends $NotifierProvider<SessionColumnShown, bool> {
  /// Whether the session column shows (the header button "Column", 01 5.1 D1); it lasts while
  /// the app runs.
  SessionColumnShownProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sessionColumnShownProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sessionColumnShownHash();

  @$internal
  @override
  SessionColumnShown create() => SessionColumnShown();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$sessionColumnShownHash() =>
    r'a2922dc514801271d805ea52a2dcb7ff7cb5f3f8';

/// Whether the session column shows (the header button "Column", 01 5.1 D1); it lasts while
/// the app runs.

abstract class _$SessionColumnShown extends $Notifier<bool> {
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

/// The [ThreadPermissions] of [channelId].

@ProviderFor(threadPermissions)
final threadPermissionsProvider = ThreadPermissionsFamily._();

/// The [ThreadPermissions] of [channelId].

final class ThreadPermissionsProvider
    extends
        $FunctionalProvider<
          ThreadPermissions,
          ThreadPermissions,
          ThreadPermissions
        >
    with $Provider<ThreadPermissions> {
  /// The [ThreadPermissions] of [channelId].
  ThreadPermissionsProvider._({
    required ThreadPermissionsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadPermissionsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadPermissionsHash();

  @override
  String toString() {
    return r'threadPermissionsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<ThreadPermissions> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ThreadPermissions create(Ref ref) {
    final argument = this.argument as String;
    return threadPermissions(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ThreadPermissions value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ThreadPermissions>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadPermissionsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadPermissionsHash() => r'f3aef45150a7c7c53a8930d1c3b8df81a2ce7c5d';

/// The [ThreadPermissions] of [channelId].

final class ThreadPermissionsFamily extends $Family
    with $FunctionalFamilyOverride<ThreadPermissions, String> {
  ThreadPermissionsFamily._()
    : super(
        retry: null,
        name: r'threadPermissionsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The [ThreadPermissions] of [channelId].

  ThreadPermissionsProvider call(String channelId) =>
      ThreadPermissionsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'threadPermissionsProvider';
}

/// The visible channels that are no direct message and not archived, from the sidebar's
/// tree (move targets, 01 6.7).

@ProviderFor(visibleChannels)
final visibleChannelsProvider = VisibleChannelsProvider._();

/// The visible channels that are no direct message and not archived, from the sidebar's
/// tree (move targets, 01 6.7).

final class VisibleChannelsProvider
    extends $FunctionalProvider<List<Channel>, List<Channel>, List<Channel>>
    with $Provider<List<Channel>> {
  /// The visible channels that are no direct message and not archived, from the sidebar's
  /// tree (move targets, 01 6.7).
  VisibleChannelsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'visibleChannelsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$visibleChannelsHash();

  @$internal
  @override
  $ProviderElement<List<Channel>> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  List<Channel> create(Ref ref) {
    return visibleChannels(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<Channel> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<Channel>>(value),
    );
  }
}

String _$visibleChannelsHash() => r'd43866547a7ec776426bae716c09075b2ea37bdd';

/// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).

@ProviderFor(threadsOfChannel)
final threadsOfChannelProvider = ThreadsOfChannelFamily._();

/// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).

final class ThreadsOfChannelProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Thread>>,
          List<Thread>,
          Stream<List<Thread>>
        >
    with $FutureModifier<List<Thread>>, $StreamProvider<List<Thread>> {
  /// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).
  ThreadsOfChannelProvider._({
    required ThreadsOfChannelFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadsOfChannelProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadsOfChannelHash();

  @override
  String toString() {
    return r'threadsOfChannelProvider'
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
    return threadsOfChannel(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadsOfChannelProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadsOfChannelHash() => r'dd8579c6a070f981a8677261b7f712f47ddd6a75';

/// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).

final class ThreadsOfChannelFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Thread>>, String> {
  ThreadsOfChannelFamily._()
    : super(
        retry: null,
        name: r'threadsOfChannelProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).

  ThreadsOfChannelProvider call(String channelId) =>
      ThreadsOfChannelProvider._(argument: channelId, from: this);

  @override
  String toString() => r'threadsOfChannelProvider';
}

/// The reactions of [messageId], ordered by emoji.

@ProviderFor(messageReactions)
final messageReactionsProvider = MessageReactionsFamily._();

/// The reactions of [messageId], ordered by emoji.

final class MessageReactionsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<ReactionSummary>>,
          List<ReactionSummary>,
          Stream<List<ReactionSummary>>
        >
    with
        $FutureModifier<List<ReactionSummary>>,
        $StreamProvider<List<ReactionSummary>> {
  /// The reactions of [messageId], ordered by emoji.
  MessageReactionsProvider._({
    required MessageReactionsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'messageReactionsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$messageReactionsHash();

  @override
  String toString() {
    return r'messageReactionsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<ReactionSummary>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<ReactionSummary>> create(Ref ref) {
    final argument = this.argument as String;
    return messageReactions(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is MessageReactionsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$messageReactionsHash() => r'efee358574a6506294a41ba4f160af494abe0d14';

/// The reactions of [messageId], ordered by emoji.

final class MessageReactionsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<ReactionSummary>>, String> {
  MessageReactionsFamily._()
    : super(
        retry: null,
        name: r'messageReactionsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The reactions of [messageId], ordered by emoji.

  MessageReactionsProvider call(String messageId) =>
      MessageReactionsProvider._(argument: messageId, from: this);

  @override
  String toString() => r'messageReactionsProvider';
}

/// The stored message [messageId] (the quote of a reply), or null.

@ProviderFor(storedMessage)
final storedMessageProvider = StoredMessageFamily._();

/// The stored message [messageId] (the quote of a reply), or null.

final class StoredMessageProvider
    extends
        $FunctionalProvider<AsyncValue<Message?>, Message?, FutureOr<Message?>>
    with $FutureModifier<Message?>, $FutureProvider<Message?> {
  /// The stored message [messageId] (the quote of a reply), or null.
  StoredMessageProvider._({
    required StoredMessageFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storedMessageProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storedMessageHash();

  @override
  String toString() {
    return r'storedMessageProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<Message?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Message?> create(Ref ref) {
    final argument = this.argument as String;
    return storedMessage(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoredMessageProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storedMessageHash() => r'b32281876ea98495e8ac0a6e8753d9438cfc3655';

/// The stored message [messageId] (the quote of a reply), or null.

final class StoredMessageFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<Message?>, String> {
  StoredMessageFamily._()
    : super(
        retry: null,
        name: r'storedMessageProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The stored message [messageId] (the quote of a reply), or null.

  StoredMessageProvider call(String messageId) =>
      StoredMessageProvider._(argument: messageId, from: this);

  @override
  String toString() => r'storedMessageProvider';
}

/// The threads on screen and whether the window is idle, sent to the gateway as `focus`
/// (04 11.3 item 2; brief M4-20 Design note 2): every history column adds its thread while it
/// shows and removes it when it closes.

@ProviderFor(VisibleThreads)
final visibleThreadsProvider = VisibleThreadsProvider._();

/// The threads on screen and whether the window is idle, sent to the gateway as `focus`
/// (04 11.3 item 2; brief M4-20 Design note 2): every history column adds its thread while it
/// shows and removes it when it closes.
final class VisibleThreadsProvider
    extends $NotifierProvider<VisibleThreads, Set<ThreadId>> {
  /// The threads on screen and whether the window is idle, sent to the gateway as `focus`
  /// (04 11.3 item 2; brief M4-20 Design note 2): every history column adds its thread while it
  /// shows and removes it when it closes.
  VisibleThreadsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'visibleThreadsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$visibleThreadsHash();

  @$internal
  @override
  VisibleThreads create() => VisibleThreads();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Set<ThreadId> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Set<ThreadId>>(value),
    );
  }
}

String _$visibleThreadsHash() => r'fb8015b31022db51ba3d343c8d2b08558c66fec5';

/// The threads on screen and whether the window is idle, sent to the gateway as `focus`
/// (04 11.3 item 2; brief M4-20 Design note 2): every history column adds its thread while it
/// shows and removes it when it closes.

abstract class _$VisibleThreads extends $Notifier<Set<ThreadId>> {
  Set<ThreadId> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<Set<ThreadId>, Set<ThreadId>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<Set<ThreadId>, Set<ThreadId>>,
              Set<ThreadId>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

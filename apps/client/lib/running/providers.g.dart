// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The filter state of D10.

@ProviderFor(RunningFilter)
final runningFilterProvider = RunningFilterProvider._();

/// The filter state of D10.
final class RunningFilterProvider
    extends $NotifierProvider<RunningFilter, RunningFilterState> {
  /// The filter state of D10.
  RunningFilterProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runningFilterProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runningFilterHash();

  @$internal
  @override
  RunningFilter create() => RunningFilter();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RunningFilterState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RunningFilterState>(value),
    );
  }
}

String _$runningFilterHash() => r'9638b3df00667167e507228d706452c4eca25118';

/// The filter state of D10.

abstract class _$RunningFilter extends $Notifier<RunningFilterState> {
  RunningFilterState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<RunningFilterState, RunningFilterState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<RunningFilterState, RunningFilterState>,
              RunningFilterState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// Loads every page of `listActiveRuns` into the local `runs` table by the version rule when the
/// view opens (Design note 4); returns whether the server answered. A failure leaves the table
/// as it is: the view then shows what the device knows with a note.

@ProviderFor(runningRefresh)
final runningRefreshProvider = RunningRefreshProvider._();

/// Loads every page of `listActiveRuns` into the local `runs` table by the version rule when the
/// view opens (Design note 4); returns whether the server answered. A failure leaves the table
/// as it is: the view then shows what the device knows with a note.

final class RunningRefreshProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// Loads every page of `listActiveRuns` into the local `runs` table by the version rule when the
  /// view opens (Design note 4); returns whether the server answered. A failure leaves the table
  /// as it is: the view then shows what the device knows with a note.
  RunningRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runningRefreshProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runningRefreshHash();

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    return runningRefresh(ref);
  }
}

String _$runningRefreshHash() => r'f629db15829216e73336254d388898feed373e5d';

/// Every active run of the local table as a row, in queue order.

@ProviderFor(runningRows)
final runningRowsProvider = RunningRowsProvider._();

/// Every active run of the local table as a row, in queue order.

final class RunningRowsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RunningRow>>,
          List<RunningRow>,
          Stream<List<RunningRow>>
        >
    with $FutureModifier<List<RunningRow>>, $StreamProvider<List<RunningRow>> {
  /// Every active run of the local table as a row, in queue order.
  RunningRowsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runningRowsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runningRowsHash();

  @$internal
  @override
  $StreamProviderElement<List<RunningRow>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RunningRow>> create(Ref ref) {
    return runningRows(ref);
  }
}

String _$runningRowsHash() => r'f17ef5e303e0fcafc0b718d39d748a40375d7177';

/// The rows after the filters of [RunningFilter].

@ProviderFor(filteredRunningRows)
final filteredRunningRowsProvider = FilteredRunningRowsProvider._();

/// The rows after the filters of [RunningFilter].

final class FilteredRunningRowsProvider
    extends
        $FunctionalProvider<
          List<RunningRow>,
          List<RunningRow>,
          List<RunningRow>
        >
    with $Provider<List<RunningRow>> {
  /// The rows after the filters of [RunningFilter].
  FilteredRunningRowsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'filteredRunningRowsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$filteredRunningRowsHash();

  @$internal
  @override
  $ProviderElement<List<RunningRow>> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  List<RunningRow> create(Ref ref) {
    return filteredRunningRows(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(List<RunningRow> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<List<RunningRow>>(value),
    );
  }
}

String _$filteredRunningRowsHash() =>
    r'd90d271b51588d92108e626e27718f7045cee9fd';

/// The number of active runs in the local table: the counter of the sidebar entry.

@ProviderFor(activeRunCount)
final activeRunCountProvider = ActiveRunCountProvider._();

/// The number of active runs in the local table: the counter of the sidebar entry.

final class ActiveRunCountProvider extends $FunctionalProvider<int, int, int>
    with $Provider<int> {
  /// The number of active runs in the local table: the counter of the sidebar entry.
  ActiveRunCountProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'activeRunCountProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$activeRunCountHash();

  @$internal
  @override
  $ProviderElement<int> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  int create(Ref ref) {
    return activeRunCount(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$activeRunCountHash() => r'd80dc46f4e2b195ac521830ff79b8249493f2018';

/// The time the durations count to: the app clock now and then once a second, so a row's
/// duration counts up locally without a patch (01 6.2).

@ProviderFor(runningClock)
final runningClockProvider = RunningClockProvider._();

/// The time the durations count to: the app clock now and then once a second, so a row's
/// duration counts up locally without a patch (01 6.2).

final class RunningClockProvider
    extends
        $FunctionalProvider<AsyncValue<DateTime>, DateTime, Stream<DateTime>>
    with $FutureModifier<DateTime>, $StreamProvider<DateTime> {
  /// The time the durations count to: the app clock now and then once a second, so a row's
  /// duration counts up locally without a patch (01 6.2).
  RunningClockProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runningClockProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runningClockHash();

  @$internal
  @override
  $StreamProviderElement<DateTime> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<DateTime> create(Ref ref) {
    return runningClock(ref);
  }
}

String _$runningClockHash() => r'65df416ed1868f8a9e527116822cb8f6bc544269';

/// The used share of the context window of [runId] in percent, while the run store holds its
/// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
/// itself: one subscription per active run would make the server send every patch.

@ProviderFor(runContext)
final runContextProvider = RunContextFamily._();

/// The used share of the context window of [runId] in percent, while the run store holds its
/// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
/// itself: one subscription per active run would make the server send every patch.

final class RunContextProvider
    extends $FunctionalProvider<AsyncValue<double?>, double?, Stream<double?>>
    with $FutureModifier<double?>, $StreamProvider<double?> {
  /// The used share of the context window of [runId] in percent, while the run store holds its
  /// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
  /// itself: one subscription per active run would make the server send every patch.
  RunContextProvider._({
    required RunContextFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'runContextProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$runContextHash();

  @override
  String toString() {
    return r'runContextProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<double?> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<double?> create(Ref ref) {
    final argument = this.argument as String;
    return runContext(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is RunContextProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$runContextHash() => r'87c6c4f5f3e1cd4c7366979dbc702385172def48';

/// The used share of the context window of [runId] in percent, while the run store holds its
/// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
/// itself: one subscription per active run would make the server send every patch.

final class RunContextFamily extends $Family
    with $FunctionalFamilyOverride<Stream<double?>, String> {
  RunContextFamily._()
    : super(
        retry: null,
        name: r'runContextProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The used share of the context window of [runId] in percent, while the run store holds its
  /// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
  /// itself: one subscription per active run would make the server send every patch.

  RunContextProvider call(String runId) =>
      RunContextProvider._(argument: runId, from: this);

  @override
  String toString() => r'runContextProvider';
}

/// The form of "Running now" the person chose; cards by default (D-RV-7). It lasts while the
/// app runs.

@ProviderFor(RunningFormChoice)
final runningFormChoiceProvider = RunningFormChoiceProvider._();

/// The form of "Running now" the person chose; cards by default (D-RV-7). It lasts while the
/// app runs.
final class RunningFormChoiceProvider
    extends $NotifierProvider<RunningFormChoice, RunningForm> {
  /// The form of "Running now" the person chose; cards by default (D-RV-7). It lasts while the
  /// app runs.
  RunningFormChoiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runningFormChoiceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runningFormChoiceHash();

  @$internal
  @override
  RunningFormChoice create() => RunningFormChoice();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RunningForm value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RunningForm>(value),
    );
  }
}

String _$runningFormChoiceHash() => r'0cd2abb2aae32248463c496ae9b4c47724f77bf4';

/// The form of "Running now" the person chose; cards by default (D-RV-7). It lasts while the
/// app runs.

abstract class _$RunningFormChoice extends $Notifier<RunningForm> {
  RunningForm build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<RunningForm, RunningForm>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<RunningForm, RunningForm>,
              RunningForm,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// `stall_warned_at` of the active runs as of the last `listActiveRuns` load (09 5.6): the
/// stall notice of a card without live details. The marking changes no state, so no
/// `run.state_changed` carries it and the local table keeps none.

@ProviderFor(ActiveRunStalls)
final activeRunStallsProvider = ActiveRunStallsProvider._();

/// `stall_warned_at` of the active runs as of the last `listActiveRuns` load (09 5.6): the
/// stall notice of a card without live details. The marking changes no state, so no
/// `run.state_changed` carries it and the local table keeps none.
final class ActiveRunStallsProvider
    extends $NotifierProvider<ActiveRunStalls, Map<RunId, DateTime>> {
  /// `stall_warned_at` of the active runs as of the last `listActiveRuns` load (09 5.6): the
  /// stall notice of a card without live details. The marking changes no state, so no
  /// `run.state_changed` carries it and the local table keeps none.
  ActiveRunStallsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'activeRunStallsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$activeRunStallsHash();

  @$internal
  @override
  ActiveRunStalls create() => ActiveRunStalls();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Map<RunId, DateTime> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Map<RunId, DateTime>>(value),
    );
  }
}

String _$activeRunStallsHash() => r'bc2a378a9171a3a009377e49bf6751eb66b2b68f';

/// `stall_warned_at` of the active runs as of the last `listActiveRuns` load (09 5.6): the
/// stall notice of a card without live details. The marking changes no state, so no
/// `run.state_changed` carries it and the local table keeps none.

abstract class _$ActiveRunStalls extends $Notifier<Map<RunId, DateTime>> {
  Map<RunId, DateTime> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<Map<RunId, DateTime>, Map<RunId, DateTime>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<Map<RunId, DateTime>, Map<RunId, DateTime>>,
              Map<RunId, DateTime>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The cards of the control room on screen and which of them are live (brief M4-39 Design
/// note 4; 04 6.2 `background_thread_ids`, R-SY-27).
///
/// [show] hands the cards in on-screen order to `EngineFocus.setBackgroundRuns`: the threads
/// the app already shows stay in `thread_ids`, the rest of `sync.max_focused_threads` goes to
/// the cards in that order, and `idle` keeps following the window. The state is the set of
/// runs whose card is live; the others show "live details paused". Leaving the view frees the
/// places.

@ProviderFor(ControlRoomFocus)
final controlRoomFocusProvider = ControlRoomFocusProvider._();

/// The cards of the control room on screen and which of them are live (brief M4-39 Design
/// note 4; 04 6.2 `background_thread_ids`, R-SY-27).
///
/// [show] hands the cards in on-screen order to `EngineFocus.setBackgroundRuns`: the threads
/// the app already shows stay in `thread_ids`, the rest of `sync.max_focused_threads` goes to
/// the cards in that order, and `idle` keeps following the window. The state is the set of
/// runs whose card is live; the others show "live details paused". Leaving the view frees the
/// places.
final class ControlRoomFocusProvider
    extends $NotifierProvider<ControlRoomFocus, Set<RunId>> {
  /// The cards of the control room on screen and which of them are live (brief M4-39 Design
  /// note 4; 04 6.2 `background_thread_ids`, R-SY-27).
  ///
  /// [show] hands the cards in on-screen order to `EngineFocus.setBackgroundRuns`: the threads
  /// the app already shows stay in `thread_ids`, the rest of `sync.max_focused_threads` goes to
  /// the cards in that order, and `idle` keeps following the window. The state is the set of
  /// runs whose card is live; the others show "live details paused". Leaving the view frees the
  /// places.
  ControlRoomFocusProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'controlRoomFocusProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$controlRoomFocusHash();

  @$internal
  @override
  ControlRoomFocus create() => ControlRoomFocus();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Set<RunId> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Set<RunId>>(value),
    );
  }
}

String _$controlRoomFocusHash() => r'64c1fa44a92c0dfc25d72d95a8e841d240dde9c8';

/// The cards of the control room on screen and which of them are live (brief M4-39 Design
/// note 4; 04 6.2 `background_thread_ids`, R-SY-27).
///
/// [show] hands the cards in on-screen order to `EngineFocus.setBackgroundRuns`: the threads
/// the app already shows stay in `thread_ids`, the rest of `sync.max_focused_threads` goes to
/// the cards in that order, and `idle` keeps following the window. The state is the set of
/// runs whose card is live; the others show "live details paused". Leaving the view frees the
/// places.

abstract class _$ControlRoomFocus extends $Notifier<Set<RunId>> {
  Set<RunId> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<Set<RunId>, Set<RunId>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<Set<RunId>, Set<RunId>>,
              Set<RunId>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The run document of a live card from the run store (the control room focused it through
/// [ControlRoomFocus]); null while it loads or when the card is not live.

@ProviderFor(cardDocument)
final cardDocumentProvider = CardDocumentFamily._();

/// The run document of a live card from the run store (the control room focused it through
/// [ControlRoomFocus]); null while it loads or when the card is not live.

final class CardDocumentProvider
    extends
        $FunctionalProvider<
          AsyncValue<RunDocument?>,
          RunDocument?,
          Stream<RunDocument?>
        >
    with $FutureModifier<RunDocument?>, $StreamProvider<RunDocument?> {
  /// The run document of a live card from the run store (the control room focused it through
  /// [ControlRoomFocus]); null while it loads or when the card is not live.
  CardDocumentProvider._({
    required CardDocumentFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'cardDocumentProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$cardDocumentHash();

  @override
  String toString() {
    return r'cardDocumentProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<RunDocument?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<RunDocument?> create(Ref ref) {
    final argument = this.argument as String;
    return cardDocument(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is CardDocumentProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$cardDocumentHash() => r'23cdb33da3dda434c23233a364548f7ca03277a2';

/// The run document of a live card from the run store (the control room focused it through
/// [ControlRoomFocus]); null while it loads or when the card is not live.

final class CardDocumentFamily extends $Family
    with $FunctionalFamilyOverride<Stream<RunDocument?>, String> {
  CardDocumentFamily._()
    : super(
        retry: null,
        name: r'cardDocumentProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The run document of a live card from the run store (the control room focused it through
  /// [ControlRoomFocus]); null while it loads or when the card is not live.

  CardDocumentProvider call(String runId) =>
      CardDocumentProvider._(argument: runId, from: this);

  @override
  String toString() => r'cardDocumentProvider';
}

/// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
/// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).

@ProviderFor(runGraphDocument)
final runGraphDocumentProvider = RunGraphDocumentFamily._();

/// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
/// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).

final class RunGraphDocumentProvider
    extends
        $FunctionalProvider<
          AsyncValue<RunDocument?>,
          RunDocument?,
          Stream<RunDocument?>
        >
    with $FutureModifier<RunDocument?>, $StreamProvider<RunDocument?> {
  /// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
  /// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).
  RunGraphDocumentProvider._({
    required RunGraphDocumentFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'runGraphDocumentProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$runGraphDocumentHash();

  @override
  String toString() {
    return r'runGraphDocumentProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $StreamProviderElement<RunDocument?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<RunDocument?> create(Ref ref) {
    final argument = this.argument as (String, String);
    return runGraphDocument(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is RunGraphDocumentProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$runGraphDocumentHash() => r'b30f3953fab49230b54620b88aeda02133b7678e';

/// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
/// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).

final class RunGraphDocumentFamily extends $Family
    with $FunctionalFamilyOverride<Stream<RunDocument?>, (String, String)> {
  RunGraphDocumentFamily._()
    : super(
        retry: null,
        name: r'runGraphDocumentProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
  /// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).

  RunGraphDocumentProvider call(String runId, String threadId) =>
      RunGraphDocumentProvider._(argument: (runId, threadId), from: this);

  @override
  String toString() => r'runGraphDocumentProvider';
}

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

String _$runningRefreshHash() => r'b19f706539fbd877d9aeade0b72ed68f8d44ca0f';

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

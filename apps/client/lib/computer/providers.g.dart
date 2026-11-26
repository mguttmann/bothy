// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The computer ports of the signed-in instance, or null before the login; tests override it.

@ProviderFor(computerPorts)
final computerPortsProvider = ComputerPortsProvider._();

/// The computer ports of the signed-in instance, or null before the login; tests override it.

final class ComputerPortsProvider
    extends $FunctionalProvider<ComputerPorts?, ComputerPorts?, ComputerPorts?>
    with $Provider<ComputerPorts?> {
  /// The computer ports of the signed-in instance, or null before the login; tests override it.
  ComputerPortsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'computerPortsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$computerPortsHash();

  @$internal
  @override
  $ProviderElement<ComputerPorts?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ComputerPorts? create(Ref ref) {
    return computerPorts(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ComputerPorts? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ComputerPorts?>(value),
    );
  }
}

String _$computerPortsHash() => r'37fa4e9add17422448a1fc2c9f7a43957a494486';

/// The owner's computer sessions (27 11): the live ones and the ended ones this device saw
/// end, each ended one until [dismiss].
///
/// Example:
/// ```dart
/// final status = ref.watch(computerSessionsProvider).ofRun(runId);
/// await ref.read(computerSessionsProvider.notifier).stop(status!.sessionId);
/// ```

@ProviderFor(ComputerSessions)
final computerSessionsProvider = ComputerSessionsProvider._();

/// The owner's computer sessions (27 11): the live ones and the ended ones this device saw
/// end, each ended one until [dismiss].
///
/// Example:
/// ```dart
/// final status = ref.watch(computerSessionsProvider).ofRun(runId);
/// await ref.read(computerSessionsProvider.notifier).stop(status!.sessionId);
/// ```
final class ComputerSessionsProvider
    extends $NotifierProvider<ComputerSessions, ComputerSessionsState> {
  /// The owner's computer sessions (27 11): the live ones and the ended ones this device saw
  /// end, each ended one until [dismiss].
  ///
  /// Example:
  /// ```dart
  /// final status = ref.watch(computerSessionsProvider).ofRun(runId);
  /// await ref.read(computerSessionsProvider.notifier).stop(status!.sessionId);
  /// ```
  ComputerSessionsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'computerSessionsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$computerSessionsHash();

  @$internal
  @override
  ComputerSessions create() => ComputerSessions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ComputerSessionsState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ComputerSessionsState>(value),
    );
  }
}

String _$computerSessionsHash() => r'e2083f4aa559832ae98f35c7090d36419622fd26';

/// The owner's computer sessions (27 11): the live ones and the ended ones this device saw
/// end, each ended one until [dismiss].
///
/// Example:
/// ```dart
/// final status = ref.watch(computerSessionsProvider).ofRun(runId);
/// await ref.read(computerSessionsProvider.notifier).stop(status!.sessionId);
/// ```

abstract class _$ComputerSessions extends $Notifier<ComputerSessionsState> {
  ComputerSessionsState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<ComputerSessionsState, ComputerSessionsState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ComputerSessionsState, ComputerSessionsState>,
              ComputerSessionsState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

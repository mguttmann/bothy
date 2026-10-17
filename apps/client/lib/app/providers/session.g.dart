// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The services of the signed-in instance, or null before the login and after a wipe.

@ProviderFor(signedInServices)
final signedInServicesProvider = SignedInServicesProvider._();

/// The services of the signed-in instance, or null before the login and after a wipe.

final class SignedInServicesProvider
    extends
        $FunctionalProvider<
          InstanceServices?,
          InstanceServices?,
          InstanceServices?
        >
    with $Provider<InstanceServices?> {
  /// The services of the signed-in instance, or null before the login and after a wipe.
  SignedInServicesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'signedInServicesProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$signedInServicesHash();

  @$internal
  @override
  $ProviderElement<InstanceServices?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  InstanceServices? create(Ref ref) {
    return signedInServices(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(InstanceServices? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<InstanceServices?>(value),
    );
  }
}

String _$signedInServicesHash() => r'a0965b17ed284d2b3297a64270bbab4ed7b65afb';

/// The instance whose branding the app shows (01 5.6: before the login only name, logo and
/// accent), or null before E1 succeeded.

@ProviderFor(currentInstance)
final currentInstanceProvider = CurrentInstanceProvider._();

/// The instance whose branding the app shows (01 5.6: before the login only name, logo and
/// accent), or null before E1 succeeded.

final class CurrentInstanceProvider
    extends $FunctionalProvider<InstanceInfo?, InstanceInfo?, InstanceInfo?>
    with $Provider<InstanceInfo?> {
  /// The instance whose branding the app shows (01 5.6: before the login only name, logo and
  /// accent), or null before E1 succeeded.
  CurrentInstanceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentInstanceProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentInstanceHash();

  @$internal
  @override
  $ProviderElement<InstanceInfo?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  InstanceInfo? create(Ref ref) {
    return currentInstance(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(InstanceInfo? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<InstanceInfo?>(value),
    );
  }
}

String _$currentInstanceHash() => r'3f9c9e230fee3d7a34ea0ef183307d5e350141f7';

/// The local database of the signed-in instance.

@ProviderFor(localDatabase)
final localDatabaseProvider = LocalDatabaseProvider._();

/// The local database of the signed-in instance.

final class LocalDatabaseProvider
    extends $FunctionalProvider<LocalDatabase?, LocalDatabase?, LocalDatabase?>
    with $Provider<LocalDatabase?> {
  /// The local database of the signed-in instance.
  LocalDatabaseProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'localDatabaseProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$localDatabaseHash();

  @$internal
  @override
  $ProviderElement<LocalDatabase?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  LocalDatabase? create(Ref ref) {
    return localDatabase(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(LocalDatabase? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<LocalDatabase?>(value),
    );
  }
}

String _$localDatabaseHash() => r'f68f87c0bc3d0c6c2113d3223319aa64fda84599';

/// The sync engine of the signed-in instance.

@ProviderFor(syncEngine)
final syncEngineProvider = SyncEngineProvider._();

/// The sync engine of the signed-in instance.

final class SyncEngineProvider
    extends $FunctionalProvider<SyncEngine?, SyncEngine?, SyncEngine?>
    with $Provider<SyncEngine?> {
  /// The sync engine of the signed-in instance.
  SyncEngineProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'syncEngineProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$syncEngineHash();

  @$internal
  @override
  $ProviderElement<SyncEngine?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SyncEngine? create(Ref ref) {
    return syncEngine(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SyncEngine? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SyncEngine?>(value),
    );
  }
}

String _$syncEngineHash() => r'e7072878650d81fa72e7bfd921f965cb9f00f3ab';

/// The connection state of the engine (15 7.2); offline while no engine runs.

@ProviderFor(connectionState)
final connectionStateProvider = ConnectionStateProvider._();

/// The connection state of the engine (15 7.2); offline while no engine runs.

final class ConnectionStateProvider
    extends
        $FunctionalProvider<
          AsyncValue<ConnectionState>,
          ConnectionState,
          Stream<ConnectionState>
        >
    with $FutureModifier<ConnectionState>, $StreamProvider<ConnectionState> {
  /// The connection state of the engine (15 7.2); offline while no engine runs.
  ConnectionStateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectionStateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectionStateHash();

  @$internal
  @override
  $StreamProviderElement<ConnectionState> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<ConnectionState> create(Ref ref) {
    return connectionState(ref);
  }
}

String _$connectionStateHash() => r'98e98d7ef093ecc9b17c4e0bc78d056c6f9aa288';

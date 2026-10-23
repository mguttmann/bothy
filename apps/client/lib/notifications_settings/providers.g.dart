// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The personal switches of the signed-in person.

@ProviderFor(notifySettings)
final notifySettingsProvider = NotifySettingsProvider._();

/// The personal switches of the signed-in person.

final class NotifySettingsProvider
    extends $FunctionalProvider<NotifySettings, NotifySettings, NotifySettings>
    with $Provider<NotifySettings> {
  /// The personal switches of the signed-in person.
  NotifySettingsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notifySettingsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notifySettingsHash();

  @$internal
  @override
  $ProviderElement<NotifySettings> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  NotifySettings create(Ref ref) {
    return notifySettings(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(NotifySettings value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<NotifySettings>(value),
    );
  }
}

String _$notifySettingsHash() => r'bdb985bc9bed4e6e92eacc6348413f17006826ab';

/// The writer of levels and quiet hours, or null before the login.

@ProviderFor(notifyWriter)
final notifyWriterProvider = NotifyWriterProvider._();

/// The writer of levels and quiet hours, or null before the login.

final class NotifyWriterProvider
    extends $FunctionalProvider<NotifyWriter?, NotifyWriter?, NotifyWriter?>
    with $Provider<NotifyWriter?> {
  /// The writer of levels and quiet hours, or null before the login.
  NotifyWriterProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notifyWriterProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notifyWriterHash();

  @$internal
  @override
  $ProviderElement<NotifyWriter?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  NotifyWriter? create(Ref ref) {
    return notifyWriter(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(NotifyWriter? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<NotifyWriter?>(value),
    );
  }
}

String _$notifyWriterHash() => r'c4ecbf6c278d217f7ae55bae4ecf8a22e1df8484';

/// Loads the levels and quiet hours of the signed-in person once per session (the snapshot
/// carries neither, 04 6.3); a failed load leaves the stored rows and the events keep them
/// current.

@ProviderFor(notifyRefresh)
final notifyRefreshProvider = NotifyRefreshProvider._();

/// Loads the levels and quiet hours of the signed-in person once per session (the snapshot
/// carries neither, 04 6.3); a failed load leaves the stored rows and the events keep them
/// current.

final class NotifyRefreshProvider
    extends $FunctionalProvider<AsyncValue<void>, void, FutureOr<void>>
    with $FutureModifier<void>, $FutureProvider<void> {
  /// Loads the levels and quiet hours of the signed-in person once per session (the snapshot
  /// carries neither, 04 6.3); a failed load leaves the stored rows and the events keep them
  /// current.
  NotifyRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'notifyRefreshProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$notifyRefreshHash();

  @$internal
  @override
  $FutureProviderElement<void> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<void> create(Ref ref) {
    return notifyRefresh(ref);
  }
}

String _$notifyRefreshHash() => r'dc49236d6a781d440ade2a07953a2ffa98efecf0';

/// The quiet hours rules of the person, by ID.

@ProviderFor(quietHoursRules)
final quietHoursRulesProvider = QuietHoursRulesProvider._();

/// The quiet hours rules of the person, by ID.

final class QuietHoursRulesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<QuietHours>>,
          List<QuietHours>,
          Stream<List<QuietHours>>
        >
    with $FutureModifier<List<QuietHours>>, $StreamProvider<List<QuietHours>> {
  /// The quiet hours rules of the person, by ID.
  QuietHoursRulesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'quietHoursRulesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$quietHoursRulesHash();

  @$internal
  @override
  $StreamProviderElement<List<QuietHours>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<QuietHours>> create(Ref ref) {
    return quietHoursRules(ref);
  }
}

String _$quietHoursRulesHash() => r'ef36bb359d63f2982c935efa015cf8e98bb9aa21';

/// Whether the signed-in person is an owner (`getMe`, `is_owner`): only owners see
/// `notify.admin_enabled` (13 3.3). False while unknown or offline.

@ProviderFor(isOwner)
final isOwnerProvider = IsOwnerProvider._();

/// Whether the signed-in person is an owner (`getMe`, `is_owner`): only owners see
/// `notify.admin_enabled` (13 3.3). False while unknown or offline.

final class IsOwnerProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// Whether the signed-in person is an owner (`getMe`, `is_owner`): only owners see
  /// `notify.admin_enabled` (13 3.3). False while unknown or offline.
  IsOwnerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'isOwnerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$isOwnerHash();

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    return isOwner(ref);
  }
}

String _$isOwnerHash() => r'4bfc4959b184468599e1bdfc5db7f1441f897fe5';

/// The [LevelContext] of the signed-in person; reading it starts the one load of
/// [notifyRefreshProvider].

@ProviderFor(levelContext)
final levelContextProvider = LevelContextProvider._();

/// The [LevelContext] of the signed-in person; reading it starts the one load of
/// [notifyRefreshProvider].

final class LevelContextProvider
    extends $FunctionalProvider<LevelContext, LevelContext, LevelContext>
    with $Provider<LevelContext> {
  /// The [LevelContext] of the signed-in person; reading it starts the one load of
  /// [notifyRefreshProvider].
  LevelContextProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'levelContextProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$levelContextHash();

  @$internal
  @override
  $ProviderElement<LevelContext> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  LevelContext create(Ref ref) {
    return levelContext(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(LevelContext value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<LevelContext>(value),
    );
  }
}

String _$levelContextHash() => r'a1e514e316ebdd36a428227fce828b72eac83ba5';

/// The tree of D9: the spaces and channels of the sidebar in its order, below each channel
/// the threads with an own level (set or followed, 13 3.2).

@ProviderFor(levelTree)
final levelTreeProvider = LevelTreeProvider._();

/// The tree of D9: the spaces and channels of the sidebar in its order, below each channel
/// the threads with an own level (set or followed, 13 3.2).

final class LevelTreeProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<LevelTreeSpace>>,
          List<LevelTreeSpace>,
          FutureOr<List<LevelTreeSpace>>
        >
    with
        $FutureModifier<List<LevelTreeSpace>>,
        $FutureProvider<List<LevelTreeSpace>> {
  /// The tree of D9: the spaces and channels of the sidebar in its order, below each channel
  /// the threads with an own level (set or followed, 13 3.2).
  LevelTreeProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'levelTreeProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$levelTreeHash();

  @$internal
  @override
  $FutureProviderElement<List<LevelTreeSpace>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<LevelTreeSpace>> create(Ref ref) {
    return levelTree(ref);
  }
}

String _$levelTreeHash() => r'8c9c05b061d91c2f44b04bfbddb8a29e8ca2fc1c';

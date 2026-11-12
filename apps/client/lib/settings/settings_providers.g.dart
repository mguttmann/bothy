// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'settings_providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The personal settings of the signed-in person on this device: `composer.enter_sends` is
/// `false` by default on the phone (01 11).

@ProviderFor(personalSettings)
final personalSettingsProvider = PersonalSettingsProvider._();

/// The personal settings of the signed-in person on this device: `composer.enter_sends` is
/// `false` by default on the phone (01 11).

final class PersonalSettingsProvider
    extends
        $FunctionalProvider<
          PersonalSettings,
          PersonalSettings,
          PersonalSettings
        >
    with $Provider<PersonalSettings> {
  /// The personal settings of the signed-in person on this device: `composer.enter_sends` is
  /// `false` by default on the phone (01 11).
  PersonalSettingsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'personalSettingsProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$personalSettingsHash();

  @$internal
  @override
  $ProviderElement<PersonalSettings> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  PersonalSettings create(Ref ref) {
    return personalSettings(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PersonalSettings value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PersonalSettings>(value),
    );
  }
}

String _$personalSettingsHash() => r'369e9c461972a04378943563bfa3b51e43d26c2d';

/// Which levels of the sidebar are arranged by activity, from the personal settings (01 6.16,
/// D-M4-51-5); synchronized like every personal setting.

@ProviderFor(sidebarArrangement)
final sidebarArrangementProvider = SidebarArrangementProvider._();

/// Which levels of the sidebar are arranged by activity, from the personal settings (01 6.16,
/// D-M4-51-5); synchronized like every personal setting.

final class SidebarArrangementProvider
    extends
        $FunctionalProvider<
          SidebarArrangement,
          SidebarArrangement,
          SidebarArrangement
        >
    with $Provider<SidebarArrangement> {
  /// Which levels of the sidebar are arranged by activity, from the personal settings (01 6.16,
  /// D-M4-51-5); synchronized like every personal setting.
  SidebarArrangementProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sidebarArrangementProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sidebarArrangementHash();

  @$internal
  @override
  $ProviderElement<SidebarArrangement> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SidebarArrangement create(Ref ref) {
    return sidebarArrangement(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SidebarArrangement value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SidebarArrangement>(value),
    );
  }
}

String _$sidebarArrangementHash() =>
    r'fde1545679e66caa4dfeca682368be0b4a9a70bf';

/// Whether forum and project rows show the most urgent unseen status of their chats (01 6.17
/// item 6, `sidebar.show_project_status`); synchronized like every personal setting.

@ProviderFor(showProjectStatus)
final showProjectStatusProvider = ShowProjectStatusProvider._();

/// Whether forum and project rows show the most urgent unseen status of their chats (01 6.17
/// item 6, `sidebar.show_project_status`); synchronized like every personal setting.

final class ShowProjectStatusProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// Whether forum and project rows show the most urgent unseen status of their chats (01 6.17
  /// item 6, `sidebar.show_project_status`); synchronized like every personal setting.
  ShowProjectStatusProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'showProjectStatusProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$showProjectStatusHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return showProjectStatus(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$showProjectStatusHash() => r'cdf1d631bd926a76c904abff4295e0d3b34f6c04';

/// The writes of personal settings the instance refused for good and the person has not
/// decided about yet, oldest first; empty before the login.

@ProviderFor(refusedSettingWrites)
final refusedSettingWritesProvider = RefusedSettingWritesProvider._();

/// The writes of personal settings the instance refused for good and the person has not
/// decided about yet, oldest first; empty before the login.

final class RefusedSettingWritesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RefusedSettingWrite>>,
          List<RefusedSettingWrite>,
          Stream<List<RefusedSettingWrite>>
        >
    with
        $FutureModifier<List<RefusedSettingWrite>>,
        $StreamProvider<List<RefusedSettingWrite>> {
  /// The writes of personal settings the instance refused for good and the person has not
  /// decided about yet, oldest first; empty before the login.
  RefusedSettingWritesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'refusedSettingWritesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$refusedSettingWritesHash();

  @$internal
  @override
  $StreamProviderElement<List<RefusedSettingWrite>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RefusedSettingWrite>> create(Ref ref) {
    return refusedSettingWrites(ref);
  }
}

String _$refusedSettingWritesHash() =>
    r'e072b0c5450e439e3ef5215c97f47298f5a71673';

/// The actions on refused writes of the signed-in instance, or null before the login.

@ProviderFor(refusedSettingActions)
final refusedSettingActionsProvider = RefusedSettingActionsProvider._();

/// The actions on refused writes of the signed-in instance, or null before the login.

final class RefusedSettingActionsProvider
    extends
        $FunctionalProvider<
          RefusedSettingActions?,
          RefusedSettingActions?,
          RefusedSettingActions?
        >
    with $Provider<RefusedSettingActions?> {
  /// The actions on refused writes of the signed-in instance, or null before the login.
  RefusedSettingActionsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'refusedSettingActionsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$refusedSettingActionsHash();

  @$internal
  @override
  $ProviderElement<RefusedSettingActions?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  RefusedSettingActions? create(Ref ref) {
    return refusedSettingActions(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RefusedSettingActions? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RefusedSettingActions?>(value),
    );
  }
}

String _$refusedSettingActionsHash() =>
    r'19f4f73d46f6063e269b988208a17d63af237a32';

/// The settings writer of the signed-in instance, or null before the login.

@ProviderFor(settingsRepository)
final settingsRepositoryProvider = SettingsRepositoryProvider._();

/// The settings writer of the signed-in instance, or null before the login.

final class SettingsRepositoryProvider
    extends
        $FunctionalProvider<
          SettingsRepository?,
          SettingsRepository?,
          SettingsRepository?
        >
    with $Provider<SettingsRepository?> {
  /// The settings writer of the signed-in instance, or null before the login.
  SettingsRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'settingsRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$settingsRepositoryHash();

  @$internal
  @override
  $ProviderElement<SettingsRepository?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SettingsRepository? create(Ref ref) {
    return settingsRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SettingsRepository? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SettingsRepository?>(value),
    );
  }
}

String _$settingsRepositoryHash() =>
    r'82bbd8c6efda5f2bfb67b9a59a4bc02af84321b7';

/// The profile of the signed-in person: read with `getMe`, changed with `updateMyProfile`
/// (01 9: the app language follows `users.locale`).
///
/// The device remembers what the instance last said, with the secrets of the instance (the
/// local database has no row for the own profile), so a start without an answer of the
/// instance shows the language and the time zone of the last start at once (01 3.6). The
/// instance is asked at every start, with the read retry of the session, and once more each
/// time the connection is live again without an answer since it was lost; an answer replaces
/// what is shown and what is remembered, a failed read leaves both
/// (docs/decisions.d/AUD-client-settings-01.md). The remembered profile is deleted with the
/// secrets of the account (R-SY-14), and an answer that arrives after the account ended is not
/// remembered: the secrets of an ended account take no write (`InstanceSecrets.deleteAll`).

@ProviderFor(ProfileController)
final profileControllerProvider = ProfileControllerProvider._();

/// The profile of the signed-in person: read with `getMe`, changed with `updateMyProfile`
/// (01 9: the app language follows `users.locale`).
///
/// The device remembers what the instance last said, with the secrets of the instance (the
/// local database has no row for the own profile), so a start without an answer of the
/// instance shows the language and the time zone of the last start at once (01 3.6). The
/// instance is asked at every start, with the read retry of the session, and once more each
/// time the connection is live again without an answer since it was lost; an answer replaces
/// what is shown and what is remembered, a failed read leaves both
/// (docs/decisions.d/AUD-client-settings-01.md). The remembered profile is deleted with the
/// secrets of the account (R-SY-14), and an answer that arrives after the account ended is not
/// remembered: the secrets of an ended account take no write (`InstanceSecrets.deleteAll`).
final class ProfileControllerProvider
    extends $AsyncNotifierProvider<ProfileController, Profile> {
  /// The profile of the signed-in person: read with `getMe`, changed with `updateMyProfile`
  /// (01 9: the app language follows `users.locale`).
  ///
  /// The device remembers what the instance last said, with the secrets of the instance (the
  /// local database has no row for the own profile), so a start without an answer of the
  /// instance shows the language and the time zone of the last start at once (01 3.6). The
  /// instance is asked at every start, with the read retry of the session, and once more each
  /// time the connection is live again without an answer since it was lost; an answer replaces
  /// what is shown and what is remembered, a failed read leaves both
  /// (docs/decisions.d/AUD-client-settings-01.md). The remembered profile is deleted with the
  /// secrets of the account (R-SY-14), and an answer that arrives after the account ended is not
  /// remembered: the secrets of an ended account take no write (`InstanceSecrets.deleteAll`).
  ProfileControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'profileControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$profileControllerHash();

  @$internal
  @override
  ProfileController create() => ProfileController();
}

String _$profileControllerHash() => r'e9e3b4bca6e58d5f2dfc40efc898ab2c9bcc7824';

/// The profile of the signed-in person: read with `getMe`, changed with `updateMyProfile`
/// (01 9: the app language follows `users.locale`).
///
/// The device remembers what the instance last said, with the secrets of the instance (the
/// local database has no row for the own profile), so a start without an answer of the
/// instance shows the language and the time zone of the last start at once (01 3.6). The
/// instance is asked at every start, with the read retry of the session, and once more each
/// time the connection is live again without an answer since it was lost; an answer replaces
/// what is shown and what is remembered, a failed read leaves both
/// (docs/decisions.d/AUD-client-settings-01.md). The remembered profile is deleted with the
/// secrets of the account (R-SY-14), and an answer that arrives after the account ended is not
/// remembered: the secrets of an ended account take no write (`InstanceSecrets.deleteAll`).

abstract class _$ProfileController extends $AsyncNotifier<Profile> {
  FutureOr<Profile> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<AsyncValue<Profile>, Profile>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<Profile>, Profile>,
              AsyncValue<Profile>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// The time zone the screens format with: the profile's, else UTC.

@ProviderFor(personTimeZone)
final personTimeZoneProvider = PersonTimeZoneProvider._();

/// The time zone the screens format with: the profile's, else UTC.

final class PersonTimeZoneProvider
    extends $FunctionalProvider<String, String, String>
    with $Provider<String> {
  /// The time zone the screens format with: the profile's, else UTC.
  PersonTimeZoneProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'personTimeZoneProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$personTimeZoneHash();

  @$internal
  @override
  $ProviderElement<String> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String create(Ref ref) {
    return personTimeZone(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$personTimeZoneHash() => r'cc37a770618b96b51e760bc2cb1144e4b3e17da9';

/// The REST source of the device and session screens, or null before the login.

@ProviderFor(selfRepository)
final selfRepositoryProvider = SelfRepositoryProvider._();

/// The REST source of the device and session screens, or null before the login.

final class SelfRepositoryProvider
    extends
        $FunctionalProvider<SelfRepository?, SelfRepository?, SelfRepository?>
    with $Provider<SelfRepository?> {
  /// The REST source of the device and session screens, or null before the login.
  SelfRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'selfRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$selfRepositoryHash();

  @$internal
  @override
  $ProviderElement<SelfRepository?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SelfRepository? create(Ref ref) {
    return selfRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SelfRepository? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SelfRepository?>(value),
    );
  }
}

String _$selfRepositoryHash() => r'950d983e4442566dbe95b31f404054cdc65fdb98';

/// The instance description of `getInstance` (About: `spec_version`).

@ProviderFor(instanceDescription)
final instanceDescriptionProvider = InstanceDescriptionProvider._();

/// The instance description of `getInstance` (About: `spec_version`).

final class InstanceDescriptionProvider
    extends
        $FunctionalProvider<
          AsyncValue<api.InstanceInfo?>,
          api.InstanceInfo?,
          FutureOr<api.InstanceInfo?>
        >
    with
        $FutureModifier<api.InstanceInfo?>,
        $FutureProvider<api.InstanceInfo?> {
  /// The instance description of `getInstance` (About: `spec_version`).
  InstanceDescriptionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'instanceDescriptionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$instanceDescriptionHash();

  @$internal
  @override
  $FutureProviderElement<api.InstanceInfo?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<api.InstanceInfo?> create(Ref ref) {
    return instanceDescription(ref);
  }
}

String _$instanceDescriptionHash() =>
    r'15c421d0542d743f97ceef2a536cfe295dae0fde';

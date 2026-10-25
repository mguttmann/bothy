// SPDX-License-Identifier: Apache-2.0

/// The settings model of the desktop app: the personal settings of 01 11 with their defaults,
/// their writes through the outbox, the profile (language and time zone) and the REST source
/// of the device screens (docs/spec/01-product.md 9 and 11; 04 14; brief M4-25 Design notes 1
/// and 2).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' as api show InstanceInfo;
import 'package:api_client/api_client.dart' show ProfileUpdate;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:timezone/data/latest_all.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/providers/settings.dart';

part 'settings_providers.g.dart';

/// The personal setting keys of 01 11 this app reads; the server rejects any other key.
abstract final class SettingKeys {
  /// `system`, `light`, `dark`.
  static const String theme = 'appearance.theme';

  /// `comfortable`, `compact`.
  static const String density = 'appearance.density';

  /// `system`, `on`, `off`.
  static const String reduceMotion = 'appearance.reduce_motion';

  /// `true`, `false`.
  static const String enterSends = 'composer.enter_sends';

  /// `true`, `false`.
  static const String crashReports = 'privacy.crash_reports';

  /// `true`, `false`.
  static const String clientMetrics = 'privacy.client_metrics';

  /// The object of column widths (D-M4-18-5).
  static const String columnWidths = 'layout.column_widths';
}

/// The personal settings with the defaults of 01 11 applied where a key is absent.
///
/// The server stores no defaults, because some depend on the device or the instance (04 14):
/// `composer.enter_sends` is `true` on desktop and `false` on mobile, `privacy.crash_reports`
/// is `true` only where a crash target exists. A stored value applies on every device.
final class PersonalSettings {
  const PersonalSettings._({
    required this.theme,
    required this.density,
    required this.reduceMotion,
    required this.enterSends,
    required this.crashReports,
    required this.clientMetrics,
    required this.hasColumnWidths,
  });

  /// Reads [stored] (key to JSON value); a value outside the list of 01 11 counts as absent.
  ///
  /// Example: `PersonalSettings.resolve({}, desktop: true, hasCrashTarget: false).enterSends`
  /// is `true`.
  factory PersonalSettings.resolve(
    Map<String, Object?> stored, {
    required bool desktop,
    required bool hasCrashTarget,
  }) {
    String choice(String key, List<String> values) {
      final value = stored[key];
      return value is String && values.contains(value)
          ? value
          : fixedDefaults[key]! as String;
    }

    bool flag(String key, bool fallback) {
      final value = stored[key];
      return value is bool ? value : fallback;
    }

    return PersonalSettings._(
      theme: choice(SettingKeys.theme, const ['system', 'light', 'dark']),
      density: choice(SettingKeys.density, const ['comfortable', 'compact']),
      reduceMotion: choice(SettingKeys.reduceMotion, const [
        'system',
        'on',
        'off',
      ]),
      enterSends: flag(SettingKeys.enterSends, desktop),
      crashReports: flag(SettingKeys.crashReports, hasCrashTarget),
      clientMetrics: flag(SettingKeys.clientMetrics, true),
      hasColumnWidths: stored[SettingKeys.columnWidths] is Map,
    );
  }

  /// The defaults that do not depend on the device or the instance (01 11); choosing one of
  /// them deletes the key, so the server never stores a default.
  static const Map<String, Object> fixedDefaults = {
    SettingKeys.theme: 'system',
    SettingKeys.density: 'comfortable',
    SettingKeys.reduceMotion: 'system',
    SettingKeys.clientMetrics: true,
  };

  /// `appearance.theme`.
  final String theme;

  /// `appearance.density`.
  final String density;

  /// `appearance.reduce_motion`.
  final String reduceMotion;

  /// `composer.enter_sends`.
  final bool enterSends;

  /// `privacy.crash_reports`.
  final bool crashReports;

  /// `privacy.client_metrics`.
  final bool clientMetrics;

  /// Whether `layout.column_widths` is stored (the reset has something to delete).
  final bool hasColumnWidths;
}

/// The personal settings of the signed-in person on this device: `composer.enter_sends` is
/// `false` by default on the phone (01 11).
@Riverpod(keepAlive: true)
PersonalSettings personalSettings(Ref ref) => PersonalSettings.resolve(
  ref.watch(userSettingsProvider).value ?? const {},
  desktop: !ref.watch(appPortsProvider).formFactor.isPhone,
  hasCrashTarget: ref.watch(appPortsProvider).branding.hasCrashTarget,
);

/// Writes personal settings (04 14, brief M4-25 Design note 1).
///
/// A change updates the local `user_settings` row at once (one version above the stored one,
/// so the screen and the theme follow without waiting) and goes to the outbox as
/// `putMySetting`; a reset deletes the local row and goes out as `deleteMySetting`. The echo of
/// the server is ignored by `updated_by_device_id`; a later write of another device has a
/// higher server version and replaces the local row.
final class SettingsRepository {
  /// Creates the repository over the local database and the engine's outbox.
  SettingsRepository(this._db, this._engine);

  final LocalDatabase _db;
  final SyncEngine _engine;

  Future<UserSetting?> _stored(String key) async {
    final all = await _db.personalDao.watchSettings().first;
    return all.where((s) => s.key == key).firstOrNull;
  }

  /// Stores [value] for [key]; a value equal to its fixed default resets instead.
  Future<void> set(String key, Object value) async {
    if (PersonalSettings.fixedDefaults[key] == value) return reset(key);
    final stored = await _stored(key);
    if (stored != null && stored.value == value) return;
    await _db.personalDao.upsertSettingIfNewer(
      UserSetting(
        key: key,
        value: value,
        version: ResourceVersion((stored?.version.value ?? 0) + 1),
      ),
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: PersonalOps.putMySetting,
        payload: {'key': key, 'value': value},
      ),
    );
  }

  /// Deletes [key], so its default applies again; nothing happens when it is not stored.
  Future<void> reset(String key) async {
    final stored = await _stored(key);
    if (stored == null) return;
    await _db.personalDao.deleteSettingIfNewer(
      key,
      ResourceVersion(stored.version.value + 1),
    );
    await _engine.outbox.enqueue(
      OutboxRequest(kind: PersonalOps.deleteMySetting, payload: {'key': key}),
    );
  }
}

/// The settings writer of the signed-in instance, or null before the login.
@Riverpod(keepAlive: true)
SettingsRepository? settingsRepository(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  final engine = ref.watch(syncEngineProvider);
  return db == null || engine == null ? null : SettingsRepository(db, engine);
}

/// Language and time zone of the profile (`users.locale`, `users.time_zone`).
final class Profile {
  /// Creates a profile.
  const Profile({this.locale, this.timeZone});

  /// `de` or `en`, or null while the person has chosen none (the app follows the system).
  final String? locale;

  /// The IANA time zone, or null.
  final String? timeZone;
}

/// The profile of the signed-in person: read with `getMe`, changed with `updateMyProfile`
/// (01 9: the app language follows `users.locale`).
@Riverpod(keepAlive: true)
class ProfileController extends _$ProfileController {
  @override
  Future<Profile> build() async {
    final services = ref.watch(signedInServicesProvider);
    if (services == null) return const Profile();
    final me = (await services.auth.api.getMeApi().getMe()).data!;
    return Profile(locale: me.locale, timeZone: me.timeZone);
  }

  /// Changes the language and/or the time zone; the app switches at once after the answer.
  ///
  /// Throws the `DioException` of a refused or unanswered request; the state stays.
  Future<void> change({String? locale, String? timeZone}) async {
    final services = ref.read(signedInServicesProvider);
    if (services == null) return;
    final user = (await services.auth.api.getMeApi().updateMyProfile(
      profileUpdate: ProfileUpdate(
        (b) => b
          ..locale = locale
          ..timeZone = timeZone,
      ),
      idempotencyKey: services.ids.next(),
    )).data!;
    state = AsyncData(Profile(locale: user.locale, timeZone: user.timeZone));
  }
}

/// The time zone the screens format with: the profile's, else UTC.
@Riverpod(keepAlive: true)
String personTimeZone(Ref ref) =>
    ref.watch(profileControllerProvider).value?.timeZone ?? 'UTC';

/// The REST source of the device and session screens, or null before the login.
@Riverpod(keepAlive: true)
SelfRepository? selfRepository(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return ApiSelfRepository(
    services.auth.api,
    newIdempotencyKey: services.ids.next,
  );
}

/// The instance description of `getInstance` (About: `spec_version`).
@Riverpod(keepAlive: true)
Future<api.InstanceInfo?> instanceDescription(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return (await services.auth.api.getAdminApi().getInstance()).data;
}

/// The IANA time zones of the time zone database, sorted (Design note 2).
List<String> ianaTimeZones() {
  tz_data.initializeTimeZones();
  return tz.timeZoneDatabase.locations.keys.toList()..sort();
}

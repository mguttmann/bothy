// SPDX-License-Identifier: Apache-2.0

/// The settings model of the desktop app: the personal settings of 01 11 with their defaults,
/// their writes through the outbox, the profile (language and time zone) and the REST source
/// of the device screens (docs/spec/01-product.md 9 and 11; 04 14; brief M4-25 Design notes 1
/// and 2).
library;

import 'dart:async';
import 'dart:convert';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' as api show InstanceInfo;
import 'package:api_client/api_client.dart'
    show ProfileUpdate, noReadRetryExtra;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:timezone/data/latest_all.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

import '../app/bootstrap.dart' show InstanceServices;
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/providers/settings.dart';
import '../sidebar/sidebar_arrange.dart';

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

  /// `true`, `false` (FIX-scan-state-stale).
  static const String pasteOriginals = 'composer.paste_originals';

  /// `true`, `false`.
  static const String crashReports = 'privacy.crash_reports';

  /// `true`, `false`.
  static const String clientMetrics = 'privacy.client_metrics';

  /// The object of column widths (D-M4-18-5).
  static const String columnWidths = 'layout.column_widths';

  /// `true`, `false`: spaces arranged by activity (01 6.16, D-M4-51-5).
  static const String autoArrangeSpaces = 'sidebar.auto_arrange_spaces';

  /// `true`, `false`: the channels of a space arranged by activity.
  static const String autoArrangeChannels = 'sidebar.auto_arrange_channels';

  /// `true`, `false`: the open posts of a forum arranged by activity.
  static const String autoArrangeChats = 'sidebar.auto_arrange_chats';

  /// `true`, `false`: forum and project rows show the most urgent unseen status of their chats
  /// (01 6.17 item 6, D-M4-52-6).
  static const String showProjectStatus = 'sidebar.show_project_status';
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
    required this.pasteOriginals,
    required this.crashReports,
    required this.clientMetrics,
    required this.hasColumnWidths,
    required this.arrangement,
    required this.showProjectStatus,
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
      pasteOriginals: flag(SettingKeys.pasteOriginals, false),
      crashReports: flag(SettingKeys.crashReports, hasCrashTarget),
      clientMetrics: flag(SettingKeys.clientMetrics, true),
      hasColumnWidths: stored[SettingKeys.columnWidths] is Map,
      arrangement: SidebarArrangement(
        autoSpaces: flag(SettingKeys.autoArrangeSpaces, false),
        autoChannels: flag(SettingKeys.autoArrangeChannels, false),
        autoChats: flag(SettingKeys.autoArrangeChats, true),
      ),
      showProjectStatus: flag(SettingKeys.showProjectStatus, true),
    );
  }

  /// The defaults that do not depend on the device or the instance (01 11); choosing one of
  /// them deletes the key, so the server never stores a default.
  static const Map<String, Object> fixedDefaults = {
    SettingKeys.theme: 'system',
    SettingKeys.density: 'comfortable',
    SettingKeys.reduceMotion: 'system',
    SettingKeys.clientMetrics: true,
    SettingKeys.pasteOriginals: false,
    SettingKeys.autoArrangeSpaces: false,
    SettingKeys.autoArrangeChannels: false,
    SettingKeys.autoArrangeChats: true,
    SettingKeys.showProjectStatus: true,
  };

  /// `appearance.theme`.
  final String theme;

  /// `appearance.density`.
  final String density;

  /// `appearance.reduce_motion`.
  final String reduceMotion;

  /// `composer.enter_sends`.
  final bool enterSends;

  /// `composer.paste_originals`: a pasted bitmap goes as it is, never as a smaller JPEG.
  final bool pasteOriginals;

  /// `privacy.crash_reports`.
  final bool crashReports;

  /// `privacy.client_metrics`.
  final bool clientMetrics;

  /// Whether `layout.column_widths` is stored (the reset has something to delete).
  final bool hasColumnWidths;

  /// `sidebar.auto_arrange_spaces`, `sidebar.auto_arrange_channels` and
  /// `sidebar.auto_arrange_chats` (01 6.16, D-M4-51-5).
  final SidebarArrangement arrangement;

  /// `sidebar.show_project_status` (01 6.17 item 6).
  final bool showProjectStatus;
}

/// The personal settings of the signed-in person on this device: `composer.enter_sends` is
/// `false` by default on the phone (01 11).
@Riverpod(keepAlive: true)
PersonalSettings personalSettings(Ref ref) => PersonalSettings.resolve(
  ref.watch(userSettingsProvider).value ?? const {},
  desktop: !ref.watch(appPortsProvider).formFactor.isPhone,
  hasCrashTarget: ref.watch(appPortsProvider).branding.hasCrashTarget,
);

/// Which levels of the sidebar are arranged by activity, from the personal settings (01 6.16,
/// D-M4-51-5); synchronized like every personal setting.
@Riverpod(keepAlive: true)
SidebarArrangement sidebarArrangement(Ref ref) =>
    ref.watch(personalSettingsProvider).arrangement;

/// Whether forum and project rows show the most urgent unseen status of their chats (01 6.17
/// item 6, `sidebar.show_project_status`); synchronized like every personal setting.
@Riverpod(keepAlive: true)
bool showProjectStatus(Ref ref) =>
    ref.watch(personalSettingsProvider).showProjectStatus;

/// A write of a personal setting the instance refused for good (04 13.2 item 5). Its outbox
/// entry has no thread, so no history shows it: the settings show it
/// (`RefusedSettingWritesNotice`) until the person sends it again or discards it
/// ([SettingsRepository.sendAgain], [SettingsRepository.discard]). Until then the local row
/// keeps what the person chose.
final class RefusedSettingWrite {
  /// Creates the refused write.
  const RefusedSettingWrite({
    required this.entryId,
    required this.key,
    required this.code,
    this.reason,
  });

  /// The ID of the failed outbox entry.
  final String entryId;

  /// The key of the setting, for example `appearance.theme`.
  final String key;

  /// The problem code of the refusal (06 5), or a client code of the outbox.
  final String code;

  /// The `reason` of the problem, if any.
  final String? reason;

  @override
  bool operator ==(Object other) =>
      other is RefusedSettingWrite &&
      other.entryId == entryId &&
      other.key == key &&
      other.code == code &&
      other.reason == reason;

  @override
  int get hashCode => Object.hash(entryId, key, code, reason);
}

const Set<String> _settingWrites = {
  PersonalOps.putMySetting,
  PersonalOps.deleteMySetting,
};

/// The refused writes of personal settings among the outbox entries without a thread
/// ([views]), oldest first.
List<RefusedSettingWrite> refusedSettingWritesOf(List<OutboxEntryView> views) =>
    [
      for (final v in views)
        if (v.stateLabel == OutboxStateLabel.failed &&
            _settingWrites.contains(v.entry.kind))
          if (v.payload['key'] case final String key)
            RefusedSettingWrite(
              entryId: v.id,
              key: key,
              code: v.failureCode ?? 'unknown',
              reason: v.failureReason,
            ),
    ];

/// The writes of personal settings the instance refused for good and the person has not
/// decided about yet, oldest first; empty before the login.
@riverpod
Stream<List<RefusedSettingWrite>> refusedSettingWrites(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  if (engine == null) return Stream.value(const []);
  return engine.outbox.watchThread(null).map(refusedSettingWritesOf);
}

/// What the person can do with a refused write of a setting (04 13.2 item 5); the notice of
/// the settings calls these.
abstract interface class RefusedSettingActions {
  /// Sends [write] again under a new ID.
  Future<void> sendAgain(RefusedSettingWrite write);

  /// Removes [write] and shows what the instance stores for its key; throws the
  /// `DioException` of the read when the instance does not answer, and nothing changed.
  Future<void> discard(RefusedSettingWrite write);
}

/// The actions on refused writes of the signed-in instance, or null before the login.
@riverpod
RefusedSettingActions? refusedSettingActions(Ref ref) =>
    ref.watch(settingsRepositoryProvider);

/// Writes personal settings (04 14, brief M4-25 Design note 1).
///
/// A change updates the local `user_settings` row at once (one version above the stored one,
/// so the screen and the theme follow without waiting) and goes to the outbox as
/// `putMySetting`; a reset deletes the local row and goes out as `deleteMySetting`. The echo of
/// the server is ignored by `updated_by_device_id`; a later write of another device has a
/// higher server version and replaces the local row.
///
/// A write the instance refuses for good stays in the outbox as failed
/// ([RefusedSettingWrite]); [sendAgain] and [discard] are what the person can do with it.
final class SettingsRepository implements RefusedSettingActions {
  /// Creates the repository over the local database and the engine's outbox; [stored] reads
  /// the settings the instance stores, `listMySettings` of the engine's client by default.
  SettingsRepository(
    this._db,
    this._engine, {
    Future<List<UserSetting>> Function()? stored,
  }) : _storedOnInstance = stored;

  final LocalDatabase _db;
  final SyncEngine _engine;
  final Future<List<UserSetting>> Function()? _storedOnInstance;

  Future<List<UserSetting>> _listMySettings() async {
    final list = (await _engine.api.getPreferencesApi().listMySettings()).data!;
    return [
      for (final s in list.items)
        UserSetting(
          key: s.key,
          value: s.value?.value,
          version: ResourceVersion(s.version),
        ),
    ];
  }

  /// Sends the refused write [write] again under a new ID (04 13.2 item 5) and puts the local
  /// row of its key to what it sends, as [set] and [reset] do: the value of the entry one
  /// version above the stored one, or no row for a reset. The row may have moved on since the
  /// refusal (a later write of the key, a discarded one), and the device ignores the echo of
  /// its own write, so it would show another value than the instance stores otherwise.
  /// Nothing happens when its entry is gone or has left the failed state meanwhile.
  @override
  Future<void> sendAgain(RefusedSettingWrite write) async {
    final entry = await _db.outboxDao.byId(write.entryId);
    if (entry == null || entry.state != OutboxState.failed) return;
    // The row first, the entry last: an app that ends in between shows the notice again, with
    // the value the person chose in the row.
    if (entry.kind == PersonalOps.deleteMySetting) {
      await _removeLocal(write.key);
    } else {
      await _storeLocal(write.key, decodePayload(entry.payload)['value']);
    }
    try {
      await _engine.outbox.retry(write.entryId);
    } on StateError {
      // Discarded or sent again from another place: nothing is left to send.
    }
  }

  /// Discards the refused write [write] and puts the local row of its key back to what the
  /// instance stores: its value and version, or no row when the instance has none, so the
  /// default applies.
  ///
  /// The instance is asked first (`listMySettings`). Without an answer the `DioException`
  /// passes and nothing changed: the entry stays in the outbox and the local row keeps what
  /// the person chose. While another write of the same key still waits in the outbox, the
  /// local row keeps the value of that write; its answer decides.
  @override
  Future<void> discard(RefusedSettingWrite write) async {
    final all = await (_storedOnInstance ?? _listMySettings)();
    final stored = all.where((s) => s.key == write.key).firstOrNull;
    // The row first, the entry last: an app that ends in between shows the notice again
    // instead of a value nobody can put back.
    if (!await _writeWaits(write.key)) {
      await _db.transaction(() async {
        final local = await _stored(write.key);
        if (local != null) {
          await _db.personalDao.deleteSettingIfNewer(
            write.key,
            ResourceVersion(local.version.value + 1),
          );
        }
        if (stored != null) await _db.personalDao.upsertSettingIfNewer(stored);
      });
    }
    await _engine.outbox.discard(write.entryId);
  }

  /// Whether a write of the setting [key] waits in the outbox for its answer.
  Future<bool> _writeWaits(String key) async {
    for (final e in await _db.outboxDao.entriesFor(null)) {
      if (!_settingWrites.contains(e.kind) || e.state.isFinal) continue;
      if (decodePayload(e.payload)['key'] == key) return true;
    }
    return false;
  }

  Future<UserSetting?> _stored(String key) async {
    final all = await _db.personalDao.watchSettings().first;
    return all.where((s) => s.key == key).firstOrNull;
  }

  /// Puts the local row of [key] to [value], one version above the stored one; returns false
  /// and changes nothing when the row holds [value] already.
  Future<bool> _storeLocal(String key, Object? value) async {
    final stored = await _stored(key);
    if (stored != null && stored.value == value) return false;
    await _db.personalDao.upsertSettingIfNewer(
      UserSetting(
        key: key,
        value: value,
        version: ResourceVersion((stored?.version.value ?? 0) + 1),
      ),
    );
    return true;
  }

  /// Removes the local row of [key]; returns false when there is none.
  Future<bool> _removeLocal(String key) async {
    final stored = await _stored(key);
    if (stored == null) return false;
    await _db.personalDao.deleteSettingIfNewer(
      key,
      ResourceVersion(stored.version.value + 1),
    );
    return true;
  }

  /// Stores [value] for [key]; a value equal to its fixed default resets instead.
  Future<void> set(String key, Object value) async {
    if (PersonalSettings.fixedDefaults[key] == value) return reset(key);
    if (!await _storeLocal(key, value)) return;
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: PersonalOps.putMySetting,
        payload: {'key': key, 'value': value},
      ),
    );
  }

  /// Deletes [key], so its default applies again; nothing happens when it is not stored.
  Future<void> reset(String key) async {
    if (!await _removeLocal(key)) return;
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

  /// Reads the profile [ProfileController] remembered as [json], or null when the text is
  /// not such a profile.
  static Profile? fromRemembered(String? json) {
    if (json == null) return null;
    try {
      final map = jsonDecode(json);
      if (map is! Map) return null;
      final locale = map['locale'];
      final timeZone = map['time_zone'];
      if ((locale != null && locale is! String) ||
          (timeZone != null && timeZone is! String)) {
        return null;
      }
      return Profile(locale: locale as String?, timeZone: timeZone as String?);
    } on FormatException {
      return null;
    }
  }

  /// The text [fromRemembered] reads.
  String toRemembered() =>
      jsonEncode({'locale': ?locale, 'time_zone': ?timeZone});
}

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
@Riverpod(keepAlive: true)
class ProfileController extends _$ProfileController {
  /// The name of the remembered profile in the secrets of the instance.
  static const String rememberedKey = 'profile';

  /// How often [change] stored a profile, and what the instance answered to the last one: a
  /// read that was under way then may bring the profile from before.
  int _changes = 0;
  Profile? _changed;

  @override
  Future<Profile> build() async {
    final services = ref.watch(signedInServicesProvider);
    if (services == null) return const Profile();
    var current = true;
    // Whether the instance answered since the connection was last lost, how many reads are
    // under way, and whether the connection came back while one was.
    var answered = false;
    var asking = 0;
    var askAgain = false;

    void show(Future<Profile> answer) => unawaited(
      answer.then((profile) {
        if (current) state = AsyncData(profile);
      }, onError: (Object _) {}),
    );

    Future<Profile> ask({bool once = false}) async {
      asking++;
      final changes = _changes;
      try {
        final me = (await services.auth.api.getMeApi().getMe(
          // One attempt on a new connection: the connection state says when the next is due.
          extra: once ? const {noReadRetryExtra: true} : null,
        )).data!;
        answered = true;
        // The person changed the profile while this read was under way: the answer to the
        // change is the newer one, whichever arrived last.
        final changed = changes == _changes ? null : _changed;
        if (changed != null) return changed;
        final profile = Profile(locale: me.locale, timeZone: me.timeZone);
        await _remember(services, profile);
        return profile;
      } finally {
        asking--;
        // The connection came back while this read was under way and it brought no answer:
        // one more attempt, then the next new connection decides.
        if (current && askAgain && !answered && asking == 0) {
          askAgain = false;
          show(ask(once: true));
        }
      }
    }

    var live = services.engine.state.phase == ConnectionPhase.live;
    final states = services.engine.states.listen((s) {
      final now = s.phase == ConnectionPhase.live;
      // Live again without an answer since the connection was lost: ask once more, whatever
      // became of the reads before. A read that is under way is given its chance first.
      if (now && !live && !answered) {
        if (asking == 0) {
          show(ask(once: true));
        } else {
          askAgain = true;
        }
      }
      if (!now && live) {
        answered = false;
        askAgain = false;
      }
      live = now;
    });
    ref.onDispose(() {
      current = false;
      unawaited(states.cancel());
    });
    // The instance is asked at once, with the read retry of the session; what the device
    // remembers shows meanwhile, and the answer replaces it when it comes. `ignore`: a
    // failure before the secrets answered is no unhandled error; whoever takes `asked` below
    // still gets it.
    final asked = ask()..ignore();
    final remembered = await _remembered(services);
    if (remembered == null) return asked;
    show(asked);
    return remembered;
  }

  static Future<Profile?> _remembered(InstanceServices services) async {
    try {
      return Profile.fromRemembered(await services.secrets.read(rememberedKey));
    } on Object {
      // A secure storage that does not answer: as if nothing were remembered.
      return null;
    }
  }

  /// Remembers [profile]. A failed write leaves the older entry or none: the profile of this
  /// run is right either way, and the next answer of the instance writes again.
  static Future<void> _remember(
    InstanceServices services,
    Profile profile,
  ) async {
    try {
      await services.secrets.write(rememberedKey, profile.toRemembered());
    } on Object {
      // See above.
    }
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
    final profile = Profile(locale: user.locale, timeZone: user.timeZone);
    _changes++;
    _changed = profile;
    await _remember(services, profile);
    state = AsyncData(profile);
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

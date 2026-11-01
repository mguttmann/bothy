// SPDX-License-Identifier: Apache-2.0

/// Secrets of the app in the secure storage of the platform (docs/spec/15-client-architecture.md
/// 9; docs/DECISIONS.md D-M4-18-1): the refresh token, the device ID and the database key of
/// each instance, and the remembered server address.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;
import 'dart:math';

import 'package:auth/auth.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show MethodChannel, PlatformException;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:local_store/local_store.dart';

/// The raw key-value store under [InstanceSecrets] and [AppPreferences].
///
/// [FlutterSecureBackend] is the platform store; tests use an in-memory map.
abstract interface class SecureBackend {
  /// The value under [key], or null.
  Future<String?> read(String key);

  /// Stores [value] under [key]; durable when the future completes.
  Future<void> write(String key, String value);

  /// Removes [key]; a missing key is no error.
  Future<void> delete(String key);

  /// Every key the app stored.
  Future<Set<String>> keys();
}

/// [SecureBackend] on `flutter_secure_storage` (publisher steenbakker.dev).
///
/// macOS: the Data Protection keychain in a release build, not synchronized, accessible after
/// the first unlock on this device only; a debug or profile build keeps its items in the login
/// keychain, which has no access group: every app of the person reaches its items, and an app
/// signed differently from the writer gets a keychain access prompt. Windows: the package
/// default (a DPAPI-encrypted file in the support folder of the app, readable by every process
/// of the person). iOS (15 9 row iOS; brief M5-02 fixed requirement 17): the keychain of this
/// device only, never synchronized; the tokens after the first unlock, the database key only
/// while the device is unlocked ([iosOptionsFor]). Android (15 9 row Android; brief M5-03
/// Design note 3): an AES-GCM store whose key the Android Keystore wraps ([androidOptions]),
/// in the app's private shared preferences, which the backup rules exclude (15 6.3). The
/// package finds an item under every accessibility, so reads, deletes and the wipe need no
/// per-key options.
///
/// The app scope (docs/decisions.d/FIX-app-logout.md): the secrets belong to one app identity,
/// [appId] (the bundle identifier, on Android the application ID). On iOS and macOS they sit
/// under the keychain service [serviceFor] of it, elsewhere under the key prefix `<appId>/`.
/// A build with another identity therefore never reads, lists or deletes them, also not where
/// the platform store is shared (the login keychain, the Windows file).
///
/// The update from a build before the scope (docs/decisions.d/FIX-secure-store-migration.md):
/// before the first call the secrets of the old place (service `flutter_secure_storage_service`,
/// keys without prefix) are copied into the scope once, per instance and only for an instance
/// without secrets in the scope yet. Where only this app reaches the old store (iOS, Android,
/// the Data Protection keychain) every instance is its own; in a shared store (the login
/// keychain, Windows) only the instances with a folder under this app's data root
/// ([localInstances]), and the app keys only when the remembered instance (`app/instance`) is
/// one of them, so a new identity never takes another app's session and, without an instance
/// of its own, never reads the old store (no access prompt). In a shared store the copy also
/// records the instance's device key scope as the `instance_id`, the scope the old build's keys
/// have ([deviceKeyScopeKey]). The old items stay until the first successful refresh of their
/// instance (a write of its refresh token, [legacyPendingKey]) and are deleted then; a failed
/// copy is logged, leaves the old items alone and is tried again at the next start, and the app
/// starts without them (enrollment). The marker [scopeMarkerKey] (the app identity) records the
/// step.
///
/// The login keychain cannot list its items with their data: `readAll` fails with -50
/// (`errSecParam`). There the backend keeps the list of its keys in the item [indexKey] and
/// finds the old secrets by their names ([legacyAppKeys], [legacyInstanceKeys],
/// [legacyAuthKeys]).
///
/// No read of the old place ever shows a dialog or blocks the start
/// (docs/decisions.d/FIX-macos-legacy-keychain-prompt.md). An item of the macOS login keychain
/// whose access list trusts another app (a build signed with another bundle identifier) makes
/// `SecItemCopyMatching` show a keychain access dialog and wait for it, also with
/// `kSecUseAuthenticationUI`; so the old items of the login keychain are read and deleted by
/// [LegacyKeychain], which turns the user interaction of the keychain off for the call and gets
/// `errSecAuthFailed` instead of a dialog. Every other old read (iOS, the Data Protection
/// keychain) carries `kSecUseAuthenticationUI` = skip ([noAuthenticationUI]). An old store that
/// refuses an item without a dialog ([LegacyKeychain.deniedStatuses]) is not migratable: the
/// copy takes nothing, logs it once, records the marker, and the app starts with enrollment.
/// The copy has [legacyDeadline]; one that does not finish in time is abandoned (it writes
/// nothing more) and the app starts without it. Every call of the backend has [callDeadline],
/// so no start waits for the keychain without an end.
///
/// A macOS release build without the keychain entitlements (an ad-hoc signature, no team and no
/// provisioning profile) cannot use the Data Protection keychain: every write fails with
/// `errSecMissingEntitlement` (-34018) and every read finds nothing. Before its first call the
/// backend tries one write there; on -34018 it keeps the secrets in the login keychain for the
/// rest of the process, logs the downgrade once and reports it in [downgraded]
/// (docs/decisions.d/FIX-macos-keychain.md). A signed build keeps the Data Protection keychain.
final class FlutterSecureBackend implements SecureBackend {
  /// Creates the backend of the app identity [appId]; [storage] replaces the platform store in
  /// tests, [platform] the platform, [keychainOptions] the macOS options of this build,
  /// [localInstances] the instance IDs with a folder under this app's data root (none: no
  /// instance), [legacyKeychain] the reader of the old items in the macOS login keychain and
  /// [log] the developer log.
  FlutterSecureBackend({
    required this.appId,
    FlutterSecureStorage? storage,
    TargetPlatform? platform,
    MacOsOptions keychainOptions = FlutterSecureBackend.macOsOptions,
    this.localInstances,
    this.legacyKeychain = const MethodChannelLegacyKeychain(),
    this.legacyDeadline = const Duration(seconds: 5),
    this.callDeadline = const Duration(seconds: 15),
    void Function(String line)? log,
  }) : _storage =
           storage ??
           FlutterSecureStorage(
             mOptions: keychainOptions,
             iOptions: iosOptions,
             aOptions: androidOptions,
           ),
       _platform = platform ?? (kIsWeb ? null : defaultTargetPlatform),
       _keychainOptions = keychainOptions,
       _log = log ?? _developerLog;

  /// The app identity whose secrets this backend keeps.
  final String appId;

  /// The instance IDs that have a folder under this app's data root; null counts as none.
  final Future<Set<String>> Function()? localInstances;

  /// The reader of the old items in the macOS login keychain, never with a dialog.
  final LegacyKeychain legacyKeychain;

  /// How long the start waits for the copy of the old secrets; then it goes on without them.
  final Duration legacyDeadline;

  /// How long one call ([read], [write], [delete], [keys]) may wait for the platform store;
  /// then it fails with a [TimeoutException] instead of holding the start.
  final Duration callDeadline;

  /// The value of `kSecUseAuthenticationUI` on every old read of iOS and the Data Protection
  /// keychain: `kSecUseAuthenticationUISkip` (`u_AuthUIS`), so an item that needs the person is
  /// skipped instead of asked for (pinned by macos/LegacyKeychainTests).
  static const String noAuthenticationUI = 'u_AuthUIS';

  /// The options of the Android store: the data in AES-GCM, its key wrapped by an RSA key of
  /// the Android Keystore, no biometric prompt (the device keys of M5-01 carry the step-up).
  static const AndroidOptions androidOptions = AndroidOptions();

  /// The options of the iOS keychain for tokens and IDs: `AfterFirstUnlockThisDeviceOnly`.
  static const IOSOptions iosOptions = IOSOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
    synchronizable: false,
  );

  /// The options of the iOS keychain for the database key: `WhenUnlockedThisDeviceOnly`.
  static const IOSOptions iosDatabaseKeyOptions = IOSOptions(
    accessibility: KeychainAccessibility.unlocked_this_device,
    synchronizable: false,
  );

  /// The iOS options of the secret [key]: [iosDatabaseKeyOptions] for the database key of an
  /// instance ([SecureDatabaseKeySource.keyName]), [iosOptions] for every other key.
  ///
  /// Example: `iosOptionsFor('instance/<id>/database_key')` is [iosDatabaseKeyOptions].
  static IOSOptions iosOptionsFor(String key) =>
      key.endsWith('/${SecureDatabaseKeySource.keyName}')
      ? iosDatabaseKeyOptions
      : iosOptions;

  /// The options of the macOS keychain.
  static const MacOsOptions macOsOptions = MacOsOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
    synchronizable: false,
    usesDataProtectionKeychain: kReleaseMode,
  );

  /// The keychain service of the secrets of [appId] on iOS and macOS.
  ///
  /// Example: `serviceFor('com.example.chat')` is `com.example.chat.secrets`.
  static String serviceFor(String appId) => '$appId.secrets';

  /// The key of the item that tries the Data Protection keychain; it never holds a secret.
  static const String probeKey = 'app/keychain_probe';

  /// The key of the item in the app scope that records the copy of the old secrets (its value
  /// is the app identity); never a secret and never listed by [keys].
  static const String scopeMarkerKey = 'app/secret_scope';

  /// The key of the item in the app scope that lists the keys of the scope where the store
  /// cannot list them (the login keychain); never listed by [keys].
  static const String indexKey = 'app/secret_index';

  /// The key of the item in the app scope that names the old items still waiting for the first
  /// refresh of their instance: a JSON object from the `instance_id` to the old keys, the old
  /// app keys under ""; never listed by [keys].
  static const String legacyPendingKey = 'app/legacy_pending';

  /// The name of the stored device key scope in an instance's secrets ([DeviceKeyScopes]).
  static const String deviceKeyScopeKey = 'device_key_scope';

  /// The key prefixes of the secrets a build before the app scope wrote.
  static const List<String> legacyPrefixes = ['app/', 'instance/'];

  /// The app keys a build before the app scope wrote; frozen with that build.
  static const List<String> legacyAppKeys = [
    'app/server_address',
    'app/instance',
    'app/update_notice_dismissed',
  ];

  /// The instance keys (after `instance/<instance_id>/`) a build before the app scope wrote,
  /// besides [legacyAuthKeys]; frozen with that build.
  static const Set<String> legacyInstanceKeys = {
    'database_key',
    'app_lock/enabled',
    'push/apns_token',
    'push/fcm_token',
    'onboarding/pending_device',
  };

  /// The keys of a server session (after `instance/<instance_id>/auth/<origin>/`) a build
  /// before the app scope wrote; frozen with that build.
  static const List<String> legacyAuthKeys = ['refresh_token', 'device_id'];

  static const Set<String> _internalKeys = {
    scopeMarkerKey,
    indexKey,
    legacyPendingKey,
    probeKey,
  };

  static final RegExp _refreshTokenKey = RegExp(
    r'^instance/([^/]+)/auth/.+/refresh_token$',
  );

  /// The status `errSecMissingEntitlement` of the Security framework.
  static const int missingEntitlementStatus = -34018;

  final FlutterSecureStorage _storage;
  final TargetPlatform? _platform;
  final MacOsOptions _keychainOptions;
  final void Function(String line) _log;
  Future<MacOsOptions>? _chosen;
  bool _downgraded = false;
  Set<String>? _index;
  bool _pendingChecked = false;
  Future<void> _serial = Future<void>.value();

  bool get _macOS => _platform == TargetPlatform.macOS;
  bool get _apple => _macOS || _platform == TargetPlatform.iOS;

  static void _developerLog(String line) =>
      developer.log(line, name: 'secure_store');

  /// Whether [error] is the answer of the keychain to a process without the keychain
  /// entitlements: `flutter_secure_storage` puts the `OSStatus` into `details`.
  static bool isMissingEntitlement(Object error) =>
      error is PlatformException && error.details == missingEntitlementStatus;

  /// Whether the secrets went to the login keychain because the Data Protection keychain is not
  /// available to this build; false on every other platform.
  Future<bool> get downgraded async {
    await _ready();
    return _downgraded;
  }

  /// Whether the secrets lie in a store every app of the person reaches: the Windows file, the
  /// macOS login keychain (a debug or profile build, or a build without keychain entitlements).
  Future<bool> get sharedStore async => _shared(await _ready());

  bool _shared(MacOsOptions mac) =>
      _platform == TargetPlatform.windows ||
      (_macOS && !mac.usesDataProtectionKeychain);

  /// Whether the store cannot list its items (the login keychain), so [indexKey] lists them.
  bool _indexed(MacOsOptions mac) => _macOS && !mac.usesDataProtectionKeychain;

  /// The macOS options of every call, chosen once before the first call, after the old secrets
  /// were copied into the app scope. A failed choice is not kept, so the next call tries
  /// again; a failed copy is logged and does not fail the call.
  Future<MacOsOptions> _ready() {
    final chosen = _chosen;
    if (chosen != null) return chosen;
    final choice = _choose().then((options) async {
      final adoption = _Adoption();
      try {
        await _adoptLegacy(options, adoption).timeout(legacyDeadline);
      } on TimeoutException {
        adoption.abandoned = true;
        _log(
          'the copy of the secrets of the build before the app scope did not finish '
          'within ${legacyDeadline.inSeconds} s: the app starts without them, the next '
          'start tries again',
        );
      }
      return options;
    });
    _chosen = choice;
    return choice.catchError((Object error) {
      if (identical(_chosen, choice)) _chosen = null;
      throw error;
    });
  }

  Future<MacOsOptions> _choose() async {
    final options = _scoped(_keychainOptions);
    if (!_macOS || !options.usesDataProtectionKeychain) return options;
    try {
      await _storage.write(key: probeKey, value: '1', mOptions: options);
    } on PlatformException catch (error) {
      if (!isMissingEntitlement(error)) rethrow;
      _downgraded = true;
      _log(
        'the Data Protection keychain is not available to this build '
        '(errSecMissingEntitlement, no keychain entitlements): the secrets are kept '
        'in the login keychain',
      );
      return _macOptions(options, options.accountName, dp: false);
    }
    await _storage.delete(key: probeKey, mOptions: options);
    return options;
  }

  MacOsOptions _scoped(MacOsOptions options) => _macOptions(
    options,
    serviceFor(appId),
    dp: options.usesDataProtectionKeychain,
  );

  /// The macOS options of the old place: the old service in the keychain of this build, never
  /// with a dialog.
  MacOsOptions _legacyMac(MacOsOptions scoped) => _macOptions(
    scoped,
    AppleOptions.defaultAccountName,
    dp: scoped.usesDataProtectionKeychain,
    authenticationUI: noAuthenticationUI,
  );

  static MacOsOptions _macOptions(
    MacOsOptions options,
    String? service, {
    required bool dp,
    String? authenticationUI,
  }) => MacOsOptions(
    accountName: service,
    accessibility: options.accessibility,
    synchronizable: options.synchronizable,
    usesDataProtectionKeychain: dp,
    authenticationUIBehavior: authenticationUI,
  );

  /// The iOS options of [key] under [service].
  static IOSOptions _iosOptions(
    String key,
    String service, {
    String? authenticationUI,
  }) {
    final options = iosOptionsFor(key);
    return IOSOptions(
      accountName: service,
      accessibility: options.accessibility,
      synchronizable: options.synchronizable,
      authenticationUIBehavior: authenticationUI,
    );
  }

  /// The iOS options of the old item [key]: the old service, never with a dialog.
  static IOSOptions _legacyIos(String key) => _iosOptions(
    key,
    AppleOptions.defaultAccountName,
    authenticationUI: noAuthenticationUI,
  );

  String get _prefix => _apple ? '' : '$appId/';

  /// Runs [step] after every earlier one, so the read-modify-write of [indexKey] and
  /// [legacyPendingKey] never interleaves.
  Future<T> _serialized<T>(Future<T> Function() step) {
    final result = _serial.then((_) => step());
    _serial = result.then<void>((_) {}, onError: (Object _) {});
    return result;
  }

  /// Copies the secrets of a build before the app scope into it (see the class comment); never
  /// throws. Writes nothing once [adoption] is abandoned.
  Future<void> _adoptLegacy(MacOsOptions scoped, _Adoption adoption) async {
    try {
      if (await _read(scopeMarkerKey, scoped) != null) return;
      final Set<String>? own = _shared(scoped)
          ? await (localInstances?.call() ?? Future.value(<String>{}))
          : null;
      try {
        if (own == null || own.isNotEmpty) {
          await _copyLegacy(scoped, own, adoption);
        }
      } on _NotMigratable catch (refused) {
        _log(
          'the secrets of the build before the app scope are not migratable: the '
          'keychain refuses ${refused.key} without a dialog (status ${refused.status}); '
          'the app starts without them (enrollment)',
        );
      }
      if (adoption.abandoned) return;
      await _write(scopeMarkerKey, appId, scoped);
    } on Object catch (error) {
      _log(
        'the secrets of the build before the app scope were not copied ($error): '
        'the app starts without them, the next start tries again',
      );
    }
  }

  /// Copies the old secrets of the instances in [own] (null: every instance) into [scoped];
  /// throws [_NotMigratable] before any write when the old store refuses an item.
  Future<void> _copyLegacy(
    MacOsOptions scoped,
    Set<String>? own,
    _Adoption adoption,
  ) async {
    final legacy = await _legacyItems(_legacyMac(scoped), own);
    if (legacy.isEmpty || adoption.abandoned) return;
    final current = await _scopedKeys(scoped);
    final pending = <String, List<String>>{};
    for (final key in legacy.keys) {
      final id = _instanceOf(key);
      if (id == null) continue;
      if (own != null && !own.contains(id)) continue;
      if (current.any((k) => k.startsWith('instance/$id/'))) continue;
      (pending[id] ??= []).add(key);
    }
    final appKeys = [
      for (final key in legacy.keys)
        if (key.startsWith('app/') && !_internalKeys.contains(key)) key,
    ];
    final remembered = _instanceIdOf(legacy['app/instance']);
    if (own == null || (remembered != null && own.contains(remembered))) {
      if (appKeys.isNotEmpty) pending[''] = appKeys;
    }
    if (pending.isEmpty) return;
    var copied = 0;
    for (final key in pending.values.expand((keys) => keys)) {
      if (adoption.abandoned) return;
      if (current.contains(key)) continue;
      await _put(key, legacy[key]!, scoped);
      copied++;
    }
    if (_shared(scoped)) {
      for (final id in pending.keys.where((id) => id.isNotEmpty)) {
        await _put('instance/$id/$deviceKeyScopeKey', id, scoped);
      }
    }
    await _write(legacyPendingKey, jsonEncode(pending), scoped);
    _log(
      'copied $copied secrets of the build before the app scope into the scope of '
      '$appId (instances ${pending.keys.where((id) => id.isNotEmpty).join(', ')}); '
      'the old items stay until the first refresh',
    );
  }

  /// The secrets a build before the app scope left, under [legacyPrefixes]: listed where the
  /// store can list them, else read by their names for the instances in [own].
  Future<Map<String, String>> _legacyItems(
    MacOsOptions legacyMac,
    Set<String>? own,
  ) async {
    if (_indexed(legacyMac)) return _legacyByName(own ?? const {});
    final Map<String, String> all;
    try {
      all = await _storage.readAll(
        iOptions: _apple ? _legacyIos('') : null,
        mOptions: legacyMac,
      );
    } on PlatformException catch (error) {
      final status = error.details;
      if (status is int && LegacyKeychain.deniedStatuses.contains(status)) {
        throw _NotMigratable('the old items', status);
      }
      rethrow;
    }
    return {
      for (final MapEntry(:key, :value) in all.entries)
        if (legacyPrefixes.any(key.startsWith) && !_internalKeys.contains(key))
          key: value,
    };
  }

  /// The old secrets of the instances [ids] in the login keychain, read by their names without
  /// a dialog ([LegacyKeychain]): the app keys, the instance keys and the session keys under
  /// the origin of the remembered server. Throws [_NotMigratable] when the keychain refuses one.
  Future<Map<String, String>> _legacyByName(Set<String> ids) async {
    final found = <String, String>{};
    Future<void> take(String key) async {
      final (:status, :value) = await legacyKeychain.read(
        AppleOptions.defaultAccountName,
        key,
      );
      if (status == LegacyKeychain.itemNotFound) return;
      if (LegacyKeychain.deniedStatuses.contains(status)) {
        throw _NotMigratable(key, status);
      }
      if (status != LegacyKeychain.success || value == null) {
        throw StateError('the old item $key answered status $status');
      }
      found[key] = value;
    }

    for (final key in legacyAppKeys) {
      await take(key);
    }
    final origins = <String>{
      for (final address in [
        found['app/server_address'],
        _addressOf(found['app/instance']),
      ])
        if (address != null && Uri.tryParse(address)?.hasAuthority == true)
          Uri.parse(address).origin,
    };
    for (final id in ids) {
      for (final name in legacyInstanceKeys) {
        await take('instance/$id/$name');
      }
      for (final origin in origins) {
        for (final name in legacyAuthKeys) {
          await take('instance/$id/auth/$origin/$name');
        }
      }
    }
    return found;
  }

  static String? _instanceOf(String key) {
    if (!key.startsWith('instance/')) return null;
    final rest = key.substring('instance/'.length);
    final slash = rest.indexOf('/');
    return slash <= 0 ? null : rest.substring(0, slash);
  }

  static Map<Object?, Object?>? _jsonMap(String? json) {
    if (json == null) return null;
    try {
      final decoded = jsonDecode(json);
      return decoded is Map ? decoded : null;
    } on FormatException {
      return null;
    }
  }

  static String? _instanceIdOf(String? instanceJson) {
    final id = _jsonMap(instanceJson)?['instance_id'];
    return id is String ? id : null;
  }

  static String? _addressOf(String? instanceJson) {
    final address = _jsonMap(instanceJson)?['address'];
    return address is String ? address : null;
  }

  /// Deletes the old items of [instanceId] after its first successful refresh (see the class
  /// comment), and the old app keys with the last instance; never throws.
  Future<void> _retireLegacy(
    String instanceId,
    MacOsOptions scoped,
  ) => _serialized(() async {
    if (_pendingChecked) return;
    try {
      final pending = _jsonMap(await _read(legacyPendingKey, scoped));
      if (pending == null) {
        _pendingChecked = true;
        return;
      }
      final keys = pending.remove(instanceId);
      if (keys is! List) return;
      final last = !pending.keys.any((id) => id != '');
      final appKeys = last ? pending.remove('') : null;
      final drop = [...keys, if (appKeys is List) ...appKeys];
      final legacyMac = _legacyMac(scoped);
      for (final key in drop.whereType<String>()) {
        if (_indexed(legacyMac)) {
          final status = await legacyKeychain.delete(
            AppleOptions.defaultAccountName,
            key,
          );
          if (status != LegacyKeychain.success &&
              status != LegacyKeychain.itemNotFound) {
            throw StateError('the old item $key answered status $status');
          }
          continue;
        }
        await _storage.delete(
          key: key,
          iOptions: _apple ? _legacyIos(key) : null,
          mOptions: legacyMac,
        );
      }
      if (pending.isEmpty) {
        await _remove(legacyPendingKey, scoped);
        _pendingChecked = true;
      } else {
        await _write(legacyPendingKey, jsonEncode(pending), scoped);
      }
      _log(
        'removed ${drop.length} old secrets of instance $instanceId after its '
        'first refresh',
      );
    } on Object catch (error) {
      _log(
        'the old secrets of instance $instanceId were not removed ($error): '
        'the next refresh tries again',
      );
    }
  });

  Future<String?> _read(String key, MacOsOptions mac) => _storage.read(
    key: '$_prefix$key',
    iOptions: _iosOptions(key, serviceFor(appId)),
    mOptions: mac,
  );

  Future<void> _write(String key, String value, MacOsOptions mac) =>
      _storage.write(
        key: '$_prefix$key',
        value: value,
        iOptions: _iosOptions(key, serviceFor(appId)),
        mOptions: mac,
      );

  Future<void> _remove(String key, MacOsOptions mac) => _storage.delete(
    key: '$_prefix$key',
    iOptions: _iosOptions(key, serviceFor(appId)),
    mOptions: mac,
  );

  /// Writes [key] and lists it in [indexKey] where the store needs the index.
  Future<void> _put(String key, String value, MacOsOptions mac) async {
    await _write(key, value, mac);
    if (!_indexed(mac) || _internalKeys.contains(key)) return;
    await _serialized(() async {
      final index = await _loadIndex(mac);
      if (index.add(key)) await _saveIndex(index, mac);
    });
  }

  Future<Set<String>> _loadIndex(MacOsOptions mac) async {
    final known = _index;
    if (known != null) return known;
    final raw = await _read(indexKey, mac);
    final decoded = raw == null ? null : jsonDecode(raw);
    return _index = {if (decoded is List) ...decoded.whereType<String>()};
  }

  Future<void> _saveIndex(Set<String> index, MacOsOptions mac) =>
      _write(indexKey, jsonEncode(index.toList()..sort()), mac);

  /// Every key of the app scope without the prefix and without the internal items.
  Future<Set<String>> _scopedKeys(MacOsOptions mac) async {
    if (_indexed(mac)) {
      return _serialized(() async => {...await _loadIndex(mac)});
    }
    final all = await _storage.readAll(
      iOptions: _iosOptions('', serviceFor(appId)),
      mOptions: mac,
    );
    final prefix = _prefix;
    return {
      for (final key in all.keys)
        if (key.startsWith(prefix)) key.substring(prefix.length),
    }..removeAll(_internalKeys);
  }

  /// Runs [call] with [callDeadline]; a call the platform store does not answer in time fails
  /// with a [TimeoutException] and is logged.
  Future<T> _bounded<T>(
    String what,
    Future<T> Function() call,
  ) => call().timeout(
    callDeadline,
    onTimeout: () {
      _log(
        'the keychain did not answer $what within ${callDeadline.inSeconds} s: '
        'the call fails instead of holding the app',
      );
      throw TimeoutException('secure store $what', callDeadline);
    },
  );

  @override
  Future<String?> read(String key) =>
      _bounded('read', () async => _read(key, await _ready()));

  @override
  Future<void> write(String key, String value) => _bounded('write', () async {
    final mac = await _ready();
    await _put(key, value, mac);
    final refreshed = _refreshTokenKey.firstMatch(key)?.group(1);
    if (refreshed != null) await _retireLegacy(refreshed, mac);
  });

  @override
  Future<void> delete(String key) => _bounded('delete', () async {
    final mac = await _ready();
    await _remove(key, mac);
    if (!_indexed(mac)) return;
    await _serialized(() async {
      final index = await _loadIndex(mac);
      if (index.remove(key)) await _saveIndex(index, mac);
    });
  });

  @override
  Future<Set<String>> keys() =>
      _bounded('keys', () async => _scopedKeys(await _ready()));
}

/// One copy of the old secrets; [abandoned] once the start stopped waiting for it.
final class _Adoption {
  bool abandoned = false;
}

/// The old store refused [key] without a dialog ([status]): the old secrets are not migratable.
final class _NotMigratable implements Exception {
  const _NotMigratable(this.key, this.status);

  final String key;
  final int status;
}

/// The answer of [LegacyKeychain.read]: the `OSStatus` of the Security framework and the value
/// (only with [LegacyKeychain.success]).
typedef LegacyKeychainRead = ({int status, String? value});

/// The old items of the macOS login keychain, read and deleted without any user interaction
/// (docs/decisions.d/FIX-macos-legacy-keychain-prompt.md).
///
/// The login keychain guards each item with an access list of the apps it trusts; a read by any
/// other app shows a dialog and waits for it, and `kSecUseAuthenticationUI` does not stop that
/// dialog (measured, macos/LegacyKeychainTests). The runner's `LegacyKeychain` turns the user
/// interaction of the keychain off around the call (`SecKeychainSetUserInteractionAllowed`), so
/// such an item answers `errSecAuthFailed` (-25293) at once.
abstract interface class LegacyKeychain {
  /// `errSecSuccess`.
  static const int success = 0;

  /// `errSecItemNotFound`.
  static const int itemNotFound = -25300;

  /// The answers of an item the keychain does not give out without the person:
  /// `errSecAuthFailed` (an access list without this app, user interaction off),
  /// `errSecInteractionNotAllowed`, `errSecNoAccessForItem`, `errSecUserCanceled`,
  /// `errSecInvalidOwnerEdit` (a delete the access list refuses).
  static const Set<int> deniedStatuses = {-25293, -25308, -25243, -128, -25244};

  /// Reads the item [account] of the generic password service [service].
  Future<LegacyKeychainRead> read(String service, String account);

  /// Deletes the item [account] of [service]; the `OSStatus`.
  Future<int> delete(String service, String account);
}

/// [LegacyKeychain] over the channel `bothy/legacy_keychain` of the macOS runner
/// (macos/Runner/LegacyKeychain.swift).
final class MethodChannelLegacyKeychain implements LegacyKeychain {
  /// Creates the reader on the runner's channel.
  const MethodChannelLegacyKeychain();

  static const MethodChannel _channel = MethodChannel('bothy/legacy_keychain');

  @override
  Future<LegacyKeychainRead> read(String service, String account) async {
    final answer = await _channel.invokeMapMethod<String, Object?>('read', {
      'service': service,
      'account': account,
    });
    return (
      status: answer?['status'] as int? ?? LegacyKeychain.itemNotFound,
      value: answer?['value'] as String?,
    );
  }

  @override
  Future<int> delete(String service, String account) async =>
      await _channel.invokeMethod<int>('delete', {
        'service': service,
        'account': account,
      }) ??
      LegacyKeychain.itemNotFound;
}

/// The [SecretStore] of one instance: every key is prefixed with the instance ID, so the
/// secrets of an instance can be deleted together ([deleteAll]) and two instances never share
/// a key.
///
/// Example:
/// ```dart
/// final secrets = InstanceSecrets(backend, instanceId);
/// await secrets.write('device_id', id); // stored as '<instanceId>/device_id'
/// ```
final class InstanceSecrets implements SecretStore {
  /// Creates the store of [instanceId] on [backend].
  InstanceSecrets(this._backend, this.instanceId);

  final SecureBackend _backend;

  /// The `instance_id` of discovery.
  final String instanceId;

  String get _prefix => 'instance/$instanceId/';

  @override
  Future<String?> read(String key) => _backend.read('$_prefix$key');

  @override
  Future<void> write(String key, String value) =>
      _backend.write('$_prefix$key', value);

  @override
  Future<void> delete(String key) => _backend.delete('$_prefix$key');

  /// Whether this instance has no secret yet.
  Future<bool> isEmpty() async =>
      !(await _backend.keys()).any((key) => key.startsWith(_prefix));

  /// Deletes every secret of this instance: tokens, device ID and database key (R-SY-14).
  Future<void> deleteAll() async {
    for (final key in await _backend.keys()) {
      if (key.startsWith(_prefix)) await _backend.delete(key);
    }
  }
}

/// The [DatabaseKeySource] of an instance: 32 random bytes from `Random.secure()`, created at
/// the first start and kept in the secret store (15 6.3).
final class SecureDatabaseKeySource implements DatabaseKeySource {
  /// Creates the source over [secrets]; [random] replaces the secure source in tests.
  SecureDatabaseKeySource(this._secrets, {Random? random})
    : _random = random ?? Random.secure();

  /// The key name of the database key in the instance's secrets.
  static const String keyName = 'database_key';

  final SecretStore _secrets;
  final Random _random;

  @override
  Future<Uint8List> obtainKey() async {
    final stored = await _secrets.read(keyName);
    if (stored != null) return base64Decode(stored);
    final key = Uint8List.fromList(
      List<int>.generate(databaseKeyBytes, (_) => _random.nextInt(256)),
    );
    await _secrets.write(keyName, base64Encode(key));
    return key;
  }
}

/// What the app remembers outside every instance: the server address (01 5.6, B13) and the
/// public description of its instance (name, accent, login methods), no secret.
final class AppPreferences {
  /// Creates the preferences on [backend].
  AppPreferences(this._backend);

  static const String _serverAddressKey = 'app/server_address';

  final SecureBackend _backend;

  /// The address of the last instance E1 connected to, or null.
  Future<Uri?> serverAddress() async {
    final value = await _backend.read(_serverAddressKey);
    return value == null ? null : Uri.tryParse(value);
  }

  /// Remembers [address].
  Future<void> setServerAddress(Uri address) =>
      _backend.write(_serverAddressKey, address.toString());

  static const String _instanceKey = 'app/instance';

  /// The public description of the last instance discovery found, as JSON, or null: lets a
  /// signed-in app start without network (docs/DECISIONS.md D-M4-18-13).
  Future<String?> instanceJson() => _backend.read(_instanceKey);

  /// Remembers the description of the instance.
  Future<void> setInstanceJson(String json) =>
      _backend.write(_instanceKey, json);

  static const String _updateNoticeKey = 'app/update_notice_dismissed';

  /// The recommended version whose notice the person dismissed, or null (brief M7-13
  /// requirement 14: once per version).
  Future<String?> dismissedUpdateNotice() => _backend.read(_updateNoticeKey);

  /// Remembers that the notice for [version] was dismissed.
  Future<void> setDismissedUpdateNotice(String version) =>
      _backend.write(_updateNoticeKey, version);
}

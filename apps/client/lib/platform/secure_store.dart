// SPDX-License-Identifier: Apache-2.0

/// Secrets of the app in the secure storage of the platform (docs/spec/15-client-architecture.md
/// 9; docs/DECISIONS.md D-M4-18-1): the refresh token, the device ID and the database key of
/// each instance, and the remembered server address.
library;

import 'dart:convert';
import 'dart:developer' as developer;
import 'dart:math';

import 'package:auth/auth.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show PlatformException;
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
/// the platform store is shared (the login keychain, the Windows file). Secrets of a build
/// before the scope (service `flutter_secure_storage_service`, keys without prefix) move into
/// the scope once, before the first call, and leave the old place, so no stale refresh token
/// stays behind: always where only this app reaches the old store (iOS, Android, the Data
/// Protection keychain), in a shared store (the login keychain, Windows) only when
/// [hasLocalData] says this app has local data of an instance, the sign that the old secrets
/// are its own. Otherwise the shared store is not touched: no access prompt, no foreign
/// session. The marker [scopeMarkerKey] in the scope records the step.
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
  /// [hasLocalData] the check for local instance data (none: false) and [log] the developer
  /// log.
  FlutterSecureBackend({
    required this.appId,
    FlutterSecureStorage? storage,
    TargetPlatform? platform,
    MacOsOptions keychainOptions = FlutterSecureBackend.macOsOptions,
    this.hasLocalData,
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

  /// Whether this app has local data of an instance; null counts as no.
  final Future<bool> Function()? hasLocalData;

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

  /// The key of the item in the app scope that records the move of the old secrets; never a
  /// secret and never listed by [keys].
  static const String scopeMarkerKey = 'app/secret_scope';

  /// The key prefixes of the secrets a build before the app scope wrote.
  static const List<String> legacyPrefixes = ['app/', 'instance/'];

  /// The status `errSecMissingEntitlement` of the Security framework.
  static const int missingEntitlementStatus = -34018;

  final FlutterSecureStorage _storage;
  final TargetPlatform? _platform;
  final MacOsOptions _keychainOptions;
  final void Function(String line) _log;
  Future<MacOsOptions>? _chosen;
  bool _downgraded = false;

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
  Future<bool> get sharedStore async {
    if (_platform == TargetPlatform.windows) return true;
    if (!_macOS) return false;
    return !(await _ready()).usesDataProtectionKeychain;
  }

  /// The macOS options of every call, chosen once before the first call, after the old secrets
  /// moved into the app scope. A failed choice or move is not kept, so the next call tries
  /// again.
  Future<MacOsOptions> _ready() {
    final chosen = _chosen;
    if (chosen != null) return chosen;
    final choice = _choose().then((options) async {
      await _adoptLegacy(options);
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

  static MacOsOptions _macOptions(
    MacOsOptions options,
    String? service, {
    required bool dp,
  }) => MacOsOptions(
    accountName: service,
    accessibility: options.accessibility,
    synchronizable: options.synchronizable,
    usesDataProtectionKeychain: dp,
  );

  /// The iOS options of [key] under [service].
  static IOSOptions _iosOptions(String key, String service) {
    final options = iosOptionsFor(key);
    return IOSOptions(
      accountName: service,
      accessibility: options.accessibility,
      synchronizable: options.synchronizable,
    );
  }

  String get _prefix => _apple ? '' : '$appId/';

  /// Moves the secrets of a build before the app scope into it (see the class comment).
  Future<void> _adoptLegacy(MacOsOptions scoped) async {
    if (await _read(scopeMarkerKey, scoped) != null) return;
    final legacyMac = _macOptions(
      scoped,
      AppleOptions.defaultAccountName,
      dp: scoped.usesDataProtectionKeychain,
    );
    final shared =
        _platform == TargetPlatform.windows ||
        (_macOS && !scoped.usesDataProtectionKeychain);
    final check = hasLocalData;
    final adopt = !shared || (check != null && await check());
    if (adopt) await _moveLegacy(legacyMac, scoped);
    await _write(scopeMarkerKey, '1', scoped);
  }

  /// Moves every secret under [legacyMac] into [scoped] without overwriting a scoped one.
  Future<void> _moveLegacy(MacOsOptions legacyMac, MacOsOptions scoped) async {
    final legacy = await _legacyItems(legacyMac);
    final current = await _readAll(scoped);
    for (final MapEntry(:key, :value) in legacy.entries) {
      if (!current.containsKey(key)) await _write(key, value, scoped);
      await _storage.delete(
        key: key,
        iOptions: _apple ? _iosOptions(key, legacyMac.accountName!) : null,
        mOptions: legacyMac,
      );
    }
    if (legacy.isNotEmpty) {
      _log('moved ${legacy.length} secrets into the scope of $appId');
    }
  }

  /// The secrets a build before the app scope left: on Apple every item of the old service, on
  /// the other platforms the keys under [legacyPrefixes].
  Future<Map<String, String>> _legacyItems(MacOsOptions legacyMac) async {
    final all = await _storage.readAll(
      iOptions: _apple
          ? _iosOptions('', AppleOptions.defaultAccountName)
          : null,
      mOptions: legacyMac,
    );
    if (_apple) return all;
    return {
      for (final MapEntry(:key, :value) in all.entries)
        if (legacyPrefixes.any(key.startsWith)) key: value,
    };
  }

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

  /// Every secret of the app scope with its key without the prefix, the marker included.
  Future<Map<String, String>> _readAll(MacOsOptions mac) async {
    final all = await _storage.readAll(
      iOptions: _iosOptions('', serviceFor(appId)),
      mOptions: mac,
    );
    final prefix = _prefix;
    return {
      for (final MapEntry(:key, :value) in all.entries)
        if (key.startsWith(prefix)) key.substring(prefix.length): value,
    };
  }

  @override
  Future<String?> read(String key) async => _read(key, await _ready());

  @override
  Future<void> write(String key, String value) async =>
      _write(key, value, await _ready());

  @override
  Future<void> delete(String key) async {
    final mac = await _ready();
    await _storage.delete(
      key: '$_prefix$key',
      iOptions: _iosOptions(key, serviceFor(appId)),
      mOptions: mac,
    );
  }

  @override
  Future<Set<String>> keys() async =>
      (await _readAll(await _ready())).keys.toSet()..remove(scopeMarkerKey);
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

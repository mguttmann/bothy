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
/// the first unlock on this device only; a debug or profile build is not signed by a team and
/// so has no keychain access group, and keeps its items in the login keychain under the app's
/// access list. Windows: the package default (Credential Manager with AES-GCM files). iOS
/// (15 9 row iOS; brief M5-02 fixed requirement 17): the keychain of this device only, never
/// synchronized; the tokens after the first unlock, the database key only while the device
/// is unlocked ([iosOptionsFor]). Android (15 9 row Android; brief M5-03 Design note 3): an
/// AES-GCM store whose key the Android Keystore wraps ([androidOptions]), in the app's private
/// shared preferences, which the backup rules exclude (15 6.3). The package finds an item under
/// every accessibility, so reads, deletes and the wipe need no per-key options.
///
/// A macOS release build without the keychain entitlements (an ad-hoc signature, no team and no
/// provisioning profile) cannot use the Data Protection keychain: every write fails with
/// `errSecMissingEntitlement` (-34018) and every read finds nothing. Before its first call the
/// backend tries one write there; on -34018 it keeps the secrets in the login keychain for the
/// rest of the process, logs the downgrade once and reports it in [downgraded]
/// (docs/decisions.d/FIX-macos-keychain.md). A signed build keeps the Data Protection keychain.
final class FlutterSecureBackend implements SecureBackend {
  /// Creates the backend; [storage] replaces the platform store in tests, [macOS] the platform
  /// check, [keychainOptions] the macOS options of this build and [log] the developer log.
  FlutterSecureBackend({
    FlutterSecureStorage? storage,
    bool? macOS,
    MacOsOptions keychainOptions = FlutterSecureBackend.macOsOptions,
    void Function(String line)? log,
  }) : _storage =
           storage ??
           FlutterSecureStorage(
             mOptions: keychainOptions,
             iOptions: iosOptions,
             aOptions: androidOptions,
           ),
       _macOS =
           macOS ?? (!kIsWeb && defaultTargetPlatform == TargetPlatform.macOS),
       _keychainOptions = keychainOptions,
       _log = log ?? _developerLog;

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

  /// The key of the item that tries the Data Protection keychain; it never holds a secret.
  static const String probeKey = 'app/keychain_probe';

  /// The status `errSecMissingEntitlement` of the Security framework.
  static const int missingEntitlementStatus = -34018;

  final FlutterSecureStorage _storage;
  final bool _macOS;
  final MacOsOptions _keychainOptions;
  final void Function(String line) _log;
  Future<MacOsOptions>? _chosen;
  bool _downgraded = false;

  static void _developerLog(String line) =>
      developer.log(line, name: 'secure_store');

  /// Whether [error] is the answer of the keychain to a process without the keychain
  /// entitlements: `flutter_secure_storage` puts the `OSStatus` into `details`.
  static bool isMissingEntitlement(Object error) =>
      error is PlatformException && error.details == missingEntitlementStatus;

  /// Whether the secrets went to the login keychain because the Data Protection keychain is not
  /// available to this build; false on every other platform.
  Future<bool> get downgraded async {
    await _options();
    return _downgraded;
  }

  /// The macOS options of every call: chosen once, before the first call. A failed choice is
  /// not kept, so the next call tries again.
  Future<MacOsOptions> _options() {
    final chosen = _chosen;
    if (chosen != null) return chosen;
    final choice = _choose();
    _chosen = choice;
    return choice.catchError((Object error) {
      if (identical(_chosen, choice)) _chosen = null;
      throw error;
    });
  }

  Future<MacOsOptions> _choose() async {
    final options = _keychainOptions;
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
      return MacOsOptions(
        accountName: options.accountName,
        accessibility: options.accessibility,
        synchronizable: options.synchronizable,
        usesDataProtectionKeychain: false,
      );
    }
    await _storage.delete(key: probeKey, mOptions: options);
    return options;
  }

  @override
  Future<String?> read(String key) async =>
      _storage.read(key: key, mOptions: await _options());

  @override
  Future<void> write(String key, String value) async => _storage.write(
    key: key,
    value: value,
    iOptions: iosOptionsFor(key),
    mOptions: await _options(),
  );

  @override
  Future<void> delete(String key) async =>
      _storage.delete(key: key, mOptions: await _options());

  @override
  Future<Set<String>> keys() async =>
      (await _storage.readAll(mOptions: await _options())).keys.toSet();
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

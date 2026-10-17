// SPDX-License-Identifier: Apache-2.0

/// Secrets of the app in the secure storage of the platform (docs/spec/15-client-architektur.md
/// 9; docs/DECISIONS.md D-M4-18-1): the refresh token, the device ID and the database key of
/// each instance, and the remembered server address.
library;

import 'dart:convert';
import 'dart:math';

import 'package:auth/auth.dart';
import 'package:flutter/foundation.dart';
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
/// access list. Windows: the package default (Credential Manager with AES-GCM files).
final class FlutterSecureBackend implements SecureBackend {
  /// Creates the backend; [storage] replaces the platform store in tests.
  FlutterSecureBackend({FlutterSecureStorage? storage})
    : _storage = storage ?? const FlutterSecureStorage(mOptions: macOsOptions);

  /// The options of the macOS keychain.
  static const MacOsOptions macOsOptions = MacOsOptions(
    accessibility: KeychainAccessibility.first_unlock_this_device,
    synchronizable: false,
    usesDataProtectionKeychain: kReleaseMode,
  );

  final FlutterSecureStorage _storage;

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);

  @override
  Future<Set<String>> keys() async => (await _storage.readAll()).keys.toSet();
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
}

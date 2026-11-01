// SPDX-License-Identifier: Apache-2.0

// The platform stores under `FlutterSecureBackend` as the tests see them: one fake of
// `flutter_secure_storage` that answers like the store of each platform, and the writer of a
// build before the app scope (docs/decisions.d/FIX-app-logout.md,
// docs/decisions.d/FIX-secure-store-migration.md).

import 'package:client/platform/secure_store.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show PlatformException;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

/// The answer of `flutter_secure_storage_darwin` 0.4.3 to `readAll` in the macOS login keychain
/// (the file-based keychain): `SecItemCopyMatching` with `kSecMatchLimitAll` and
/// `kSecReturnData` returns `errSecParam` (-50) there, measured on macOS with a Swift process
/// (docs/decisions.d/FIX-secure-store-migration.md FIX-SSM/1). The Data Protection keychain and
/// the iOS keychain answer the same query.
PlatformException loginKeychainReadAllError() => PlatformException(
  code: 'Unexpected security result code',
  message: 'Code: -50, Message: One or more parameters passed to a function were not valid.',
  details: -50,
);

/// The answer of the Data Protection keychain to a write of an app without the keychain
/// entitlements (docs/decisions.d/FIX-macos-keychain.md).
PlatformException missingEntitlementError() => PlatformException(
  code: 'Unexpected security result code',
  message: "Code: -34018, Message: A required entitlement isn't present.",
  details: -34018,
);

/// One platform store as every app of the person sees it.
///
/// Apple items are told apart by their service (`accountName`), on macOS also by the keychain
/// (the Data Protection keychain or the login keychain); the other platforms by their key only.
/// The login keychain refuses `readAll` as the real one does ([loginKeychainReadAllError]); an
/// app without the keychain entitlements ([entitled] false) gets -34018 for every write to the
/// Data Protection keychain and finds nothing there.
class SharedStorage extends Fake implements FlutterSecureStorage {
  /// Creates the store of [platform].
  SharedStorage(this.platform, {this.entitled = true});

  /// The platform whose store this is.
  final TargetPlatform platform;

  /// Whether the app has the keychain entitlements (macOS).
  final bool entitled;

  /// The items per place ([placeOf]).
  final Map<String, Map<String, String>> items = {};

  /// The place of every read, list and delete, in order: a read of another app's service in
  /// the login keychain is an access prompt on macOS.
  final List<String> touched = [];

  /// Throws on the next `readAll` when set (a store that fails).
  Object? failReadAll;

  bool get _apple =>
      platform == TargetPlatform.iOS || platform == TargetPlatform.macOS;

  /// The place of [service] (Apple) in the Data Protection keychain ([dataProtection]) or the
  /// login keychain on macOS; "" on the platforms without services.
  String placeOf(String service, {bool dataProtection = false}) {
    if (!_apple) return '';
    if (platform == TargetPlatform.iOS) return service;
    return '${dataProtection ? 'dp' : 'login'}:$service';
  }

  /// The items of [service] (see [placeOf]).
  Map<String, String> at(String service, {bool dataProtection = false}) => items
      .putIfAbsent(placeOf(service, dataProtection: dataProtection), () => {});

  bool _dp(AppleOptions? m) =>
      platform == TargetPlatform.macOS &&
      ((m as MacOsOptions?) ?? MacOsOptions.defaultOptions)
          .usesDataProtectionKeychain;

  String _place(AppleOptions? i, AppleOptions? m) {
    if (!_apple) return '';
    final options = platform == TargetPlatform.iOS ? i : m;
    return placeOf(
      options?.accountName ?? AppleOptions.defaultAccountName,
      dataProtection: _dp(m),
    );
  }

  bool _blind(AppleOptions? m) => _dp(m) && !entitled;

  @override
  Future<void> write({
    required String key,
    required String? value,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    if (_blind(mOptions)) throw missingEntitlementError();
    items.putIfAbsent(_place(iOptions, mOptions), () => {})[key] = value!;
  }

  @override
  Future<String?> read({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final place = _place(iOptions, mOptions);
    touched.add(place);
    if (_blind(mOptions)) return null;
    return items[place]?[key];
  }

  @override
  Future<void> delete({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final place = _place(iOptions, mOptions);
    touched.add(place);
    items[place]?.remove(key);
  }

  @override
  Future<Map<String, String>> readAll({
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    final place = _place(iOptions, mOptions);
    touched.add(place);
    final failure = failReadAll;
    if (failure != null) throw failure;
    if (platform == TargetPlatform.macOS && !_dp(mOptions)) {
      throw loginKeychainReadAllError();
    }
    if (_blind(mOptions)) return {};
    return Map.of(items[place] ?? const {});
  }
}

/// The secrets as a build before the app scope kept them: on iOS and macOS under the service
/// `flutter_secure_storage_service` (on macOS in the Data Protection keychain when
/// [dataProtection], else in the login keychain), elsewhere under the bare key.
final class LegacyBackend implements SecureBackend {
  /// Creates the old build's view of [storage].
  LegacyBackend(this.storage, {this.dataProtection = false});

  /// The platform store.
  final SharedStorage storage;

  /// Whether the old build used the Data Protection keychain (macOS).
  final bool dataProtection;

  /// Where the old build kept its secrets.
  Map<String, String> get items => storage.at(
    AppleOptions.defaultAccountName,
    dataProtection: dataProtection,
  );

  /// The secrets at the old place only: on the platforms without services the old and the
  /// scoped keys share one place, and only the old ones have no app prefix.
  Map<String, String> get legacyItems => {
    for (final MapEntry(:key, :value) in items.entries)
      if (FlutterSecureBackend.legacyPrefixes.any(key.startsWith)) key: value,
  };

  @override
  Future<String?> read(String key) async => items[key];

  @override
  Future<void> write(String key, String value) async => items[key] = value;

  @override
  Future<void> delete(String key) async => items.remove(key);

  @override
  Future<Set<String>> keys() async => items.keys.toSet();
}

/// The macOS options of a debug or profile build: the login keychain.
const MacOsOptions loginKeychainOptions = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
  usesDataProtectionKeychain: false,
);

/// The macOS options of a release build: the Data Protection keychain (an app without the
/// keychain entitlements falls back to the login keychain).
const MacOsOptions dataProtectionOptions = MacOsOptions(
  accessibility: KeychainAccessibility.first_unlock_this_device,
  synchronizable: false,
);

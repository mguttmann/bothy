// SPDX-License-Identifier: Apache-2.0

// The platform stores under `FlutterSecureBackend` as the tests see them: one fake of
// `flutter_secure_storage` that answers like the store of each platform, and the writer of a
// build before the app scope (docs/decisions.d/FIX-app-logout.md,
// docs/decisions.d/FIX-secure-store-migration.md).

import 'dart:async';

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

  /// The old items of the login keychain whose access list trusts another app (the build of
  /// another bundle identifier wrote them): a plugin call on them shows the keychain dialog and
  /// waits for it, so it never completes here; [legacyKeychain] gets `errSecAuthFailed`.
  final Set<String> foreign = {};

  /// The keys of every plugin call on the old place of the login keychain, in order.
  final List<String> pluginLegacyCalls = [];

  /// The Apple options of every plugin call on the old service, in order.
  final List<AppleOptions?> legacyOptions = [];

  /// Calls that wait for a dialog nobody answers.
  final Completer<Never> _dialog = Completer<Never>();

  /// The runner's reader of the old login keychain items over this store.
  late final SharedLegacyKeychain legacyKeychain = SharedLegacyKeychain(this);

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

  /// Records a call on [key] at [place] and waits for the dialog of a [foreign] item.
  Future<void> _call(
    String key,
    String place,
    AppleOptions? i,
    AppleOptions? m,
  ) async {
    final legacy = placeOf(AppleOptions.defaultAccountName);
    if (_apple && place.endsWith(AppleOptions.defaultAccountName)) {
      legacyOptions.add(platform == TargetPlatform.iOS ? i : m);
    }
    if (platform != TargetPlatform.macOS || place != legacy) return;
    pluginLegacyCalls.add(key);
    if (foreign.contains(key)) await _dialog.future;
  }

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
    final place = _place(iOptions, mOptions);
    await _call(key, place, iOptions, mOptions);
    items.putIfAbsent(place, () => {})[key] = value!;
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
    await _call(key, place, iOptions, mOptions);
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
    await _call(key, place, iOptions, mOptions);
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
    if (_apple && place.endsWith(AppleOptions.defaultAccountName)) {
      legacyOptions.add(platform == TargetPlatform.iOS ? iOptions : mOptions);
    }
    final failure = failReadAll;
    if (failure != null) throw failure;
    if (platform == TargetPlatform.macOS && !_dp(mOptions)) {
      throw loginKeychainReadAllError();
    }
    if (_blind(mOptions)) return {};
    return Map.of(items[place] ?? const {});
  }
}

/// The runner's `LegacyKeychain` over a [SharedStorage] of macOS: the old items of the login
/// keychain without any dialog. A [SharedStorage.foreign] item answers `errSecAuthFailed`
/// (-25293) to a read and `errSecInvalidOwnerEdit` (-25244) to a delete, as the real keychain
/// does with the user interaction off (measured by macos/LegacyKeychainTests).
final class SharedLegacyKeychain implements LegacyKeychain {
  /// Creates the reader of [storage].
  SharedLegacyKeychain(this.storage);

  /// The platform store.
  final SharedStorage storage;

  /// The accounts of every call, in order.
  final List<String> calls = [];

  /// When set, no call ever answers (a channel that hangs).
  bool hang = false;

  /// The answer of a read of a [SharedStorage.foreign] item: `errSecAuthFailed`, or
  /// `errSecInteractionNotAllowed` (-25308) for a locked keychain.
  int foreignReadStatus = -25293;

  Map<String, String> _items(String service) => storage.at(service);

  @override
  Future<LegacyKeychainRead> read(String service, String account) async {
    calls.add(account);
    if (hang) await Completer<Never>().future;
    if (storage.foreign.contains(account)) {
      return (status: foreignReadStatus, value: null);
    }
    final value = _items(service)[account];
    return value == null
        ? (status: LegacyKeychain.itemNotFound, value: null)
        : (status: LegacyKeychain.success, value: value);
  }

  @override
  Future<int> delete(String service, String account) async {
    calls.add(account);
    if (hang) await Completer<Never>().future;
    if (storage.foreign.contains(account)) return -25244;
    return _items(service).remove(account) == null
        ? LegacyKeychain.itemNotFound
        : LegacyKeychain.success;
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

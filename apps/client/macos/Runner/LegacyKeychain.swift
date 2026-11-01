// SPDX-License-Identifier: Apache-2.0

import Foundation
import Security

/// The old items of the login keychain, read and deleted without any dialog
/// (docs/decisions.d/FIX-macos-legacy-keychain-prompt.md).
///
/// The login keychain guards each item with an access list of the apps it trusts. A read of an
/// item whose list does not name this app (a build with another bundle identifier wrote it) makes
/// `SecItemCopyMatching` show a keychain access dialog and wait for it; `kSecUseAuthenticationUI`
/// with skip or fail does not stop that dialog, it only covers items with an access control. So
/// every call here turns the user interaction of the keychain off
/// (`SecKeychainSetUserInteractionAllowed`, a setting of the whole process) and restores it
/// afterwards; such an item then answers `errSecAuthFailed` (-25293) at once, a delete
/// `errSecInvalidOwnerEdit` (-25244). The calls run one after another on `queue`, never on the
/// main thread. The checks in macos/LegacyKeychainTests run this file against a real keychain.
///
/// The Dart side is `LegacyKeychain` in lib/platform/secure_store.dart; the channel is
/// `LegacyKeychainChannel` in AppDelegate.swift.
enum LegacyKeychain {
  /// The queue of every call.
  static let queue = DispatchQueue(label: "legacy_keychain")

  /// Reads the generic password `account` of `service`: the status and, on success, the value as
  /// UTF-8. `keychains` limits the search to these keychains (the checks); nil is the search list
  /// of the user.
  static func read(service: String, account: String, keychains: [SecKeychain]? = nil)
    -> (status: OSStatus, value: String?)
  {
    var query = baseQuery(service: service, account: account, keychains: keychains)
    query[kSecReturnData] = true
    query[kSecMatchLimit] = kSecMatchLimitOne
    var result: CFTypeRef?
    let status = withoutUserInteraction { SecItemCopyMatching(query as CFDictionary, &result) }
    guard status == errSecSuccess else { return (status, nil) }
    guard let data = result as? Data, let value = String(data: data, encoding: .utf8) else {
      return (errSecDecode, nil)
    }
    return (status, value)
  }

  /// Deletes the generic password `account` of `service`; the status.
  static func delete(service: String, account: String, keychains: [SecKeychain]? = nil)
    -> OSStatus
  {
    let query = baseQuery(service: service, account: account, keychains: keychains)
    return withoutUserInteraction { SecItemDelete(query as CFDictionary) }
  }

  private static func baseQuery(service: String, account: String, keychains: [SecKeychain]?)
    -> [CFString: Any]
  {
    var query: [CFString: Any] = [
      kSecClass: kSecClassGenericPassword,
      kSecAttrService: service,
      kSecAttrAccount: account,
      // An item with an access control (not written by the old builds) fails instead of asking.
      kSecUseAuthenticationUI: kSecUseAuthenticationUIFail,
    ]
    if let keychains = keychains { query[kSecMatchSearchList] = keychains }
    return query
  }

  /// Runs `body` with the user interaction of the keychain off and restores the earlier setting.
  static func withoutUserInteraction<T>(_ body: () -> T) -> T {
    var allowed: DarwinBoolean = true
    SecKeychainGetUserInteractionAllowed(&allowed)
    SecKeychainSetUserInteractionAllowed(false)
    defer { SecKeychainSetUserInteractionAllowed(allowed.boolValue) }
    return body()
  }
}

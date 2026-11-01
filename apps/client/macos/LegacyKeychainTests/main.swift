// SPDX-License-Identifier: Apache-2.0

// The checks of `Runner/LegacyKeychain.swift` against a real keychain
// (FIX-macos-legacy-keychain-prompt). `run_tests.sh` creates the keychain and its items and runs
// this file with the keychain's path.
//
// Why not fakes: the storage fakes of the Dart tests answered every read at once, so the tests of
// the move into the app scope passed while the real app stopped at its start on a keychain
// dialog for an item another app wrote. Whether a query shows that dialog is decided by
// securityd from the item's access list and the caller's code signature; only a real keychain
// with a real foreign access list shows it. The plain read below proves the fixture guards its
// item; the reads of `LegacyKeychain` prove they get an answer without a dialog.

import Foundation
import Security

var failures = 0
func check(_ ok: Bool, _ what: String) {
  if ok {
    print("ok   \(what)")
  } else {
    failures += 1
    print("FAIL \(what)")
  }
  fflush(stdout)
}

let service = "flutter_secure_storage_service"
let foreign = "app/instance"
let own = "app/server_address"

/// A plain read with the user interaction of the keychain on, as the plugin reads; run in a child
/// process because it may wait for a dialog.
if CommandLine.arguments.count == 3 && CommandLine.arguments[1] == "plain" {
  var keychain: SecKeychain?
  SecKeychainOpen(CommandLine.arguments[2], &keychain)
  let query: [CFString: Any] = [
    kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: foreign,
    kSecReturnData: true, kSecMatchLimit: kSecMatchLimitOne,
    kSecUseAuthenticationUI: kSecUseAuthenticationUISkip, kSecMatchSearchList: [keychain!],
  ]
  var result: CFTypeRef?
  exit(SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess ? 0 : 1)
}

guard CommandLine.arguments.count == 2 else {
  print("usage: checks <keychain path>")
  exit(2)
}
let path = CommandLine.arguments[1]
var opened: SecKeychain?
guard SecKeychainOpen(path, &opened) == errSecSuccess, let keychain = opened else {
  print("the keychain \(path) does not open")
  exit(2)
}
let keychains = [keychain]

/// The statuses `LegacyKeychain.deniedStatuses` of lib/platform/secure_store.dart treats as "not
/// migratable".
let denied: Set<OSStatus> = [-25293, -25308, -25243, -128, -25244]

// The Dart side passes kSecUseAuthenticationUISkip by its value
// (`FlutterSecureBackend.noAuthenticationUI`).
check((kSecUseAuthenticationUISkip as String) == "u_AuthUIS", "kSecUseAuthenticationUISkip is u_AuthUIS")

// The fixture: a plain read of the foreign item, even with kSecUseAuthenticationUISkip, does not
// get the value; on a Mac with a logged-in session it waits for the dialog until it is stopped.
let plain = Process()
plain.executableURL = URL(fileURLWithPath: CommandLine.arguments[0])
plain.arguments = ["plain", path]
try plain.run()
let deadline = Date().addingTimeInterval(3)
while plain.isRunning && Date() < deadline { usleep(50_000) }
let waited = plain.isRunning
if waited { plain.terminate() }
plain.waitUntilExit()
check(waited || plain.terminationStatus != 0,
  "the plain read of the foreign item gets no value (waited for a dialog: \(waited))")

var allowedBefore: DarwinBoolean = true
SecKeychainGetUserInteractionAllowed(&allowedBefore)
let started = Date()

let refused = LegacyKeychain.read(service: service, account: foreign, keychains: keychains)
check(denied.contains(refused.status) && refused.value == nil,
  "the foreign item answers without a dialog: \(refused.status)")

let readable = LegacyKeychain.read(service: service, account: own, keychains: keychains)
check(readable.status == errSecSuccess && readable.value == "https://chat.example.com",
  "the item this app may read is read: \(readable.status)")

let missing = LegacyKeychain.read(service: service, account: "app/none", keychains: keychains)
check(missing.status == errSecItemNotFound, "a missing item is not found: \(missing.status)")

let keep = LegacyKeychain.delete(service: service, account: foreign, keychains: keychains)
check(denied.contains(keep), "the foreign item is not deleted, without a dialog: \(keep)")
var attributes: CFTypeRef?
let still = SecItemCopyMatching([
  kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: foreign,
  kSecReturnAttributes: true, kSecMatchSearchList: keychains,
] as CFDictionary, &attributes)
check(still == errSecSuccess, "the foreign item is still there: \(still)")

let removed = LegacyKeychain.delete(service: service, account: own, keychains: keychains)
check(removed == errSecSuccess, "the item this app may read is deleted: \(removed)")
let gone = LegacyKeychain.read(service: service, account: own, keychains: keychains)
check(gone.status == errSecItemNotFound, "and is gone: \(gone.status)")

let elapsed = Date().timeIntervalSince(started)
check(elapsed < 3, "every call answered at once (\(String(format: "%.3f", elapsed)) s)")

var allowedAfter: DarwinBoolean = false
SecKeychainGetUserInteractionAllowed(&allowedAfter)
check(allowedAfter.boolValue == allowedBefore.boolValue, "the user interaction setting is restored")

if failures > 0 {
  print("\(failures) check(s) failed")
  exit(1)
}
print("all checks passed")

// SPDX-License-Identifier: Apache-2.0
//
// The keychain choice of `KeychainKeyStore`: a macOS process without the keychain entitlements (a
// build without a team signature, a test binary) gets errSecMissingEntitlement from the Data
// Protection keychain; the store then keeps the keys in the login keychain without an access
// group, and the keys report `hardwareBound = false` (docs/decisions.d/FIX-macos-keychain.md).
// A fake keychain answers like the system does: -34018 for every write to the Data Protection
// keychain, errSecItemNotFound for every read of it.

import CryptoKit
import Foundation
import Security
import Testing

@testable import DeviceKeyCore

/// A keychain in memory that behaves like the system's for a process with or without the
/// keychain entitlements.
private final class FakeKeychain: KeychainAPI {
    /// Whether the process lacks the entitlements of the Data Protection keychain.
    let missingEntitlement: Bool
    /// The stored items by service and account, with the attributes they were written with.
    var items: [String: [String: Any]] = [:]
    /// Every `add` query, in order.
    var adds: [[String: Any]] = []

    init(missingEntitlement: Bool) {
        self.missingEntitlement = missingEntitlement
    }

    private static func key(_ query: [String: Any]) -> String {
        "\(query[kSecAttrService as String] as? String ?? "")\u{0}\(query[kSecAttrAccount as String] as? String ?? "")"
    }

    private static func dataProtection(_ query: [String: Any]) -> Bool {
        (query[kSecUseDataProtectionKeychain as String] as? Bool) == true
    }

    func add(_ query: [String: Any]) -> OSStatus {
        adds.append(query)
        let dataProtection = Self.dataProtection(query)
        if dataProtection && missingEntitlement {
            return errSecMissingEntitlement
        }
        // The login keychain refuses an access control with user presence the same way.
        if !dataProtection && query[kSecAttrAccessControl as String] != nil {
            return errSecMissingEntitlement
        }
        if items[Self.key(query)] != nil {
            return errSecDuplicateItem
        }
        var stored = query
        if !dataProtection {
            // The login keychain keeps no accessibility, group or synchronizable flag to read back.
            stored[kSecAttrAccessible as String] = nil
            stored[kSecAttrAccessGroup as String] = nil
            stored[kSecAttrSynchronizable as String] = nil
        }
        items[Self.key(query)] = stored
        return errSecSuccess
    }

    func copyMatching(_ query: [String: Any]) -> (OSStatus, CFTypeRef?) {
        if Self.dataProtection(query) && missingEntitlement {
            return (errSecItemNotFound, nil)
        }
        guard let item = items[Self.key(query)], Self.dataProtection(item) == Self.dataProtection(query) else {
            return (errSecItemNotFound, nil)
        }
        if (query[kSecReturnData as String] as? Bool) == true {
            return (errSecSuccess, item[kSecValueData as String] as CFTypeRef?)
        }
        var attributes = item
        attributes[kSecValueData as String] = nil
        return (errSecSuccess, attributes as CFDictionary)
    }

    func delete(_ query: [String: Any]) -> OSStatus {
        if Self.dataProtection(query) && missingEntitlement {
            return errSecItemNotFound
        }
        guard let item = items[Self.key(query)], Self.dataProtection(item) == Self.dataProtection(query) else {
            return errSecItemNotFound
        }
        _ = item
        items[Self.key(query)] = nil
        return errSecSuccess
    }
}

private let scope = "https://chat.example.com"
private let group = "TEAMID.com.example.shared"

private func manager(_ store: KeyStore, enclave: Bool = false) -> DeviceKeyManager {
    DeviceKeyManager(
        store: store,
        capabilities: PlatformCapabilities(enclaveAvailable: enclave, biometryEnrolled: false),
        labelPrefix: "test.device_key.\(UUID().uuidString)",
        accessGroup: group)
}

#if os(macOS)
@Test func testMissingEntitlementFallsBackToTheLoginKeychain() throws {
    let keychain = FakeKeychain(missingEntitlement: true)
    let store = KeychainKeyStore(keychain: keychain)
    let keys = manager(store)
    let info = try keys.ensureKey(scope: scope, purpose: .dpop)
    #expect(store.location == .login)
    #expect(store.downgraded)
    #expect(info.hardwareBound == false)
    // The item sits in the login keychain, without the shared group the process may not use.
    let item = try #require(keychain.items.values.first { ($0[kSecAttrAccount as String] as? String)?.hasSuffix(".dpop") == true })
    #expect(item[kSecUseDataProtectionKeychain as String] == nil)
    #expect(item[kSecAttrAccessGroup as String] == nil)
    // A second start reads the same key back, with the policy it was created with.
    #expect(try keys.ensureKey(scope: scope, purpose: .dpop) == info)
    let attributes = try #require(try keys.storedAttributes(scope: scope, purpose: .dpop))
    #expect(attributes.policy.accessibility == .afterFirstUnlockThisDeviceOnly)
    #expect(attributes.accessGroup == nil)
    #expect(attributes.synchronizable == false)
    let data = Data("proof".utf8)
    let raw = try keys.sign(scope: scope, purpose: .dpop, data: data)
    let point = Data([0x04]) + info.x + info.y
    #expect(try P256.Signing.PublicKey(x963Representation: point).isValidSignature(.init(rawRepresentation: raw), for: data))
    try keys.deleteKeys(scope: scope)
    #expect(try keys.keyInfo(scope: scope, purpose: .dpop) == nil)
}

@Test func testTheKeychainIsChosenOncePerStore() throws {
    let keychain = FakeKeychain(missingEntitlement: true)
    let store = KeychainKeyStore(keychain: keychain)
    let keys = manager(store)
    _ = try keys.ensureKey(scope: scope, purpose: .dpop)
    _ = try keys.ensureKey(scope: "https://other.example.com", purpose: .dpop)
    let probes = keychain.adds.filter { ($0[kSecUseDataProtectionKeychain as String] as? Bool) == true }
    #expect(probes.count == 1)
    // The probe leaves nothing behind.
    #expect(keychain.items.count == 2)
}

@Test func testSoftwareStepUpKeyIsNotAvailableInTheLoginKeychain() throws {
    // The login keychain cannot carry user presence on an item, so a Mac without a Secure
    // Enclave has no step-up key in such a build; the instance falls back to the other step-up
    // methods (08 6). With an enclave the confirmation sits on the enclave key instead.
    let keys = manager(KeychainKeyStore(keychain: FakeKeychain(missingEntitlement: true)))
    #expect(throws: DeviceKeyError.notAvailable) {
        try keys.ensureKey(scope: scope, purpose: .stepUp)
    }
}
#endif

@Test func testEntitledProcessKeepsTheDataProtectionKeychain() throws {
    let keychain = FakeKeychain(missingEntitlement: false)
    let store = KeychainKeyStore(keychain: keychain)
    let keys = manager(store)
    _ = try keys.ensureKey(scope: scope, purpose: .dpop)
    #expect(store.location == .dataProtection)
    #expect(store.downgraded == false)
    let item = try #require(keychain.items.values.first { ($0[kSecAttrAccount as String] as? String)?.hasSuffix(".dpop") == true })
    #expect(item[kSecUseDataProtectionKeychain as String] as? Bool == true)
    #expect(item[kSecAttrAccessGroup as String] as? String == group)
    let attributes = try #require(try keys.storedAttributes(scope: scope, purpose: .dpop))
    #expect(attributes.accessGroup == group)
    #expect(attributes.policy.accessibility == .afterFirstUnlockThisDeviceOnly)
    #expect(keychain.items.count == 1)
}

/// A store whose keys are hardware-bound but which reports the downgrade of its keychain.
private final class DowngradedStore: KeyStore {
    let inner = InMemoryKeyStore()
    var downgraded: Bool { true }
    func add(_ item: KeyItem) throws { try inner.add(item) }
    func attributes(service: String, account: String) throws -> KeyItemAttributes? {
        try inner.attributes(service: service, account: account)
    }
    func secret(service: String, account: String) throws -> Data? { try inner.secret(service: service, account: account) }
    func delete(service: String, account: String) throws { try inner.delete(service: service, account: account) }
}

@Test func testDowngradedStoreReportsKeysAsNotHardwareBound() throws {
    let store = DowngradedStore()
    let prefix = "test.device_key.\(UUID().uuidString)"
    let keys = DeviceKeyManager(
        store: store, capabilities: PlatformCapabilities(enclaveAvailable: false, biometryEnrolled: false),
        labelPrefix: prefix)
    let account = DeviceKeyManager.account(scope: scope, purpose: .dpop)
    let point = P256.Signing.PrivateKey().publicKey.x963Representation
    try store.add(KeyItem(
        service: prefix, account: account, label: "\(prefix).\(account)", secret: Data([1]),
        attributes: KeyItemAttributes(
            publicKey: point, hardwareBound: true,
            policy: KeyPolicy.policy(for: .dpop, capabilities: PlatformCapabilities(enclaveAvailable: true, biometryEnrolled: false)),
            accessGroup: nil, synchronizable: false)))
    #expect(try keys.keyInfo(scope: scope, purpose: .dpop)?.hardwareBound == false)
    #expect(try keys.ensureKey(scope: scope, purpose: .dpop).hardwareBound == false)
    // The same item in a store without a downgrade is hardware-bound.
    #expect(InMemoryKeyStore().downgraded == false)
}

/// Whether this process may write to the login keychain without the keychain entitlements and
/// lacks the Data Protection keychain: true for `swift test` on an unlocked Mac.
private func onlyLoginKeychainUsable() -> Bool {
    #if os(macOS)
    let base: [String: Any] = [
        kSecClass as String: kSecClassGenericPassword,
        kSecAttrService as String: "test.device_key.probe.\(UUID().uuidString)",
        kSecAttrAccount as String: "probe",
    ]
    var dataProtection = base
    dataProtection[kSecUseDataProtectionKeychain as String] = true
    dataProtection[kSecValueData as String] = Data([0])
    let entitled = SecItemAdd(dataProtection as CFDictionary, nil)
    if entitled == errSecSuccess {
        dataProtection[kSecValueData as String] = nil
        SecItemDelete(dataProtection as CFDictionary)
        return false
    }
    var login = base
    login[kSecValueData as String] = Data([0])
    let status = SecItemAdd(login as CFDictionary, nil)
    SecItemDelete(base as CFDictionary)
    return entitled == errSecMissingEntitlement && status == errSecSuccess
    #else
    return false
    #endif
}

/// The failure of brief FIX-macos-keychain on the real keychain: a process without the
/// entitlements (as an app with an ad-hoc signature) creates, reads and signs with its key.
@Test(.enabled(if: onlyLoginKeychainUsable(), "the process has the keychain entitlements or no unlocked login keychain"))
func testUnentitledProcessUsesTheLoginKeychain() throws {
    let store = KeychainKeyStore()
    let keys = manager(store, enclave: SecureEnclave.isAvailable)
    defer { try? keys.deleteKeys(scope: scope) }
    let info = try keys.ensureKey(scope: scope, purpose: .dpop)
    #expect(store.location == .login)
    #expect(info.hardwareBound == false)
    #expect(try keys.ensureKey(scope: scope, purpose: .dpop) == info)
    let data = Data("proof".utf8)
    let raw = try keys.sign(scope: scope, purpose: .dpop, data: data)
    let point = Data([0x04]) + info.x + info.y
    #expect(try P256.Signing.PublicKey(x963Representation: point).isValidSignature(.init(rawRepresentation: raw), for: data))
    try keys.deleteKeys(scope: scope)
    #expect(try keys.keyInfo(scope: scope, purpose: .dpop) == nil)
}

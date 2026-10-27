// SPDX-License-Identifier: Apache-2.0

import Foundation
import LocalAuthentication
import Security

/// The calls of the Security framework the store makes; the system's in the app, a fake in tests.
public protocol KeychainAPI: AnyObject {
    /// `SecItemAdd` without a result.
    func add(_ query: [String: Any]) -> OSStatus
    /// `SecItemCopyMatching`: the status and the result.
    func copyMatching(_ query: [String: Any]) -> (OSStatus, CFTypeRef?)
    /// `SecItemDelete`.
    func delete(_ query: [String: Any]) -> OSStatus
}

/// The keychain of the process.
public final class SystemKeychain: KeychainAPI {
    /// Creates the system keychain.
    public init() {}

    /// See `KeychainAPI.add(_:)`.
    public func add(_ query: [String: Any]) -> OSStatus {
        SecItemAdd(query as CFDictionary, nil)
    }

    /// See `KeychainAPI.copyMatching(_:)`.
    public func copyMatching(_ query: [String: Any]) -> (OSStatus, CFTypeRef?) {
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        return (status, result)
    }

    /// See `KeychainAPI.delete(_:)`.
    public func delete(_ query: [String: Any]) -> OSStatus {
        SecItemDelete(query as CFDictionary)
    }
}

/// The keychain behind the device keys: generic password items in the Data Protection keychain
/// (`kSecUseDataProtectionKeychain`, also on macOS), never synchronizable, always `ThisDeviceOnly`.
///
/// The secret of an item is the enclave-wrapped blob of a Secure Enclave key (useless on any other
/// device) or, without an enclave, the software private key. The public key, the storage kind and
/// the access flags sit in `kSecAttrGeneric`, so they are readable without asking the user.
///
/// A macOS process without the keychain entitlements (an app with an ad-hoc signature, no team
/// and no provisioning profile) cannot use the Data Protection keychain: writes fail with
/// `errSecMissingEntitlement` and reads find nothing. The store tries one write before its first
/// call and, on that answer, keeps every item in the login keychain instead: without an access
/// group, under the app's access list, and `downgraded`, so the keys report
/// `hardwareBound = false` (docs/decisions.d/FIX-macos-keychain.md). A signed app never takes
/// this path; iOS has only the Data Protection keychain.
public final class KeychainKeyStore: KeyStore {
    /// Where a store keeps its items.
    public enum Location: Equatable, Sendable {
        /// The Data Protection keychain, with the access groups of the entitlements.
        case dataProtection
        /// The file-based login keychain of macOS, without access groups.
        case login
    }

    /// Format of `kSecAttrGeneric`: version, hardware flag, key flags, item flags, X9.63 key.
    private static let metaVersion: UInt8 = 1
    /// The same plus one byte of accessibility at the end: the login keychain keeps no
    /// accessibility class to read back.
    private static let loginMetaVersion: UInt8 = 2
    /// The service of the item that tries the Data Protection keychain; it never holds a key.
    static let probeService = "device_key.keychain_probe"

    private let keychain: KeychainAPI
    private let lock = NSLock()
    private var chosen: Location?

    /// Creates a store over the keychain of the process.
    public convenience init() {
        self.init(keychain: SystemKeychain())
    }

    /// Creates a store over `keychain`; tests pass a fake.
    ///
    /// - Parameter keychain: the Security calls.
    public init(keychain: KeychainAPI) {
        self.keychain = keychain
    }

    /// The keychain this store uses, chosen at the first call and then kept.
    public var location: Location {
        lock.lock()
        defer { lock.unlock() }
        if let chosen {
            return chosen
        }
        let location = Self.choose(keychain)
        chosen = location
        return location
    }

    /// See `KeyStore.downgraded`: true in the login keychain.
    public var downgraded: Bool { location == .login }

    /// Tries one write to the Data Protection keychain: `errSecMissingEntitlement` means the
    /// process lacks the entitlements, every other answer that it has them.
    static func choose(_ keychain: KeychainAPI) -> Location {
        #if os(macOS)
        let probe: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: probeService,
            kSecAttrAccount as String: "probe",
            kSecUseDataProtectionKeychain as String: true,
        ]
        var add = probe
        add[kSecValueData as String] = Data()
        add[kSecAttrSynchronizable as String] = kCFBooleanFalse
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        let status = keychain.add(add)
        if status == errSecMissingEntitlement {
            return .login
        }
        if status == errSecSuccess {
            _ = keychain.delete(probe)
        }
        #endif
        return .dataProtection
    }

    /// See `KeyStore.add(_:)`.
    public func add(_ item: KeyItem) throws {
        let attributes = item.attributes
        let place = location
        var query = base(service: item.service, account: item.account)
        query[kSecAttrLabel as String] = item.label
        query[kSecValueData as String] = item.secret
        query[kSecAttrGeneric as String] = Self.meta(attributes, location: place)
        query[kSecAttrSynchronizable as String] = kCFBooleanFalse
        if let group = attributes.accessGroup, place == .dataProtection {
            query[kSecAttrAccessGroup as String] = group
        }
        if attributes.policy.itemFlags.isEmpty {
            query[kSecAttrAccessible as String] = attributes.policy.accessibility.secValue
        } else {
            var error: Unmanaged<CFError>?
            guard let control = SecAccessControlCreateWithFlags(
                nil, attributes.policy.accessibility.secValue, attributes.policy.itemFlags.secFlags, &error)
            else {
                throw DeviceKeyError.internal
            }
            query[kSecAttrAccessControl as String] = control
        }
        let status = keychain.add(query)
        if status == errSecDuplicateItem {
            throw KeyStoreError.duplicate
        }
        try Self.check(status)
    }

    /// See `KeyStore.attributes(service:account:)`.
    public func attributes(service: String, account: String) throws -> KeyItemAttributes? {
        var query = base(service: service, account: account)
        query[kSecReturnAttributes as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        // Attributes need no confirmation; the context makes sure no prompt can appear.
        #if !targetEnvironment(simulator)
        // The iOS simulator refuses an authentication context (LocalAuthentication -1020); its
        // items carry no access control that could prompt (`KeyPolicy.policy`, M5-06).
        let context = LAContext()
        context.interactionNotAllowed = true
        query[kSecUseAuthenticationContext as String] = context
        #endif
        let (status, result) = keychain.copyMatching(query)
        if status == errSecItemNotFound {
            return nil
        }
        try Self.check(status)
        guard let found = result as? [String: Any],
              let meta = found[kSecAttrGeneric as String] as? Data,
              let accessibility = Self.accessibility(meta: meta, found: found)
        else {
            throw DeviceKeyError.internal
        }
        let synchronizable = (found[kSecAttrSynchronizable as String] as? NSNumber)?.boolValue ?? false
        return KeyItemAttributes(
            publicKey: meta.subdata(in: 4..<69),
            hardwareBound: meta[1] == 1,
            policy: KeyPolicy(
                accessibility: accessibility,
                keyFlags: Set(bits: meta[2]),
                itemFlags: Set(bits: meta[3])),
            accessGroup: found[kSecAttrAccessGroup as String] as? String,
            synchronizable: synchronizable)
    }

    /// See `KeyStore.secret(service:account:)`.
    public func secret(service: String, account: String) throws -> Data? {
        var query = base(service: service, account: account)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        let (status, result) = keychain.copyMatching(query)
        if status == errSecItemNotFound {
            return nil
        }
        try Self.check(status)
        guard let data = result as? Data else { throw DeviceKeyError.internal }
        return data
    }

    /// See `KeyStore.delete(service:account:)`.
    public func delete(service: String, account: String) throws {
        let status = keychain.delete(base(service: service, account: account))
        if status == errSecItemNotFound {
            return
        }
        try Self.check(status)
    }

    private func base(service: String, account: String) -> [String: Any] {
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        if location == .dataProtection {
            query[kSecUseDataProtectionKeychain as String] = true
        }
        return query
    }

    private static func meta(_ attributes: KeyItemAttributes, location: Location) -> Data {
        var data = Data([
            location == .login ? loginMetaVersion : metaVersion,
            attributes.hardwareBound ? 1 : 0,
            attributes.policy.keyFlags.bits,
            attributes.policy.itemFlags.bits,
        ])
        data.append(attributes.publicKey)
        if location == .login {
            data.append(attributes.policy.accessibility == .afterFirstUnlockThisDeviceOnly ? 0 : 1)
        }
        return data
    }

    /// The accessibility of an item: from the keychain for the first format, from the last byte
    /// of the attributes for the login keychain's format; nil for anything else.
    private static func accessibility(meta: Data, found: [String: Any]) -> Accessibility? {
        if meta.count == 69, meta[0] == metaVersion {
            return (found[kSecAttrAccessible as String] as? String).flatMap { Accessibility(secValue: $0) }
        }
        if meta.count == 70, meta[0] == loginMetaVersion, meta[69] <= 1 {
            return meta[69] == 0 ? .afterFirstUnlockThisDeviceOnly : .whenPasscodeSetThisDeviceOnly
        }
        return nil
    }

    /// Throws the status as an `NSOSStatusErrorDomain` error, which `DeviceKeyError.classify` maps.
    private static func check(_ status: OSStatus) throws {
        guard status == errSecSuccess else {
            throw NSError(domain: NSOSStatusErrorDomain, code: Int(status))
        }
    }
}

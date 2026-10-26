// SPDX-License-Identifier: Apache-2.0

import Foundation
import LocalAuthentication
import Security

/// The keychain behind the device keys: generic password items in the Data Protection keychain
/// (`kSecUseDataProtectionKeychain`, also on macOS), never synchronizable, always `ThisDeviceOnly`.
///
/// The secret of an item is the enclave-wrapped blob of a Secure Enclave key (useless on any other
/// device) or, without an enclave, the software private key. The public key, the storage kind and
/// the access flags sit in `kSecAttrGeneric`, so they are readable without asking the user.
public final class KeychainKeyStore: KeyStore {
    /// Format of `kSecAttrGeneric`: version, hardware flag, key flags, item flags, X9.63 key.
    private static let metaVersion: UInt8 = 1

    /// Creates a store over the keychain of the process.
    public init() {}

    /// See `KeyStore.add(_:)`.
    public func add(_ item: KeyItem) throws {
        let attributes = item.attributes
        var query = Self.base(service: item.service, account: item.account)
        query[kSecAttrLabel as String] = item.label
        query[kSecValueData as String] = item.secret
        query[kSecAttrGeneric as String] = Self.meta(attributes)
        query[kSecAttrSynchronizable as String] = kCFBooleanFalse
        if let group = attributes.accessGroup {
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
        let status = SecItemAdd(query as CFDictionary, nil)
        if status == errSecDuplicateItem {
            throw KeyStoreError.duplicate
        }
        try Self.check(status)
    }

    /// See `KeyStore.attributes(service:account:)`.
    public func attributes(service: String, account: String) throws -> KeyItemAttributes? {
        var query = Self.base(service: service, account: account)
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
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound {
            return nil
        }
        try Self.check(status)
        guard let found = result as? [String: Any],
              let meta = found[kSecAttrGeneric as String] as? Data,
              meta.count == 69, meta[0] == Self.metaVersion,
              let accessible = found[kSecAttrAccessible as String] as? String,
              let accessibility = Accessibility(secValue: accessible)
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
        var query = Self.base(service: service, account: account)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound {
            return nil
        }
        try Self.check(status)
        guard let data = result as? Data else { throw DeviceKeyError.internal }
        return data
    }

    /// See `KeyStore.delete(service:account:)`.
    public func delete(service: String, account: String) throws {
        let status = SecItemDelete(Self.base(service: service, account: account) as CFDictionary)
        if status == errSecItemNotFound {
            return
        }
        try Self.check(status)
    }

    private static func base(service: String, account: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecUseDataProtectionKeychain as String: true,
        ]
    }

    private static func meta(_ attributes: KeyItemAttributes) -> Data {
        var data = Data([
            metaVersion,
            attributes.hardwareBound ? 1 : 0,
            attributes.policy.keyFlags.bits,
            attributes.policy.itemFlags.bits,
        ])
        data.append(attributes.publicKey)
        return data
    }

    /// Throws the status as an `NSOSStatusErrorDomain` error, which `DeviceKeyError.classify` maps.
    private static func check(_ status: OSStatus) throws {
        guard status == errSecSuccess else {
            throw NSError(domain: NSOSStatusErrorDomain, code: Int(status))
        }
    }
}

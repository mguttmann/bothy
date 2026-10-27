// SPDX-License-Identifier: Apache-2.0

import CryptoKit
import Foundation
import Security

/// Creates, uses and deletes the two device keys of one app for each server
/// (docs/spec/08-identity.md 6.1, 15 9; brief M4-10, Design note 3).
///
/// The plugin `device_key` and the app extensions of M5 use this type with the same label prefix
/// and access group, so the Notification Service Extension signs with the app's DPoP key.
///
/// Example:
///
///     let keys = DeviceKeyManager(store: KeychainKeyStore(), accessGroup: "TEAMID.com.example.shared")
///     let info = try keys.ensureKey(scope: "https://chat.example.com", purpose: .dpop)
///     let signature = try keys.sign(scope: "https://chat.example.com", purpose: .dpop, data: proof)
///
/// There is no operation that replaces the key of a registration (08 5.4, R-ID-11):
/// `deleteKeys(scope:)` removes both keys, and the app then registers as a new device.
public final class DeviceKeyManager {
    /// The label prefix the app and its extensions use.
    public static let defaultLabelPrefix = "device_key"

    private let store: KeyStore
    private let capabilities: PlatformCapabilities
    private let labelPrefix: String
    private let accessGroup: String?

    /// Creates a manager.
    ///
    /// - Parameters:
    ///   - store: where the keys are kept; `KeychainKeyStore` in the app.
    ///   - capabilities: what the device offers; tests force the enclave or the fallback path.
    ///   - labelPrefix: the service of every item; tests use a unique prefix of their own.
    ///   - accessGroup: the keychain access group the DPoP key shares with the extensions, from
    ///     the app's entitlements; nil keeps it in the app's default group.
    public init(
        store: KeyStore,
        capabilities: PlatformCapabilities = .current,
        labelPrefix: String = defaultLabelPrefix,
        accessGroup: String? = nil
    ) {
        self.store = store
        self.capabilities = capabilities
        self.labelPrefix = labelPrefix
        self.accessGroup = accessGroup
    }

    /// The account of an item: the SHA-256 of the scope in hex and the purpose.
    ///
    /// Hashing keeps the server's address out of the keychain and gives every scope a fixed length.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server, for example the server's origin.
    ///   - purpose: the key's purpose.
    /// - Returns: `<sha256 hex>.<purpose>`.
    public static func account(scope: String, purpose: KeyPurpose) -> String {
        let digest = SHA256.hash(data: Data(scope.utf8)).map { String(format: "%02x", $0) }.joined()
        return "\(digest).\(purpose.rawValue)"
    }

    /// Returns the key of `purpose` for `scope`, creating it when it is missing.
    ///
    /// Creating asks the user for nothing; only signing with the step-up key does.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server.
    ///   - purpose: the key's purpose.
    /// - Returns: the public key and how it is stored.
    /// - Throws: `DeviceKeyError`.
    public func ensureKey(scope: String, purpose: KeyPurpose) throws -> DeviceKeyInfo {
        try mapped {
            if let attributes = try store.attributes(service: labelPrefix, account: Self.account(scope: scope, purpose: purpose)) {
                return try info(attributes, purpose: purpose)
            }
            let item = try makeItem(scope: scope, purpose: purpose)
            do {
                try store.add(item)
            } catch KeyStoreError.duplicate {
                // Another caller (an extension) created it first: use that key.
                guard let attributes = try store.attributes(service: labelPrefix, account: item.account) else {
                    throw DeviceKeyError.internal
                }
                return try info(attributes, purpose: purpose)
            }
            return try info(item.attributes, purpose: purpose)
        }
    }

    /// Returns the key of `purpose` for `scope`, or nil when there is none; never creates one.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server.
    ///   - purpose: the key's purpose.
    /// - Returns: the public key and how it is stored, or nil.
    /// - Throws: `DeviceKeyError`.
    public func keyInfo(scope: String, purpose: KeyPurpose) throws -> DeviceKeyInfo? {
        try mapped {
            try storedAttributes(scope: scope, purpose: purpose).map { try info($0, purpose: purpose) }
        }
    }

    /// Returns the stored attributes of a key, for checks of the storage policy.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server.
    ///   - purpose: the key's purpose.
    /// - Returns: the attributes, or nil when there is no key.
    /// - Throws: `DeviceKeyError`.
    public func storedAttributes(scope: String, purpose: KeyPurpose) throws -> KeyItemAttributes? {
        try mapped { try store.attributes(service: labelPrefix, account: Self.account(scope: scope, purpose: purpose)) }
    }

    /// Signs `data` with ES256 (SHA-256 inside) and returns the JOSE form `R || S`.
    ///
    /// For the step-up key the system asks for biometrics or the device password; the call
    /// blocks until the user answered, so callers run it off the main thread.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server.
    ///   - purpose: the key's purpose.
    ///   - data: the bytes to sign, for example the JWS signing input.
    /// - Returns: 64 bytes, `R || S` (RFC 7518 3.4).
    /// - Throws: `DeviceKeyError.notAvailable` without a key, `.userCancelled`,
    ///   `.keyInvalidated` when an enclave key can no longer be used, `.internal` otherwise.
    public func sign(scope: String, purpose: KeyPurpose, data: Data) throws -> Data {
        try mapped {
            let account = Self.account(scope: scope, purpose: purpose)
            guard let attributes = try store.attributes(service: labelPrefix, account: account),
                  let secret = try store.secret(service: labelPrefix, account: account)
            else {
                throw DeviceKeyError.notAvailable
            }
            if attributes.hardwareBound {
                let enclaveSigner: SecureEnclave.P256.Signing.PrivateKey
                do {
                    enclaveSigner = try SecureEnclave.P256.Signing.PrivateKey(dataRepresentation: secret)
                } catch {
                    // The enclave no longer accepts the blob: the key is gone for good (08 5.4).
                    // Usually the rejection comes only from signing (`classify`, CryptoTokenKit).
                    throw DeviceKeyError.keyInvalidated
                }
                // A blob that no longer yields the registered key is as unusable as a rejected one.
                guard enclaveSigner.publicKey.x963Representation == attributes.publicKey else {
                    throw DeviceKeyError.keyInvalidated
                }
                return try enclaveSigner.signature(for: data).rawRepresentation
            }
            let softwareSigner: P256.Signing.PrivateKey
            do {
                softwareSigner = try P256.Signing.PrivateKey(rawRepresentation: secret)
            } catch {
                throw DeviceKeyError.internal
            }
            guard softwareSigner.publicKey.x963Representation == attributes.publicKey else { throw DeviceKeyError.internal }
            return try softwareSigner.signature(for: data).rawRepresentation
        }
    }

    /// Returns an attestation of the key; Apple platforms return none in this version.
    ///
    /// App Attest is planned with `auth.require_key_attestation` (08 5.1, brief M4-10 "Out of
    /// scope"), so the field stays empty.
    ///
    /// - Parameters:
    ///   - scope: separates the keys per server.
    ///   - purpose: the key's purpose.
    ///   - challenge: the server's challenge.
    /// - Returns: nil.
    /// - Throws: `DeviceKeyError.notAvailable` when the key does not exist.
    public func attestation(scope: String, purpose: KeyPurpose, challenge: Data) throws -> Data? {
        try mapped {
            guard try store.attributes(service: labelPrefix, account: Self.account(scope: scope, purpose: purpose)) != nil else {
                throw DeviceKeyError.notAvailable
            }
            return nil
        }
    }

    /// Deletes both keys of `scope`; missing keys are not an error.
    ///
    /// - Parameter scope: separates the keys per server.
    /// - Throws: `DeviceKeyError`.
    public func deleteKeys(scope: String) throws {
        try mapped {
            for purpose in KeyPurpose.allCases {
                try store.delete(service: labelPrefix, account: Self.account(scope: scope, purpose: purpose))
            }
        }
    }

    private func makeItem(scope: String, purpose: KeyPurpose) throws -> KeyItem {
        let policy = KeyPolicy.policy(for: purpose, capabilities: capabilities)
        let secret: Data
        let publicKey: Data
        if capabilities.enclaveAvailable {
            var error: Unmanaged<CFError>?
            guard let control = SecAccessControlCreateWithFlags(
                nil, policy.accessibility.secValue, policy.keyFlags.secFlags, &error)
            else {
                throw DeviceKeyError.internal
            }
            let key = try SecureEnclave.P256.Signing.PrivateKey(compactRepresentable: false, accessControl: control)
            secret = key.dataRepresentation
            publicKey = key.publicKey.x963Representation
        } else {
            let key = P256.Signing.PrivateKey(compactRepresentable: false)
            secret = key.rawRepresentation
            publicKey = key.publicKey.x963Representation
        }
        let account = Self.account(scope: scope, purpose: purpose)
        return KeyItem(
            service: labelPrefix,
            account: account,
            label: "\(labelPrefix).\(account)",
            secret: secret,
            attributes: KeyItemAttributes(
                publicKey: publicKey,
                hardwareBound: capabilities.enclaveAvailable,
                policy: policy,
                accessGroup: purpose == .dpop ? accessGroup : nil,
                synchronizable: false))
    }

    /// The public description of a stored key; a key of a downgraded store is reported as not
    /// hardware-bound, as a software key is (README "Storage per platform").
    private func info(_ attributes: KeyItemAttributes, purpose: KeyPurpose) throws -> DeviceKeyInfo {
        let point = attributes.publicKey
        guard point.count == 65, point.first == 0x04 else { throw DeviceKeyError.internal }
        return DeviceKeyInfo(
            x: point.subdata(in: 1..<33),
            y: point.subdata(in: 33..<65),
            hardwareBound: attributes.hardwareBound && !store.downgraded,
            requiresUserPresence: purpose == .stepUp)
    }

    /// Runs `body` and turns every error into a `DeviceKeyError`, so no platform message leaves.
    private func mapped<T>(_ body: () throws -> T) throws -> T {
        do {
            return try body()
        } catch {
            throw DeviceKeyError.classify(error)
        }
    }
}

extension Accessibility {
    /// The Security constant of the accessibility class.
    var secValue: CFString {
        switch self {
        case .afterFirstUnlockThisDeviceOnly: return kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        case .whenPasscodeSetThisDeviceOnly: return kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly
        }
    }

    /// The accessibility class of a Security constant read back from the keychain.
    init?(secValue: String) {
        if secValue == kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly as String {
            self = .afterFirstUnlockThisDeviceOnly
        } else if secValue == kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly as String {
            self = .whenPasscodeSetThisDeviceOnly
        } else {
            return nil
        }
    }
}

extension Set where Element == AccessFlag {
    /// The Security flags of the set.
    var secFlags: SecAccessControlCreateFlags {
        var flags: SecAccessControlCreateFlags = []
        if contains(.privateKeyUsage) { flags.insert(.privateKeyUsage) }
        if contains(.biometryCurrentSet) { flags.insert(.biometryCurrentSet) }
        if contains(.userPresence) { flags.insert(.userPresence) }
        return flags
    }

    /// One byte for the item attributes: bit 0 privateKeyUsage, bit 1 biometryCurrentSet, bit 2 userPresence.
    var bits: UInt8 {
        (contains(.privateKeyUsage) ? 1 : 0) | (contains(.biometryCurrentSet) ? 2 : 0) | (contains(.userPresence) ? 4 : 0)
    }

    /// The set of a byte written by `bits`.
    init(bits: UInt8) {
        self = []
        if bits & 1 != 0 { insert(.privateKeyUsage) }
        if bits & 2 != 0 { insert(.biometryCurrentSet) }
        if bits & 4 != 0 { insert(.userPresence) }
    }
}

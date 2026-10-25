// SPDX-License-Identifier: Apache-2.0

import CryptoKit
import Foundation
import Security

/// Storage of small secrets by service and account; the keychain in the app and its extensions,
/// memory in the tests.
public protocol SecretStore: AnyObject {
    /// Stores `value`, replacing an item of the same service and account.
    ///
    /// - Parameters:
    ///   - value: the bytes.
    ///   - service: the service of the item.
    ///   - account: the account of the item.
    /// - Throws: `SecretStoreError` on a platform failure.
    func write(_ value: Data, service: String, account: String) throws

    /// Reads an item.
    ///
    /// - Parameters:
    ///   - service: the service of the item.
    ///   - account: the account of the item.
    /// - Returns: the bytes, or nil when there is no item.
    /// - Throws: `SecretStoreError` on a platform failure.
    func read(service: String, account: String) throws -> Data?

    /// Deletes an item; a missing item is not an error.
    ///
    /// - Parameters:
    ///   - service: the service of the item.
    ///   - account: the account of the item.
    /// - Throws: `SecretStoreError` on a platform failure.
    func delete(service: String, account: String) throws
}

/// A failure of the keychain, with the `OSStatus` only (never a value).
public struct SecretStoreError: Error, Equatable {
    /// The status the Security framework returned.
    public let status: OSStatus
}

/// The keychain behind `SharedKeychain`: generic password items in the Data Protection keychain
/// (`kSecUseDataProtectionKeychain`, also on macOS), in the shared access group,
/// `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` and never synchronizable
/// (docs/spec/15-client-architecture.md 9; brief M5-04 Design note 1).
///
/// `AfterFirstUnlock` because the extension runs while the device is locked; `ThisDeviceOnly`
/// keeps the items out of backups and the iCloud keychain.
public final class KeychainSecretStore: SecretStore {
    private let accessGroup: String

    /// Creates the store for `accessGroup`.
    ///
    /// - Parameter accessGroup: the shared keychain access group of `ClientKitConfiguration`.
    public init(accessGroup: String) {
        self.accessGroup = accessGroup
    }

    /// The attributes of a new item; separate so the tests can check the protection without a
    /// keychain entitlement.
    ///
    /// - Parameters:
    ///   - value: the bytes.
    ///   - service: the service.
    ///   - account: the account.
    ///   - accessGroup: the access group.
    /// - Returns: the query of `SecItemAdd`.
    static func addAttributes(value: Data, service: String, account: String, accessGroup: String) -> [String: Any] {
        var query = base(service: service, account: account, accessGroup: accessGroup)
        query[kSecValueData as String] = value
        query[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        query[kSecAttrSynchronizable as String] = kCFBooleanFalse
        return query
    }

    /// See `SecretStore.write(_:service:account:)`.
    public func write(_ value: Data, service: String, account: String) throws {
        try delete(service: service, account: account)
        let status = SecItemAdd(
            Self.addAttributes(value: value, service: service, account: account, accessGroup: accessGroup) as CFDictionary,
            nil)
        guard status == errSecSuccess else { throw SecretStoreError(status: status) }
    }

    /// See `SecretStore.read(service:account:)`.
    public func read(service: String, account: String) throws -> Data? {
        var query = Self.base(service: service, account: account, accessGroup: accessGroup)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = result as? Data else { throw SecretStoreError(status: status) }
        return data
    }

    /// See `SecretStore.delete(service:account:)`.
    public func delete(service: String, account: String) throws {
        let status = SecItemDelete(Self.base(service: service, account: account, accessGroup: accessGroup) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw SecretStoreError(status: status) }
    }

    private static func base(service: String, account: String, accessGroup: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecAttrAccessGroup as String: accessGroup,
            kSecUseDataProtectionKeychain as String: true,
        ]
    }
}

/// A `SecretStore` in memory, for tests.
public final class MemorySecretStore: SecretStore {
    /// The items by `service` and `account`.
    public private(set) var items: [String: Data] = [:]
    private let lock = NSLock()

    /// Creates an empty store.
    public init() {}

    /// See `SecretStore.write(_:service:account:)`.
    public func write(_ value: Data, service: String, account: String) throws {
        lock.lock()
        defer { lock.unlock() }
        items["\(service)\u{0}\(account)"] = value
    }

    /// See `SecretStore.read(service:account:)`.
    public func read(service: String, account: String) throws -> Data? {
        lock.lock()
        defer { lock.unlock() }
        return items["\(service)\u{0}\(account)"]
    }

    /// See `SecretStore.delete(service:account:)`.
    public func delete(service: String, account: String) throws {
        lock.lock()
        defer { lock.unlock() }
        items["\(service)\u{0}\(account)"] = nil
    }
}

/// What the app stores for the extension after `registerPush`: the preview token, the
/// registration ID and the server address (docs/spec/13-push-notifications.md 6.4).
///
/// `description` names none of the values, so a log line never carries the token.
public struct PushCredentials: Equatable, CustomStringConvertible, CustomDebugStringConvertible {
    /// The preview token `bothy_pvw_...`; the only credential of the extension.
    public let previewToken: String
    /// The ID of the push registration, for `unregisterPush`.
    public let registrationID: String
    /// The server address (origin and base path) of the instance.
    public let serverURL: URL

    /// Creates the credentials.
    ///
    /// - Parameters:
    ///   - previewToken: the preview token.
    ///   - registrationID: the registration ID.
    ///   - serverURL: the server address.
    public init(previewToken: String, registrationID: String, serverURL: URL) {
        self.previewToken = previewToken
        self.registrationID = registrationID
        self.serverURL = serverURL
    }

    /// A fixed text without any value.
    public var description: String { "PushCredentials(<redacted>)" }

    /// The same as `description`.
    public var debugDescription: String { description }
}

/// The push items of the shared keychain, per instance (brief M5-04 Design note 1).
///
/// Three generic password items per instance under the service `clientkit`, with the account
/// `<sha256 hex of the instance ID>.<name>` for `preview_token`, `registration_id` and
/// `server_url`: the same scope hash as the device keys (`DeviceKeyManager.account`), so the
/// instance ID itself is not stored in clear text. The app writes and deletes them; the
/// extension only reads.
///
/// Example:
///
///     let keychain = SharedKeychain(store: KeychainSecretStore(accessGroup: config.keychainGroup))
///     try keychain.save(credentials, instanceID: instance)
///     let stored = try keychain.load(instanceID: instance)
public final class SharedKeychain {
    /// The service of every item.
    public static let service = "clientkit"
    static let names = ["preview_token", "registration_id", "server_url"]

    private let store: SecretStore

    /// Creates the keychain view over `store`.
    ///
    /// - Parameter store: the keychain or a memory store.
    public init(store: SecretStore) {
        self.store = store
    }

    /// The account of item `name` of `instanceID`.
    ///
    /// - Parameters:
    ///   - instanceID: the instance.
    ///   - name: `preview_token`, `registration_id` or `server_url`.
    /// - Returns: `<sha256 hex>.<name>`.
    public static func account(instanceID: String, name: String) -> String {
        let digest = SHA256.hash(data: Data(instanceID.utf8)).map { String(format: "%02x", $0) }.joined()
        return "\(digest).\(name)"
    }

    /// Stores the credentials of `instanceID`, replacing earlier ones.
    ///
    /// - Parameters:
    ///   - credentials: what `registerPush` returned and the server address.
    ///   - instanceID: the instance.
    /// - Throws: `SecretStoreError`.
    public func save(_ credentials: PushCredentials, instanceID: String) throws {
        let values = [credentials.previewToken, credentials.registrationID, credentials.serverURL.absoluteString]
        for (name, value) in zip(Self.names, values) {
            try store.write(Data(value.utf8), service: Self.service, account: Self.account(instanceID: instanceID, name: name))
        }
    }

    /// Reads the credentials of `instanceID`.
    ///
    /// - Parameter instanceID: the instance.
    /// - Returns: the credentials, or nil when any item is missing or unreadable as text.
    /// - Throws: `SecretStoreError`.
    public func load(instanceID: String) throws -> PushCredentials? {
        var values: [String] = []
        for name in Self.names {
            guard let data = try store.read(service: Self.service, account: Self.account(instanceID: instanceID, name: name)),
                  let text = String(data: data, encoding: .utf8)
            else {
                return nil
            }
            values.append(text)
        }
        guard let url = URL(string: values[2]) else { return nil }
        return PushCredentials(previewToken: values[0], registrationID: values[1], serverURL: url)
    }

    /// Deletes the credentials of `instanceID`; missing items are not an error.
    ///
    /// - Parameter instanceID: the instance.
    /// - Throws: `SecretStoreError`.
    public func delete(instanceID: String) throws {
        for name in Self.names {
            try store.delete(service: Self.service, account: Self.account(instanceID: instanceID, name: name))
        }
    }
}

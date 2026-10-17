// SPDX-License-Identifier: Apache-2.0

import Foundation

/// What a store keeps next to the secret of a key; readable without asking the user.
public struct KeyItemAttributes: Equatable, Sendable {
    /// The public key in X9.63 form (`04 || x || y`, 65 bytes).
    public let publicKey: Data
    /// Whether the secret is an enclave-wrapped blob (true) or a software private key (false).
    public let hardwareBound: Bool
    /// The protection the key and its item were created with.
    public let policy: KeyPolicy
    /// The keychain access group; set only for the DPoP key shared with the extensions.
    public let accessGroup: String?
    /// Whether the item syncs through the iCloud keychain; always false.
    public let synchronizable: Bool

    /// Creates the attributes of an item.
    ///
    /// - Parameters:
    ///   - publicKey: the public key, X9.63 form.
    ///   - hardwareBound: whether the secret is an enclave blob.
    ///   - policy: the protection of key and item.
    ///   - accessGroup: the keychain access group or nil for the default one.
    ///   - synchronizable: whether the item syncs; the manager always writes false.
    public init(publicKey: Data, hardwareBound: Bool, policy: KeyPolicy, accessGroup: String?, synchronizable: Bool) {
        self.publicKey = publicKey
        self.hardwareBound = hardwareBound
        self.policy = policy
        self.accessGroup = accessGroup
        self.synchronizable = synchronizable
    }
}

/// One stored key: a generic password item named by service and account.
public struct KeyItem: Equatable, Sendable {
    /// The label prefix of the manager; groups the items of one app.
    public let service: String
    /// `<sha256(scope)>.<purpose>`: the server is hashed, not stored in clear text.
    public let account: String
    /// `<service>.<account>`, the label of brief M4-10 note 3.
    public let label: String
    /// The enclave blob or the software private key; reading it may ask the user.
    public let secret: Data
    /// What is readable without asking the user.
    public let attributes: KeyItemAttributes

    /// Creates an item.
    ///
    /// - Parameters:
    ///   - service: the label prefix.
    ///   - account: the hashed scope and the purpose.
    ///   - label: the full label.
    ///   - secret: the key blob.
    ///   - attributes: the readable attributes.
    public init(service: String, account: String, label: String, secret: Data, attributes: KeyItemAttributes) {
        self.service = service
        self.account = account
        self.label = label
        self.secret = secret
        self.attributes = attributes
    }

    /// The key of the item in a store: service and account.
    public var key: String { "\(service)\u{0}\(account)" }
}

/// Errors a store reports besides platform errors.
public enum KeyStoreError: Error, Equatable {
    /// An item with this service and account exists already.
    case duplicate
}

/// The storage of key items; the keychain in the app, memory in the unit tests (note 3).
public protocol KeyStore: AnyObject {
    /// Adds an item.
    ///
    /// - Parameter item: the new item.
    /// - Throws: `KeyStoreError.duplicate` when the item exists, or a platform error.
    func add(_ item: KeyItem) throws

    /// Reads the attributes of an item without asking the user.
    ///
    /// - Parameters:
    ///   - service: the label prefix.
    ///   - account: the hashed scope and the purpose.
    /// - Returns: the attributes, or nil when there is no such item.
    /// - Throws: a platform error.
    func attributes(service: String, account: String) throws -> KeyItemAttributes?

    /// Reads the secret of an item; for the software step-up key this asks the user.
    ///
    /// - Parameters:
    ///   - service: the label prefix.
    ///   - account: the hashed scope and the purpose.
    /// - Returns: the secret, or nil when there is no such item.
    /// - Throws: a platform error, for example the user cancelled.
    func secret(service: String, account: String) throws -> Data?

    /// Deletes an item; a missing item is not an error.
    ///
    /// - Parameters:
    ///   - service: the label prefix.
    ///   - account: the hashed scope and the purpose.
    /// - Throws: a platform error.
    func delete(service: String, account: String) throws
}

/// A store in memory, for tests; it keeps every attribute as written.
public final class InMemoryKeyStore: KeyStore {
    /// The items by `KeyItem.key`.
    public var items: [String: KeyItem] = [:]

    /// Creates an empty store.
    public init() {}

    /// See `KeyStore.add(_:)`.
    public func add(_ item: KeyItem) throws {
        guard items[item.key] == nil else { throw KeyStoreError.duplicate }
        items[item.key] = item
    }

    /// See `KeyStore.attributes(service:account:)`.
    public func attributes(service: String, account: String) throws -> KeyItemAttributes? {
        items["\(service)\u{0}\(account)"]?.attributes
    }

    /// See `KeyStore.secret(service:account:)`.
    public func secret(service: String, account: String) throws -> Data? {
        items["\(service)\u{0}\(account)"]?.secret
    }

    /// See `KeyStore.delete(service:account:)`.
    public func delete(service: String, account: String) throws {
        items["\(service)\u{0}\(account)"] = nil
    }
}

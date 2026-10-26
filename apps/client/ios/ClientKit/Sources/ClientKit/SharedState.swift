// SPDX-License-Identifier: Apache-2.0

import Foundation

/// A small key-value store: the `UserDefaults` suite of the App Group in the app and its
/// extensions, a dictionary in the tests.
public protocol KeyValueStore: AnyObject {
    /// The value of `key`, or nil.
    ///
    /// - Parameter key: the key.
    /// - Returns: the stored property list value.
    func value(forKey key: String) -> Any?

    /// Stores `value` under `key`; nil removes the key.
    ///
    /// - Parameters:
    ///   - value: a property list value or nil.
    ///   - key: the key.
    func set(_ value: Any?, forKey key: String)

    /// Every key the store holds; the tests check that nothing beyond the allowed keys is written.
    var keys: [String] { get }
}

/// `UserDefaults` as a `KeyValueStore`.
public final class DefaultsStore: KeyValueStore {
    private let defaults: UserDefaults

    /// Creates the store over `defaults`.
    ///
    /// - Parameter defaults: the suite of the App Group, `UserDefaults(suiteName:)`.
    public init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    /// See `KeyValueStore.value(forKey:)`.
    public func value(forKey key: String) -> Any? {
        defaults.object(forKey: key)
    }

    /// See `KeyValueStore.set(_:forKey:)`.
    public func set(_ value: Any?, forKey key: String) {
        if let value {
            defaults.set(value, forKey: key)
        } else {
            defaults.removeObject(forKey: key)
        }
    }

    /// The keys of this store's own prefix; the suite also reports the global domain's keys.
    public var keys: [String] {
        defaults.dictionaryRepresentation().keys.filter { $0.hasPrefix(SharedState.prefix) }
    }
}

/// A `KeyValueStore` in memory, for tests.
public final class MemoryStore: KeyValueStore {
    /// The values by key.
    public private(set) var values: [String: Any] = [:]
    private let lock = NSLock()

    /// Creates an empty store.
    public init() {}

    /// See `KeyValueStore.value(forKey:)`.
    public func value(forKey key: String) -> Any? {
        lock.lock()
        defer { lock.unlock() }
        return values[key]
    }

    /// See `KeyValueStore.set(_:forKey:)`.
    public func set(_ value: Any?, forKey key: String) {
        lock.lock()
        defer { lock.unlock() }
        values[key] = value
    }

    /// See `KeyValueStore.keys`.
    public var keys: [String] {
        lock.lock()
        defer { lock.unlock() }
        return Array(values.keys)
    }
}

/// The value of `notify.preview_mode` (docs/spec/13-push-notifications.md 3.3, 6.4).
public enum PreviewMode: String, CaseIterable, Sendable {
    /// Names and excerpt.
    case full
    /// Only the names.
    case names
    /// Nothing: the extension fetches nothing.
    case none
}

/// The values of the App Group the app writes and the extensions read (brief M5-04 Design
/// note 1; brief M5-06 Design note 7; docs/spec/15-client-architecture.md 10.1, 12).
///
/// Only these keys exist: the preview mode, the instance ID of the signed-in account, the
/// fetch timeout `client.push.fetch_timeout` (the extensions have no `AppConfig`, so the app
/// writes the value at start), the last `DPoP-Nonce` per server (`NonceCache`), the display
/// counters (`DisplayCounters`), and for the Share Extension the share settings
/// (`client.share.recent_targets`, `file_max_upload_bytes`, `message_max_attachments`) and the
/// recent targets (`TargetStore`). No message content and no people: the shared container is
/// outside the encrypted database (15 6.3). The recent targets with their display paths
/// (channel and thread names the person can see) are the only names in it, because the share
/// sheet must show them without the app (15 12; brief M5-06 Design note 7); every end of the
/// account removes them. The app writes every value except the counters and the nonce, which
/// the extensions write, and the recent targets, which both write.
public final class SharedState {
    /// The prefix of every key, so `DefaultsStore.keys` finds them.
    public static let prefix = "clientkit."
    static let previewModeKey = "clientkit.preview_mode"
    static let instanceKey = "clientkit.instance_id"
    static let fetchTimeoutKey = "clientkit.fetch_timeout_ms"
    static let shareRecentMaxKey = "clientkit.share.recent_max"
    static let shareMaxUploadBytesKey = "clientkit.share.max_upload_bytes"
    static let shareMaxAttachmentsKey = "clientkit.share.max_attachments"
    static let shareRecentTargetsKey = "clientkit.share.recent_targets"

    /// The store of the App Group.
    public let store: KeyValueStore

    /// Creates the state over `store`.
    ///
    /// - Parameter store: the App Group store.
    public init(store: KeyValueStore) {
        self.store = store
    }

    /// The state of the App Group `group`, or nil when the process may not open the suite.
    ///
    /// - Parameter group: the App Group identifier of `ClientKitConfiguration`.
    /// - Returns: the state over the group's `UserDefaults`.
    public static func appGroup(_ group: String) -> SharedState? {
        guard let defaults = UserDefaults(suiteName: group) else { return nil }
        return SharedState(store: DefaultsStore(defaults: defaults))
    }

    /// `notify.preview_mode` of the signed-in person as the app last saw it; nil before the app
    /// wrote one.
    public var previewMode: PreviewMode? {
        get { (store.value(forKey: Self.previewModeKey) as? String).flatMap(PreviewMode.init(rawValue:)) }
        set { store.set(newValue?.rawValue, forKey: Self.previewModeKey) }
    }

    /// The `instance_id` of the signed-in account, or nil.
    public var instanceID: String? {
        get { store.value(forKey: Self.instanceKey) as? String }
        set { store.set(newValue, forKey: Self.instanceKey) }
    }

    /// `client.push.fetch_timeout` in milliseconds as the app wrote it, or nil before the first
    /// start; the extension then keeps the placeholder, because it has no value of its own.
    public var fetchTimeoutMillis: Int? {
        get { (store.value(forKey: Self.fetchTimeoutKey) as? NSNumber)?.intValue }
        set { store.set(newValue.map { NSNumber(value: $0) }, forKey: Self.fetchTimeoutKey) }
    }

    /// Writes what the app knows at start and on every settings change, in one call.
    ///
    /// - Parameters:
    ///   - instanceID: the instance of the signed-in account.
    ///   - previewMode: `notify.preview_mode`.
    ///   - fetchTimeoutMillis: `client.push.fetch_timeout` in milliseconds.
    public func writeSettings(instanceID: String, previewMode: PreviewMode, fetchTimeoutMillis: Int) {
        self.instanceID = instanceID
        self.previewMode = previewMode
        self.fetchTimeoutMillis = fetchTimeoutMillis
    }

    /// Removes the values of the account (logout and every other end of the account), with the
    /// share settings and the recent targets; the counters stay for the next metrics batch.
    public func clearAccount() {
        instanceID = nil
        previewMode = nil
        clearShareData()
    }

    /// The settings of the Share Extension as the app last wrote them, or nil before the app
    /// wrote them (then the extension offers no recent targets and keeps none).
    public var shareSettings: ShareSettings? {
        guard let instanceID,
              let recentMax = (store.value(forKey: Self.shareRecentMaxKey) as? NSNumber)?.intValue
        else {
            return nil
        }
        return ShareSettings(
            instanceID: instanceID, recentMax: recentMax,
            maxUploadBytes: (store.value(forKey: Self.shareMaxUploadBytesKey) as? NSNumber)?.int64Value,
            maxAttachments: (store.value(forKey: Self.shareMaxAttachmentsKey) as? NSNumber)?.intValue)
    }

    /// Writes the share settings at start and whenever the configuration or the instance limits
    /// change; an unknown limit removes its key, so the sheet never enforces a stale value.
    ///
    /// - Parameter settings: the instance, `client.share.recent_targets` and the instance limits.
    public func writeShareSettings(_ settings: ShareSettings) {
        instanceID = settings.instanceID
        store.set(NSNumber(value: settings.recentMax), forKey: Self.shareRecentMaxKey)
        store.set(settings.maxUploadBytes.map { NSNumber(value: $0) }, forKey: Self.shareMaxUploadBytesKey)
        store.set(settings.maxAttachments.map { NSNumber(value: $0) }, forKey: Self.shareMaxAttachmentsKey)
    }

    /// Removes the share settings and the recent targets with their names (logout, the end of
    /// the account, `access.changed` that removes targets before the app writes them anew).
    public func clearShareData() {
        for key in [Self.shareRecentMaxKey, Self.shareMaxUploadBytesKey, Self.shareMaxAttachmentsKey, Self.shareRecentTargetsKey] {
            store.set(nil, forKey: key)
        }
    }
}

/// What the app tells the Share Extension through the App Group (brief M5-06 Design notes 2
/// and 7; D-W-115): the extension has neither `AppConfig` nor `InstanceLimits`.
public struct ShareSettings: Equatable, Sendable {
    /// The `instance_id` of the signed-in account.
    public var instanceID: String
    /// `client.share.recent_targets`: how many recent targets the sheet keeps and shows.
    public var recentMax: Int
    /// `file_max_upload_bytes` of the instance limits, or nil while unknown.
    public var maxUploadBytes: Int64?
    /// `message_max_attachments` of the instance limits, or nil while unknown.
    public var maxAttachments: Int?

    /// Creates the settings.
    ///
    /// - Parameters:
    ///   - instanceID: the instance of the signed-in account.
    ///   - recentMax: `client.share.recent_targets`.
    ///   - maxUploadBytes: `file_max_upload_bytes`, nil while unknown.
    ///   - maxAttachments: `message_max_attachments`, nil while unknown.
    public init(instanceID: String, recentMax: Int, maxUploadBytes: Int64?, maxAttachments: Int?) {
        self.instanceID = instanceID
        self.recentMax = recentMax
        self.maxUploadBytes = maxUploadBytes
        self.maxAttachments = maxAttachments
    }
}

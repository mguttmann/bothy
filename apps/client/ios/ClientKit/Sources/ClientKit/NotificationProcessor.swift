// SPDX-License-Identifier: Apache-2.0

import DeviceKeyCore
import Foundation
import UserNotifications

/// The own keys of a push next to `aps`, in the object `b` (docs/spec/13-push-notifications.md
/// 6.1): version, notification ID, kind, instance, thread and message.
public struct PushPayload: Equatable, Sendable {
    /// The kinds that point at a message; only these have a preview (13 6.4 check 1).
    public static let previewKinds: Set<String> = ["message", "mention", "dm", "run_done", "run_failed", "input_needed"]

    /// `b.nid`, the notification ID.
    public let nid: String
    /// `b.k`, the kind.
    public let kind: String
    /// `b.i`, the `instance_id`.
    public let instanceID: String
    /// `b.t`, the thread, absent for `security` and `admin`.
    public let threadID: String?
    /// `b.m`, the message, absent without a message.
    public let messageID: String?

    /// Reads `b` of a notification's `userInfo`.
    ///
    /// - Parameter userInfo: the payload without `aps` handling.
    /// - Returns: the keys, or nil when `b` is missing, its `v` is not `1` or a required key
    ///   (`nid`, `k`, `i`) is missing: then the extension leaves the notification unchanged.
    public static func parse(_ userInfo: [AnyHashable: Any]) -> PushPayload? {
        guard let b = userInfo["b"] as? [String: Any],
              (b["v"] as? NSNumber)?.intValue == 1,
              let nid = b["nid"] as? String, !nid.isEmpty,
              let kind = b["k"] as? String, !kind.isEmpty,
              let instance = b["i"] as? String, !instance.isEmpty
        else {
            return nil
        }
        return PushPayload(
            nid: nid, kind: kind, instanceID: instance,
            threadID: (b["t"] as? String).flatMap { $0.isEmpty ? nil : $0 },
            messageID: (b["m"] as? String).flatMap { $0.isEmpty ? nil : $0 })
    }
}

/// Waits for a time; `Task.sleep` in the extension, a switch the test turns in the tests, so no
/// test depends on the wall clock.
public protocol Sleeper {
    /// Returns after `seconds`, or earlier when the task is cancelled.
    ///
    /// - Parameter seconds: the time to wait.
    func sleep(seconds: TimeInterval) async
}

/// `Sleeper` over `Task.sleep`.
public struct TaskSleeper: Sleeper {
    /// Creates the sleeper.
    public init() {}

    /// See `Sleeper.sleep(seconds:)`.
    public func sleep(seconds: TimeInterval) async {
        try? await Task.sleep(nanoseconds: UInt64(max(0, seconds) * 1_000_000_000))
    }
}

/// What the extension needs from the outside; `production` builds it from the bundle.
public struct NotificationEnvironment {
    /// The App Group state, or nil when the build has no App Group.
    public let state: SharedState?
    /// The push items of the shared keychain, or nil when the build has no shared group.
    public let keychain: SharedKeychain?
    /// The display counters, or nil without an App Group.
    public let counters: DisplayCounters?
    /// The nonce cache, or nil without an App Group.
    public let nonces: NonceCache?
    /// The HTTP exchange of the preview fetch.
    public let transport: HTTPTransport
    /// Opens the `dpop` key of an instance; nil when there is none.
    public let signer: (String) -> ProofSigner?
    /// The localized texts.
    public let strings: PushStrings
    /// The clock of the display duration and of `iat`.
    public let now: () -> Date
    /// The timer of `client.push.fetch_timeout`.
    public let sleeper: Sleeper

    /// Creates an environment.
    ///
    /// - Parameters: the fields of the same names.
    public init(
        state: SharedState?, keychain: SharedKeychain?, counters: DisplayCounters?, nonces: NonceCache?,
        transport: HTTPTransport, signer: @escaping (String) -> ProofSigner?, strings: PushStrings,
        now: @escaping () -> Date, sleeper: Sleeper
    ) {
        self.state = state
        self.keychain = keychain
        self.counters = counters
        self.nonces = nonces
        self.transport = transport
        self.signer = signer
        self.strings = strings
        self.now = now
        self.sleeper = sleeper
    }

    /// The environment of the running extension: the shared places of its Info.plist, the
    /// keychain, `URLSession`, the device keys through `DeviceKeyCore` and its own strings.
    ///
    /// - Parameter bundle: the extension's bundle.
    /// - Returns: the environment; without a configuration every store is nil and every push
    ///   keeps its placeholder.
    public static func production(bundle: Bundle = .main) -> NotificationEnvironment {
        let config = ClientKitConfiguration.fromBundle(bundle)
        let state = config.flatMap { SharedState.appGroup($0.appGroup) }
        let manager = config.map { DeviceKeyManager(store: KeychainKeyStore(), accessGroup: $0.keychainGroup) }
        return NotificationEnvironment(
            state: state,
            keychain: config.map { SharedKeychain(store: KeychainSecretStore(accessGroup: $0.keychainGroup)) },
            counters: state.map { DisplayCounters(store: $0.store) },
            nonces: state.map { NonceCache(store: $0.store) },
            transport: URLSessionTransport(),
            signer: { scope in manager.flatMap { DeviceKeyProofSigner(manager: $0, scope: scope) } },
            strings: PushStrings.fromBundle(bundle),
            now: Date.init,
            sleeper: TaskSleeper())
    }
}

/// How a push was shown.
public enum DisplayOutcome: Equatable, Sendable {
    /// Title and body from the preview.
    case text
    /// "Message not available" after `404`.
    case notAvailable
    /// The placeholder stayed: `none`, no credentials, no network, an error or the time ran out.
    case placeholder
    /// The push was not one with a preview (`security`, `admin`, an unknown version); shown as
    /// it came.
    case unchanged
}

/// The logic of the Notification Service Extension (docs/spec/13-push-notifications.md 5.1, 6.1,
/// 6.4; 15 10.1; brief M5-04 Design note 4).
///
/// For each push with a preview kind it reads the credentials of the instance `b.i` from the
/// shared keychain and fetches the preview within `client.push.fetch_timeout`; with
/// `notify.preview_mode = none` (or no mode written yet) and without credentials it delivers
/// the placeholder at once and makes no request. It keeps `thread-id`, badge and sound of `aps`
/// (the content is a copy of the push's). It delivers exactly once and counts the duration from
/// receipt to delivery and whether the placeholder stayed. It never refreshes a token and never
/// starts a session (R-CL-07): its only credential is the preview token.
public final class NotificationProcessor {
    private let environment: NotificationEnvironment
    private let lock = NSLock()
    private var deliver: ((UNNotificationContent) -> Void)?
    private var placeholder: UNNotificationContent?
    private var receivedAt = Date()
    private var tasks: [Task<Void, Never>] = []
    private var delivered: DisplayOutcome?

    /// Creates the processor for one push.
    ///
    /// - Parameter environment: what the extension needs from the outside.
    public init(environment: NotificationEnvironment) {
        self.environment = environment
    }

    /// Handles one push; `deliver` is called exactly once, possibly from another thread.
    ///
    /// - Parameters:
    ///   - content: the content of the push (with the localized placeholder).
    ///   - deliver: the extension's content handler.
    /// - Returns: the outcome, once delivered.
    @discardableResult
    public func process(_ content: UNNotificationContent, deliver: @escaping (UNNotificationContent) -> Void) async -> DisplayOutcome {
        let start = environment.now()
        locked {
            self.deliver = deliver
            placeholder = content
            receivedAt = start
        }

        guard let payload = PushPayload.parse(content.userInfo),
              PushPayload.previewKinds.contains(payload.kind)
        else {
            return finish(.unchanged, content: content)
        }
        guard let messageID = payload.messageID,
              let state = environment.state,
              let mode = state.previewMode, mode != .none,
              let timeoutMillis = state.fetchTimeoutMillis, timeoutMillis > 0,
              let credentials = try? environment.keychain?.load(instanceID: payload.instanceID),
              let signer = environment.signer(payload.instanceID),
              let nonces = environment.nonces
        else {
            return finish(.placeholder, content: content)
        }
        // `client.push.fetch_timeout`, written by the app into the App Group.
        let timeout = TimeInterval(timeoutMillis) / 1000
        let client = PreviewClient(
            credentials: credentials, signer: signer, nonces: nonces,
            transport: environment.transport, timeout: timeout, now: environment.now)
        let timer = Task { [weak self, environment] in
            await environment.sleeper.sleep(seconds: timeout)
            guard !Task.isCancelled else { return }
            self?.expire()
        }
        locked { tasks.append(timer) }
        let result = await client.fetch(nid: payload.nid, messageID: messageID, kind: payload.kind)
        timer.cancel()
        switch result {
        case let .content(preview):
            let mutable = Self.copy(content)
            let rendered = PreviewRenderer.render(
                preview, placeholderTitle: content.title, placeholderBody: content.body, strings: environment.strings)
            mutable.title = rendered.title
            mutable.body = rendered.body
            return finish(.text, content: mutable)
        case .notAvailable:
            let mutable = Self.copy(content)
            mutable.body = environment.strings.notAvailable
            return finish(.notAvailable, content: mutable)
        case .failed:
            return finish(.placeholder, content: content)
        }
    }

    /// Delivers the best content so far, the placeholder, when the time is up
    /// (`serviceExtensionTimeWillExpire` or the end of `client.push.fetch_timeout`).
    public func expire() {
        if let content = locked({ placeholder }) {
            _ = finish(.placeholder, content: content)
        }
    }

    /// Delivers `content` unless something was delivered already; counts the delivery.
    ///
    /// - Returns: the outcome that was delivered, which is the earlier one when the time ran
    ///   out first.
    private func finish(_ outcome: DisplayOutcome, content: UNNotificationContent) -> DisplayOutcome {
        lock.lock()
        guard let deliver else {
            let earlier = delivered ?? outcome
            lock.unlock()
            return earlier
        }
        self.deliver = nil
        delivered = outcome
        let started = receivedAt
        let pending = tasks
        tasks = []
        lock.unlock()
        pending.forEach { $0.cancel() }
        if outcome != .unchanged {
            environment.counters?.record(
                duration: environment.now().timeIntervalSince(started), placeholder: outcome == .placeholder)
        }
        deliver(content)
        return outcome
    }

    /// Runs `body` under the lock; synchronous, so it may be called from the async paths.
    private func locked<T>(_ body: () -> T) -> T {
        lock.lock()
        defer { lock.unlock() }
        return body()
    }

    private static func copy(_ content: UNNotificationContent) -> UNMutableNotificationContent {
        // `mutableCopy` keeps `threadIdentifier`, badge, sound, category and `userInfo`.
        (content.mutableCopy() as? UNMutableNotificationContent) ?? UNMutableNotificationContent()
    }
}

/// The base class of the Notification Service Extensions of iOS and macOS: each target's
/// `NotificationService` is a subclass of this one without code of its own (ADR 0056 item 1).
///
/// Tests subclass it and override `makeEnvironment()`, so the same class the extension runs is
/// the one under test.
open class ClientKitNotificationService: UNNotificationServiceExtension {
    private var processor: NotificationProcessor?

    /// The environment of one push; the extension's bundle by default.
    ///
    /// - Returns: the environment.
    open func makeEnvironment() -> NotificationEnvironment {
        NotificationEnvironment.production(bundle: Bundle(for: type(of: self)))
    }

    /// Starts the preview fetch of `request` and delivers through `contentHandler` once.
    override open func didReceive(
        _ request: UNNotificationRequest, withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void
    ) {
        let processor = NotificationProcessor(environment: makeEnvironment())
        self.processor = processor
        let content = request.content
        Task {
            await processor.process(content, deliver: contentHandler)
        }
    }

    /// The system ends the extension's time: deliver the placeholder now.
    override open func serviceExtensionTimeWillExpire() {
        processor?.expire()
    }
}

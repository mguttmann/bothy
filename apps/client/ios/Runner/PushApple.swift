// SPDX-License-Identifier: Apache-2.0

import ClientKit
import Foundation
import UserNotifications

#if os(iOS)
    import Flutter
    import UIKit
#elseif os(macOS)
    import AppKit
    import FlutterMacOS
#endif

/// The runner side of push on iOS and macOS (brief M5-04 Design note 5): the Pigeon host of
/// `pigeons/push_apple.dart`, the notification center delegate and the APNs token callbacks.
///
/// The iOS and macOS runners compile this one file. What the app shares with its Notification
/// Service Extension goes through `ClientKit` (`SharedKeychain`, `SharedState`,
/// `DisplayCounters`), so the extension reads exactly what is written here. The preview token
/// arrives from Dart once and is written to the shared keychain at once; it is never logged.
final class PushApple: NSObject, PushAppleHostApi {
    /// The one instance of the app.
    static let shared = PushApple()

    private let config = ClientKitConfiguration.fromBundle()
    private lazy var state: SharedState? = config.flatMap { SharedState.appGroup($0.appGroup) }
    private lazy var keychain: SharedKeychain? = config.map {
        SharedKeychain(store: KeychainSecretStore(accessGroup: $0.keychainGroup))
    }
    private var flutterApi: PushAppleFlutterApi?
    private let lock = NSLock()
    private var pendingTaps: [PushTap] = []
    private var dartListens = false
    private var tokenWaiters: [CheckedContinuation<String, Error>] = []

    /// The delegate of the notification center: pushes here, every other notification to the
    /// delegate it replaced (the local notifications of M4-30 on macOS, the plugins on iOS).
    private lazy var router = NotificationCenterRouter { [weak self] payload in self?.tapped(payload) }

    /// Becomes the delegate of the notification center, keeping the one it replaces; called at
    /// launch, before a tap that started the app is delivered, and again after the plugins
    /// registered, since a plugin may take the center over when it registers.
    ///
    /// - Parameter fallback: the delegate to forward to when the center has none (the iOS app
    ///   delegate, which hands the callbacks to its plugins).
    func installDelegate(fallback: UNUserNotificationCenterDelegate? = nil) {
        router.install(on: UNUserNotificationCenter.current(), fallback: fallback)
    }

    /// Connects the Pigeon channel to the engine.
    ///
    /// - Parameter messenger: the engine's binary messenger.
    func attach(messenger: FlutterBinaryMessenger) {
        PushAppleHostApiSetup.setUp(binaryMessenger: messenger, api: self)
        flutterApi = PushAppleFlutterApi(binaryMessenger: messenger)
    }

    // MARK: APNs token

    /// The token of `didRegisterForRemoteNotificationsWithDeviceToken`.
    func didRegister(deviceToken: Data) {
        let hex = deviceToken.map { String(format: "%02x", $0) }.joined()
        resolveToken(.success(hex))
    }

    /// The failure of `didFailToRegisterForRemoteNotificationsWithError`.
    func didFailToRegister() {
        resolveToken(.failure(PushApplePigeonError(code: "unavailable", message: nil, details: nil)))
    }

    private func resolveToken(_ result: Result<String, Error>) {
        lock.lock()
        let waiters = tokenWaiters
        tokenWaiters = []
        lock.unlock()
        waiters.forEach { $0.resume(with: result) }
    }

    // MARK: PushAppleHostApi

    func isAvailable() throws -> Bool {
        state != nil && keychain != nil
    }

    func environment() throws -> String {
        ApsEnvironment.ofBundle()
    }

    func appId() throws -> String {
        Bundle.main.bundleIdentifier ?? ""
    }

    func isAuthorized() async throws -> Bool {
        let settings = await UNUserNotificationCenter.current().notificationSettings()
        switch settings.authorizationStatus {
        case .authorized, .provisional:
            return true
        default:
            #if os(iOS)
                return settings.authorizationStatus == .ephemeral
            #else
                return false
            #endif
        }
    }

    func requestAuthorization() async throws -> Bool {
        do {
            return try await UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge])
        } catch {
            throw PushApplePigeonError(code: "failed", message: nil, details: nil)
        }
    }

    func deviceToken() async throws -> String {
        try await withCheckedThrowingContinuation { continuation in
            lock.lock()
            tokenWaiters.append(continuation)
            lock.unlock()
            DispatchQueue.main.async {
                #if os(iOS)
                    UIApplication.shared.registerForRemoteNotifications()
                #else
                    NSApplication.shared.registerForRemoteNotifications()
                #endif
            }
        }
    }

    func saveRegistration(record: PushRegistrationRecord) throws {
        guard let keychain, let url = URL(string: record.serverUrl) else { throw Self.unavailable }
        do {
            try keychain.save(
                PushCredentials(previewToken: record.previewToken, registrationID: record.registrationId, serverURL: url),
                instanceID: record.instanceId)
        } catch {
            throw PushApplePigeonError(code: "failed", message: nil, details: nil)
        }
    }

    func registrationId(instanceId: String) throws -> String? {
        guard let keychain else { throw Self.unavailable }
        return (try? keychain.load(instanceID: instanceId))?.registrationID
    }

    func deleteRegistration(instanceId: String) throws {
        guard let keychain else { throw Self.unavailable }
        do {
            try keychain.delete(instanceID: instanceId)
        } catch {
            throw PushApplePigeonError(code: "failed", message: nil, details: nil)
        }
        if state?.instanceID == instanceId {
            state?.clearAccount()
        }
    }

    func writeSettings(instanceId: String, previewMode: String, fetchTimeoutMillis: Int64) throws {
        guard let state, let mode = PreviewMode(rawValue: previewMode) else { throw Self.unavailable }
        state.writeSettings(instanceID: instanceId, previewMode: mode, fetchTimeoutMillis: Int(fetchTimeoutMillis))
    }

    func takeDisplayCounters() throws -> PushDisplayCounters {
        guard let state else { throw Self.unavailable }
        let snapshot = DisplayCounters(store: state.store).take()
        return PushDisplayCounters(
            bucketBounds: DisplayCounters.bounds,
            bucketCounts: snapshot.buckets.map(Int64.init),
            placeholders: Int64(snapshot.placeholders))
    }

    func takePendingTaps() throws -> [PushTap] {
        lock.lock()
        defer { lock.unlock() }
        dartListens = true
        let taps = pendingTaps
        pendingTaps = []
        return taps
    }

    private static let unavailable = PushApplePigeonError(code: "unavailable", message: nil, details: nil)

    // MARK: Taps

    /// A push the person opened: its thread and message go to Dart, or wait until Dart asks.
    private func tapped(_ payload: PushPayload) {
        guard let thread = payload.threadID else { return }
        let tap = PushTap(instanceId: payload.instanceID, threadId: thread, messageId: payload.messageID)
        lock.lock()
        let listens = dartListens
        if !listens { pendingTaps.append(tap) }
        lock.unlock()
        if listens, let api = flutterApi {
            Task { @MainActor in try? await api.onTap(tap: tap) }
        }
    }
}

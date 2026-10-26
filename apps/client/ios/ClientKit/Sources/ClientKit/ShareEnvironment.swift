// SPDX-License-Identifier: Apache-2.0

import DeviceKeyCore
import Foundation

/// What the Share Extension needs from the outside, built from the bundle (brief M5-06 Design
/// notes 1 to 5).
public enum ShareEnvironment {
    /// The timeout of each request of the extension: a fixed protocol constant, not a server
    /// limit (the extension has no `AppConfig`). `URLRequest.timeoutInterval` is the longest time
    /// without any data, so a large part that keeps moving is never cut off by it.
    public static let requestTimeout: TimeInterval = 60

    /// The live pieces of one sheet.
    public struct Live {
        /// The share settings the app wrote.
        public let settings: ShareSettings
        /// The App Group state.
        public let state: SharedState
        /// The share inbox and staging folder in the App Group container.
        public let inbox: ShareInbox
        /// The recent targets.
        public let targets: TargetStore
        /// The child session, or nil when the keychain holds none or the device key is missing:
        /// then the sheet offers the recent targets only and every send goes to the inbox.
        public let session: ExtensionSession?
        /// The search, or nil without a session.
        public let search: TargetSearching?
        /// The send; it queues when there is no session.
        public let sender: ShareSending
    }

    /// The environment of the running extension: the App Group and the keychain group of its
    /// Info.plist, the App Group container, the child session's keychain item, the `dpop` device
    /// key through `DeviceKeyCore` (as `NotificationEnvironment.production`) and `URLSession`.
    ///
    /// - Parameter bundle: the extension's bundle.
    /// - Returns: the pieces, or nil when the build has no shared groups, the container cannot be
    ///   opened or the app has not written the share settings (nobody is signed in): then the
    ///   sheet can only explain that the app must be opened first.
    public static func live(bundle: Bundle = .main) -> Live? {
        guard let config = ClientKitConfiguration.fromBundle(bundle),
              let state = SharedState.appGroup(config.appGroup),
              let container = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: config.appGroup)
        else {
            return nil
        }
        let manager = DeviceKeyManager(store: KeychainKeyStore(), accessGroup: config.keychainGroup)
        return assemble(
            state: state, root: container, secrets: KeychainSecretStore(accessGroup: config.keychainGroup),
            signer: { DeviceKeyProofSigner(manager: manager, scope: $0) }, transport: URLSessionTransport(), now: Date.init)
    }

    /// Wires the pieces; separate from `live` so the tests build the same graph over fakes.
    ///
    /// - Parameters:
    ///   - state: the App Group state.
    ///   - root: the App Group container.
    ///   - secrets: the shared keychain.
    ///   - signer: opens the `dpop` key of an instance.
    ///   - transport: the HTTP exchange.
    ///   - now: the clock.
    /// - Returns: the pieces, or nil without share settings.
    static func assemble(
        state: SharedState, root: URL, secrets: SecretStore, signer: (String) -> ProofSigner?,
        transport: HTTPTransport, now: @escaping () -> Date
    ) -> Live? {
        guard let settings = state.shareSettings else { return nil }
        let instance = settings.instanceID
        let keychain = ShareSessionKeychain(store: secrets)
        let inbox = ShareInbox(root: root)
        let targets = TargetStore(state: state)
        var session: ExtensionSession?
        if (try? keychain.load(instanceID: instance)) != nil, let key = signer(instance) {
            session = ExtensionSession(
                instanceID: instance, keychain: keychain, signer: key, nonces: NonceCache(store: state.store),
                transport: transport, now: now, timeout: requestTimeout)
        }
        return Live(
            settings: settings, state: state, inbox: inbox, targets: targets, session: session,
            search: session.map { TargetSearch(api: $0) },
            sender: ShareSender(instanceID: instance, api: session, transport: transport, inbox: inbox, targets: targets, now: now))
    }
}

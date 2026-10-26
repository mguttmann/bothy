// SPDX-License-Identifier: Apache-2.0

import ClientKit
import Foundation

#if os(iOS)
    import Flutter
#elseif os(macOS)
    import FlutterMacOS
#endif

/// The runner side of sharing on iOS and macOS (brief M5-06 Design notes 3 to 5 and 7): the
/// Pigeon host of `pigeons/share_apple.dart` over ClientKit.
///
/// The iOS and macOS runners compile this one file. Everything goes through the same ClientKit
/// pieces the Share Extension reads: the child session in the shared keychain
/// (`ShareSessionKeychain` over `KeychainSecretStore` of the access group), the settings and the
/// recent targets in the App Group (`SharedState`, `TargetStore`) and the share inbox in the App
/// Group container (`ShareInbox`). A build without the groups (no signing team, D-M5-04-1)
/// reports `isAvailable` false and answers every other call with the error `unavailable`. The
/// tokens of the child session arrive from Dart once and go to the keychain at once; no token
/// appears in a log or an error.
final class ShareApple: NSObject, ShareAppleHostApi {
    /// The one instance of the app.
    static let shared = ShareApple()

    private let config = ClientKitConfiguration.fromBundle()
    private lazy var state: SharedState? = config.flatMap { SharedState.appGroup($0.appGroup) }
    private lazy var keychain: ShareSessionKeychain? = config.map {
        ShareSessionKeychain(store: KeychainSecretStore(accessGroup: $0.keychainGroup))
    }
    private lazy var inbox: ShareInbox? = config
        .flatMap { FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: $0.appGroup) }
        .map { ShareInbox(root: $0) }

    /// Connects the Pigeon channel to the engine.
    ///
    /// - Parameter messenger: the engine's binary messenger.
    func attach(messenger: FlutterBinaryMessenger) {
        ShareAppleHostApiSetup.setUp(binaryMessenger: messenger, api: self)
    }

    // MARK: ShareAppleHostApi

    func isAvailable() throws -> Bool {
        state != nil && keychain != nil && inbox != nil
    }

    func saveExtensionSession(record: ExtensionSessionRecord) throws {
        guard state != nil, inbox != nil, let keychain else { throw Self.unavailable }
        guard let server = URL(string: record.serverUrl) else { throw Self.failed }
        let tokens = ExtensionSessionTokens(
            serverURL: server, sessionID: record.sessionId, accessToken: record.accessToken,
            accessExpiresAt: Self.date(record.accessExpiresAtMillis), refreshToken: record.refreshToken,
            refreshExpiresAt: Self.date(record.refreshExpiresAtMillis))
        do {
            try keychain.save(tokens, instanceID: record.instanceId)
        } catch {
            throw Self.failed
        }
    }

    func hasExtensionSession(instanceId: String, nowMillis: Int64) throws -> Bool {
        guard state != nil, inbox != nil, let keychain else { throw Self.unavailable }
        // A damaged or unreadable item counts as none: the app then creates a new child, which
        // replaces the old one on the server (08 7.4).
        guard let tokens = (try? keychain.load(instanceID: instanceId)) ?? nil else { return false }
        return tokens.refreshExpiresAt > Self.date(nowMillis)
    }

    func deleteExtensionSession(instanceId: String) throws {
        guard state != nil, inbox != nil, let keychain else { throw Self.unavailable }
        do {
            try keychain.delete(instanceID: instanceId)
        } catch {
            throw Self.failed
        }
    }

    func writeShareSettings(settings: ShareSettingsRecord) throws {
        guard let state, keychain != nil, inbox != nil else { throw Self.unavailable }
        state.writeShareSettings(ShareSettings(
            instanceID: settings.instanceId, recentMax: Int(settings.recentTargetsMax),
            maxUploadBytes: settings.fileMaxUploadBytes, maxAttachments: settings.messageMaxAttachments.map { Int($0) }))
    }

    func writeRecentTargets(targets: [ShareTargetRecord]) throws {
        guard let state, keychain != nil, inbox != nil else { throw Self.unavailable }
        let mapped = try targets.map { record -> ShareTarget in
            guard let kind = ShareTargetKind(rawValue: record.kind) else { throw Self.failed }
            return ShareTarget(kind: kind, channelID: record.channelId, threadID: record.threadId, path: record.path)
        }
        TargetStore(state: state).replace(mapped)
    }

    func clearShareData() throws {
        guard let state, keychain != nil, let inbox else { throw Self.unavailable }
        state.clearShareData()
        // R-SY-14: nothing of the account's shares stays in the App Group.
        do {
            try inbox.removeAll()
        } catch {
            throw Self.failed
        }
    }

    func readInbox() throws -> [ShareInboxEntryRecord] {
        guard state != nil, keychain != nil, let inbox else { throw Self.unavailable }
        return inbox.entries().map { entry in
            ShareInboxEntryRecord(
                clientMsgId: entry.clientMsgID, instanceId: entry.instanceID,
                target: ShareTargetRecord(
                    kind: entry.target.kind.rawValue, channelId: entry.target.channelID, threadId: entry.target.threadID,
                    path: entry.target.path),
                comment: entry.comment, title: entry.title, createdAtMillis: Self.millis(entry.createdAt),
                files: entry.files.map {
                    ShareInboxFileRecord(
                        path: $0.url.path, name: $0.name, mimeType: $0.mimeType, sizeBytes: $0.sizeBytes,
                        completedFileId: $0.completedFileID)
                })
        }
    }

    func deleteInboxEntry(clientMsgId: String) throws {
        guard state != nil, keychain != nil, let inbox else { throw Self.unavailable }
        do {
            try inbox.delete(clientMsgID: clientMsgId)
        } catch {
            throw Self.failed
        }
    }

    // MARK: Helpers

    private static let unavailable = ShareApplePigeonError(code: "unavailable", message: nil, details: nil)
    private static let failed = ShareApplePigeonError(code: "failed", message: nil, details: nil)

    private static func date(_ millis: Int64) -> Date {
        Date(timeIntervalSince1970: TimeInterval(millis) / 1000)
    }

    private static func millis(_ date: Date) -> Int64 {
        Int64((date.timeIntervalSince1970 * 1000).rounded())
    }
}

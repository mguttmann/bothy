// SPDX-License-Identifier: Apache-2.0

import Foundation

/// What the person sends from the share sheet.
public struct ShareDraft: Equatable, Sendable {
    /// The chosen target.
    public let target: ShareTarget
    /// The comment, the message text.
    public let comment: String
    /// The title of a new forum post; nil for other targets.
    public let title: String?
    /// The staged items, in order.
    public let items: [ShareItem]

    /// Creates a draft.
    ///
    /// - Parameters:
    ///   - target: the target.
    ///   - comment: the message text.
    ///   - title: the forum post's title, or nil.
    ///   - items: the staged items.
    public init(target: ShareTarget, comment: String, title: String?, items: [ShareItem]) {
        self.target = target
        self.comment = comment
        self.title = title
        self.items = items
    }
}

/// How a send ended; the sheet closes on `sent` and `queued` and shows `refused`.
public enum ShareOutcome: Equatable, Sendable {
    /// The message exists on the server.
    case sent
    /// The share is in the inbox; the app sends it with the same `client_msg_id`.
    case queued
    /// The server refused it with a `4xx` other than `401` and `429` (`status` and the problem
    /// `code`), or the inbox could not be written (`status` 0, code `inbox_write_failed`);
    /// nothing is kept.
    case refused(status: Int, code: String?)
}

/// Sends a draft; `ShareSender` in the extension, a fake in the sheet's tests.
public protocol ShareSending {
    /// Sends `draft` or puts it into the inbox.
    ///
    /// - Parameter draft: what the person sends.
    /// - Returns: the outcome; never throws.
    func send(_ draft: ShareDraft) async -> ShareOutcome
}

/// The send of the Share Extension (15 12; 04 13.2; brief M5-06 Design note 4).
///
/// At Send it makes the `client_msg_id` (UUIDv7) and writes the inbox entry with that ID and
/// the files first, so a share survives the end of the extension's process during an upload.
/// Then it uploads each file of the entry (`UploadClient`) and sends the message into the thread
/// or the channel's main thread, or a new forum post with `first_message` (`MessageClient`). On
/// success it records the target in the recent targets and deletes the entry. On a refusal (any
/// `4xx` other than `401` and `429`, and not `409 request_in_progress`) it deletes the entry and
/// keeps nothing: the reason is shown. On every other failure (no network, a timeout, `5xx`,
/// `429`, a missing or revoked child session) the entry stays with the IDs of the uploads
/// already completed, so the app sends it later exactly once.
///
/// Example:
///
///     let sender = ShareSender(instanceID: instance, api: session, transport: transport,
///                              inbox: inbox, targets: targets)
///     let outcome = await sender.send(draft)
public final class ShareSender: ShareSending {
    private let instanceID: String
    private let api: AuthorizedRequester?
    private let transport: HTTPTransport
    private let inbox: ShareInbox
    private let targets: TargetStore
    private let reader: FileReading
    private let now: () -> Date

    /// Creates the sender.
    ///
    /// - Parameters:
    ///   - instanceID: the instance of the signed-in account.
    ///   - api: the child session, or nil when there is none (then every send is queued).
    ///   - transport: the exchange for presigned upload parts.
    ///   - inbox: the share inbox.
    ///   - targets: the recent targets.
    ///   - reader: opens the staged files.
    ///   - now: the clock of the `client_msg_id` and of the entry's time.
    public init(
        instanceID: String, api: AuthorizedRequester?, transport: HTTPTransport, inbox: ShareInbox, targets: TargetStore,
        reader: FileReading = FileHandleReader(), now: @escaping () -> Date = Date.init
    ) {
        self.instanceID = instanceID
        self.api = api
        self.transport = transport
        self.inbox = inbox
        self.targets = targets
        self.reader = reader
        self.now = now
    }

    /// See `ShareSending.send(_:)`.
    public func send(_ draft: ShareDraft) async -> ShareOutcome {
        let createdAt = now()
        let clientMsgID = UUIDv7.make(now: createdAt)
        // The inbox entry is written before the first network call: iOS may end the extension
        // during an upload (the person switches away, the memory limit) without any `catch`
        // running, and the app must still learn of the share. It goes only after a confirmed
        // send or a refusal; a send of the app after a lost answer carries the same
        // `client_msg_id` and creates nothing twice.
        let items: [ShareItem]
        do {
            items = try queue(draft, clientMsgID: clientMsgID, createdAt: createdAt)
        } catch {
            inbox.removeStaged(draft.items)
            return .refused(status: 0, code: "inbox_write_failed")
        }
        var completed = [String?](repeating: nil, count: items.count)
        do {
            guard let api else { throw ShareError.noSession }
            let clock = now
            let uploads = UploadClient(api: api, transport: transport, reader: reader, idempotencyKey: { UUIDv7.make(now: clock()) })
            for (index, item) in items.enumerated() {
                completed[index] = try await uploads.upload(item, channelID: draft.target.channelID)
            }
            let messages = MessageClient(api: api)
            let fileIDs = completed.compactMap { $0 }
            switch draft.target.kind {
            case .thread, .channel:
                // A target without its thread cannot be sent from here; the app resolves it.
                guard let thread = draft.target.threadID else { throw ShareError.transient }
                try await messages.createMessage(threadID: thread, clientMsgID: clientMsgID, content: draft.comment, fileIDs: fileIDs)
            case .newPost:
                try await messages.createPost(
                    channelID: draft.target.channelID, title: draft.title ?? "", clientMsgID: clientMsgID,
                    content: draft.comment, fileIDs: fileIDs)
            }
        } catch let ShareError.refused(status, code) {
            try? inbox.delete(clientMsgID: clientMsgID)
            return .refused(status: status, code: code)
        } catch {
            // The uploads completed so far go into the entry, so the app does not send them again.
            if completed.contains(where: { $0 != nil }) {
                try? inbox.recordCompleted(clientMsgID: clientMsgID, fileIDs: completed)
            }
            return .queued
        }
        targets.recordUse(draft.target)
        try? inbox.delete(clientMsgID: clientMsgID)
        return .sent
    }

    /// Writes the inbox entry without completed uploads; the files move into it and the empty
    /// staging folder goes. Returns the files as they lie in the entry, which the uploads read.
    private func queue(_ draft: ShareDraft, clientMsgID: String, createdAt: Date) throws -> [ShareItem] {
        try inbox.write(
            clientMsgID: clientMsgID, instanceID: instanceID, target: draft.target, comment: draft.comment,
            title: draft.title, createdAt: createdAt,
            files: draft.items.map { ShareInboxItem(item: $0, completedFileID: nil) })
        inbox.removeStaged(draft.items)
        guard let entry = inbox.entry(clientMsgID: clientMsgID) else { throw ShareInboxError.invalidID }
        return entry.files.map { ShareItem(fileURL: $0.url, name: $0.name, mimeType: $0.mimeType, sizeBytes: $0.sizeBytes) }
    }
}

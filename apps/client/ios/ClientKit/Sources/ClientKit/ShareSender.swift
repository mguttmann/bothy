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
/// At Send it makes the `client_msg_id` (UUIDv7), uploads each item (`UploadClient`), then sends
/// the message into the thread or the channel's main thread, or a new forum post with
/// `first_message` (`MessageClient`). On success it records the target in the recent targets and
/// deletes the staged copies. On a refusal (any `4xx` other than `401` and `429`, and not
/// `409 request_in_progress`) it deletes the staged copies and keeps nothing: the reason is
/// shown. On every other failure (no network, a timeout, `5xx`, `429`, a missing or revoked
/// child session) it writes an inbox entry with the same `client_msg_id`, the files and the IDs
/// of the uploads already completed, so the app sends it later exactly once.
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
        var completed = [String?](repeating: nil, count: draft.items.count)
        do {
            guard let api else { throw ShareError.noSession }
            let clock = now
            let uploads = UploadClient(api: api, transport: transport, reader: reader, idempotencyKey: { UUIDv7.make(now: clock()) })
            for (index, item) in draft.items.enumerated() {
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
            inbox.removeStaged(draft.items)
            return .refused(status: status, code: code)
        } catch {
            return queue(draft, clientMsgID: clientMsgID, createdAt: createdAt, completed: completed)
        }
        targets.recordUse(draft.target)
        inbox.removeStaged(draft.items)
        return .sent
    }

    /// Writes the inbox entry; the files move into it and the empty staging folder goes.
    private func queue(_ draft: ShareDraft, clientMsgID: String, createdAt: Date, completed: [String?]) -> ShareOutcome {
        defer { inbox.removeStaged(draft.items) }
        do {
            try inbox.write(
                clientMsgID: clientMsgID, instanceID: instanceID, target: draft.target, comment: draft.comment,
                title: draft.title, createdAt: createdAt,
                files: zip(draft.items, completed).map { ShareInboxItem(item: $0, completedFileID: $1) })
            return .queued
        } catch {
            return .refused(status: 0, code: "inbox_write_failed")
        }
    }
}

// SPDX-License-Identifier: Apache-2.0

import Foundation

/// The message of a share: `createMessage` into a thread, or `createThread` with
/// `first_message` for a new forum post (brief M5-06 Design note 4; 04 13.2).
///
/// The `client_msg_id` comes from the caller, made when the person tapped Send: the server
/// answers a repeat with the original (`200` with `Idempotent-Replay`), so the app can send an
/// inbox entry again with the same ID and the message still exists once. Both `201` and that
/// `200` are success.
///
/// Example:
///
///     let messages = MessageClient(api: session)
///     try await messages.createMessage(threadID: thread, clientMsgID: id, content: text, fileIDs: files)
public final class MessageClient {
    private let api: AuthorizedRequester

    /// Creates the client.
    ///
    /// - Parameter api: the session for the calls to the server.
    public init(api: AuthorizedRequester) {
        self.api = api
    }

    /// Sends a message into `threadID`.
    ///
    /// - Parameters:
    ///   - threadID: the thread, or the main thread of a channel.
    ///   - clientMsgID: the UUIDv7 made at Send.
    ///   - content: the comment as `content_md`; may be empty when files are attached.
    ///   - fileIDs: the completed uploads, in order.
    /// - Throws: `ShareError` in the classes of Design note 4.
    public func createMessage(threadID: String, clientMsgID: String, content: String, fileIDs: [String]) async throws {
        let server = try await api.serverURL()
        guard let url = ShareAPI.url(server: server, path: "/threads/\(ShareAPI.segment(threadID))/messages") else {
            throw ShareError.noSession
        }
        try await post(url, body: Self.message(clientMsgID: clientMsgID, content: content, fileIDs: fileIDs))
    }

    /// Creates a new post in the forum `channelID` with the message as its first message; no
    /// `Idempotency-Key` is needed, because the first message's `client_msg_id` makes the call
    /// idempotent.
    ///
    /// - Parameters:
    ///   - channelID: the forum channel.
    ///   - title: the post's title.
    ///   - clientMsgID: the UUIDv7 made at Send.
    ///   - content: the comment as `content_md`.
    ///   - fileIDs: the completed uploads, in order.
    /// - Throws: `ShareError` in the classes of Design note 4.
    public func createPost(channelID: String, title: String, clientMsgID: String, content: String, fileIDs: [String]) async throws {
        let server = try await api.serverURL()
        guard let url = ShareAPI.url(server: server, path: "/channels/\(ShareAPI.segment(channelID))/threads") else {
            throw ShareError.noSession
        }
        try await post(url, body: ["title": title, "first_message": Self.message(clientMsgID: clientMsgID, content: content, fileIDs: fileIDs)])
    }

    private static func message(clientMsgID: String, content: String, fileIDs: [String]) -> [String: Any] {
        var body: [String: Any] = ["client_msg_id": clientMsgID, "content_md": content]
        if !fileIDs.isEmpty { body["attachment_file_ids"] = fileIDs }
        return body
    }

    private func post(_ url: URL, body: [String: Any]) async throws {
        let (data, response) = try await api.send(ShareAPI.jsonRequest("POST", url: url, body: body))
        try ShareAPI.check(response, data)
    }
}

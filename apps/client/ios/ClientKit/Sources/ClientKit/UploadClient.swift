// SPDX-License-Identifier: Apache-2.0

import CryptoKit
import Foundation

/// One shared item, a file in the App Group's staging folder (brief M5-06 Design note 2).
public struct ShareItem: Equatable, Sendable {
    /// The staged copy of the item.
    public let fileURL: URL
    /// The name shown and sent as `name` of the upload.
    public let name: String
    /// The type sent as `declared_mime_type`; the server detects the type itself.
    public let mimeType: String
    /// The size in bytes, for the sheet's check against `file_max_upload_bytes`.
    public let sizeBytes: Int64

    /// Creates an item.
    ///
    /// - Parameters:
    ///   - fileURL: the staged copy.
    ///   - name: the file name.
    ///   - mimeType: the declared type.
    ///   - sizeBytes: the size in bytes.
    public init(fileURL: URL, name: String, mimeType: String, sizeBytes: Int64) {
        self.fileURL = fileURL
        self.name = name
        self.mimeType = mimeType
        self.sizeBytes = sizeBytes
    }
}

/// Opens files for reading at offsets; `FileHandleReader` in the extension, a recording one in
/// the tests, which prove that no read is larger than a part.
public protocol FileReading {
    /// Opens `url`.
    ///
    /// - Parameter url: the file.
    /// - Returns: the open file; the caller closes it.
    /// - Throws: when the file cannot be opened.
    func open(_ url: URL) throws -> ReadableFile
}

/// An open file read in ranges.
public protocol ReadableFile: AnyObject {
    /// Reads up to `count` bytes at `offset`.
    ///
    /// - Parameters:
    ///   - offset: the byte offset.
    ///   - count: the most bytes to read.
    /// - Returns: the bytes, fewer at the end of the file, empty after it.
    /// - Throws: a read failure.
    func read(at offset: Int64, count: Int) throws -> Data

    /// Closes the file.
    func close()
}

/// `FileReading` over `FileHandle`: the bytes come from disk range by range.
public final class FileHandleReader: FileReading {
    /// Creates the reader.
    public init() {}

    /// See `FileReading.open(_:)`.
    public func open(_ url: URL) throws -> ReadableFile {
        HandleFile(handle: try FileHandle(forReadingFrom: url))
    }
}

private final class HandleFile: ReadableFile {
    private let handle: FileHandle

    init(handle: FileHandle) {
        self.handle = handle
    }

    func read(at offset: Int64, count: Int) throws -> Data {
        try handle.seek(toOffset: UInt64(offset))
        return try handle.read(upToCount: count) ?? Data()
    }

    func close() {
        try? handle.close()
    }
}

/// The upload protocol of docs/spec/12-files.md 3 for the Share Extension (15 12, 14; brief
/// M5-06 Design note 4).
///
/// It hashes the file in chunks (`sha256` of `createUpload` is the hash of the whole file), then
/// announces it with `createUpload` and an `Idempotency-Key`, sends the parts in order, each read
/// from the file at its offset with a `Content-Digest`, and closes with `completeUpload`. Never
/// more than one part is in memory, because a Share Extension has a small memory limit (15 12).
/// In mode `direct` the part URLs are paths of the server and go through the session (token and
/// proof); in mode `presigned` they are object store URLs and get neither. After a failed part it
/// asks `getUpload` once and sends only the parts the server has not confirmed; a second failure
/// ends the upload, and the app uploads the file from the inbox later.
///
/// Example:
///
///     let uploads = UploadClient(api: session, transport: transport)
///     let fileID = try await uploads.upload(item, channelID: target.channelID)
public final class UploadClient {
    /// The read buffer of the hash pass: a buffer size, not a limit, small enough for the
    /// extension's memory.
    public static let defaultHashChunkBytes = 1 << 20

    private let api: AuthorizedRequester
    private let transport: HTTPTransport
    private let reader: FileReading
    private let hashChunkBytes: Int
    private let idempotencyKey: () -> String

    /// Creates the client.
    ///
    /// - Parameters:
    ///   - api: the session for the calls to the server.
    ///   - transport: the exchange for presigned parts, which carry no credentials.
    ///   - reader: opens the staged files.
    ///   - hashChunkBytes: the read buffer of the hash pass.
    ///   - idempotencyKey: a new UUIDv7 per keyed call.
    public init(
        api: AuthorizedRequester, transport: HTTPTransport, reader: FileReading = FileHandleReader(),
        hashChunkBytes: Int = UploadClient.defaultHashChunkBytes, idempotencyKey: @escaping () -> String = { UUIDv7.make() }
    ) {
        self.api = api
        self.transport = transport
        self.reader = reader
        self.hashChunkBytes = max(1, hashChunkBytes)
        self.idempotencyKey = idempotencyKey
    }

    /// Uploads `item` for `channelID` and completes the upload session.
    ///
    /// - Parameters:
    ///   - item: the staged file.
    ///   - channelID: the channel of the target; the file is scoped to it.
    /// - Returns: the file ID, attachable once `completeUpload` answered `200` (15 14).
    /// - Throws: `ShareError` in the classes of Design note 4; a read failure of the file.
    public func upload(_ item: ShareItem, channelID: String) async throws -> String {
        let (sha256, size) = try digest(item.fileURL)
        let server = try await api.serverURL()
        guard let createURL = ShareAPI.url(server: server, path: "/channels/\(ShareAPI.segment(channelID))/uploads") else {
            throw ShareError.noSession
        }
        var declaration: [String: Any] = ["name": item.name, "size_bytes": size, "sha256": sha256]
        if !item.mimeType.isEmpty { declaration["declared_mime_type"] = item.mimeType }
        let (data, response) = try await api.send(
            ShareAPI.jsonRequest("POST", url: createURL, body: declaration, idempotencyKey: idempotencyKey()))
        try ShareAPI.check(response, data)
        guard let plan = UploadPlan(data, size: size) else { throw ShareError.transient }

        let file = try reader.open(item.fileURL)
        defer { file.close() }
        var confirmed: [Int: String] = [:]
        var resumed = false
        while let part = plan.parts.first(where: { confirmed[$0.number] == nil }) {
            do {
                confirmed[part.number] = try await send(part, of: plan, from: file, server: server)
            } catch ShareError.transient where !resumed {
                resumed = true
                for (number, etag) in try await confirmedParts(plan.fileID, server: server) {
                    confirmed[number] = etag
                }
            }
        }
        return try await complete(plan.fileID, parts: confirmed, server: server)
    }

    /// The SHA-256 (lowercase hex) and size of the file, read in chunks of `hashChunkBytes`.
    private func digest(_ url: URL) throws -> (String, Int64) {
        let file = try reader.open(url)
        defer { file.close() }
        var hasher = SHA256()
        var offset: Int64 = 0
        while true {
            let chunk = try autoreleasepool { try file.read(at: offset, count: hashChunkBytes) }
            if chunk.isEmpty { break }
            hasher.update(data: chunk)
            offset += Int64(chunk.count)
        }
        return (hasher.finalize().map { String(format: "%02x", $0) }.joined(), offset)
    }

    /// Sends one part and returns its ETag.
    private func send(_ part: UploadPlan.Part, of plan: UploadPlan, from file: ReadableFile, server: URL) async throws -> String {
        let offset = Int64(part.number - 1) * plan.partSize
        let length = Int(min(plan.partSize, plan.size - offset))
        let bytes = try autoreleasepool { try file.read(at: offset, count: length) }
        // The file changed after the hash pass: the server would refuse the whole upload.
        guard bytes.count == length else { throw CocoaError(.fileReadCorruptFile) }

        var request: URLRequest
        if plan.direct {
            guard let url = URL(string: part.url, relativeTo: server)?.absoluteURL else { throw ShareError.transient }
            request = URLRequest(url: url)
        } else {
            guard let url = URL(string: part.url), ShareAPI.isAllowed(url) else { throw ShareError.transient }
            request = URLRequest(url: url, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: ShareEnvironment.requestTimeout)
            request.httpShouldHandleCookies = false
        }
        request.httpMethod = "PUT"
        for (name, value) in part.headers {
            request.setValue(value, forHTTPHeaderField: name)
        }
        request.setValue("sha-256=:\(Data(SHA256.hash(data: bytes)).base64EncodedString()):", forHTTPHeaderField: "Content-Digest")
        request.httpBody = bytes

        let (data, response) = plan.direct
            ? try await api.send(request)
            : try await ShareAPI.exchange(request, over: transport)
        try ShareAPI.check(response, data)
        let etag = (ShareAPI.object(data)?["etag"] as? String) ?? response.value(forHTTPHeaderField: "ETag")
        guard let etag, !etag.isEmpty else { throw ShareError.transient }
        return etag
    }

    /// `getUpload`: the parts the server has confirmed, by number.
    private func confirmedParts(_ fileID: String, server: URL) async throws -> [Int: String] {
        guard let url = ShareAPI.url(server: server, path: "/uploads/\(ShareAPI.segment(fileID))") else { throw ShareError.transient }
        let (data, response) = try await api.send(ShareAPI.jsonRequest("GET", url: url, body: nil))
        try ShareAPI.check(response, data)
        guard let parts = ShareAPI.object(data)?["completed_parts"] as? [[String: Any]] else { throw ShareError.transient }
        var confirmed: [Int: String] = [:]
        for part in parts {
            if let number = (part["part_number"] as? NSNumber)?.intValue, let etag = part["etag"] as? String {
                confirmed[number] = etag
            }
        }
        return confirmed
    }

    /// `completeUpload` with every part and its ETag; returns the file ID.
    private func complete(_ fileID: String, parts: [Int: String], server: URL) async throws -> String {
        guard let url = ShareAPI.url(server: server, path: "/uploads/\(ShareAPI.segment(fileID))/complete") else { throw ShareError.transient }
        let listed = parts.keys.sorted().map { ["part_number": $0, "etag": parts[$0]!] as [String: Any] }
        let (data, response) = try await api.send(
            ShareAPI.jsonRequest("POST", url: url, body: ["parts": listed], idempotencyKey: idempotencyKey()))
        try ShareAPI.check(response, data)
        return (ShareAPI.object(data)?["id"] as? String) ?? fileID
    }
}

/// The `UploadSession` answer of `createUpload`, checked against the declared size.
struct UploadPlan {
    struct Part {
        let number: Int
        let url: String
        let headers: [(String, String)]
    }

    let fileID: String
    let direct: Bool
    let partSize: Int64
    let size: Int64
    let parts: [Part]

    /// Reads the plan; nil when it does not match the API or does not cover `size` exactly.
    init?(_ data: Data, size: Int64) {
        guard let object = ShareAPI.object(data),
              let fileID = object["file_id"] as? String, !fileID.isEmpty,
              let mode = object["mode"] as? String, mode == "direct" || mode == "presigned",
              let partSize = (object["part_size_bytes"] as? NSNumber)?.int64Value, partSize > 0,
              let listed = object["parts"] as? [[String: Any]]
        else {
            return nil
        }
        var parts: [Part] = []
        for entry in listed {
            guard let number = (entry["part_number"] as? NSNumber)?.intValue, let url = entry["url"] as? String else { return nil }
            let headers = (entry["headers"] as? [[String: Any]] ?? []).compactMap { pair -> (String, String)? in
                guard let name = pair["name"] as? String, let value = pair["value"] as? String else { return nil }
                return (name, value)
            }
            parts.append(Part(number: number, url: url, headers: headers))
        }
        parts.sort { $0.number < $1.number }
        // Exactly the parts 1...n that cover the declared size, the last one possibly shorter.
        let count = Int((size + partSize - 1) / partSize)
        guard parts.map(\.number) == Array(1...max(1, count)), count >= 1 else { return nil }
        self.fileID = fileID
        self.direct = mode == "direct"
        self.partSize = partSize
        self.size = size
        self.parts = parts
    }
}

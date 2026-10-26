// SPDX-License-Identifier: Apache-2.0

import Foundation

/// A file of an inbox entry to write: the staged item and, when its upload was completed before
/// the send failed, the file ID, so the app does not upload it again.
public struct ShareInboxItem: Equatable, Sendable {
    /// The staged item; `ShareInbox.write` moves its file into the entry.
    public let item: ShareItem
    /// The ID of the completed upload, or nil.
    public let completedFileID: String?

    /// Creates the item.
    ///
    /// - Parameters:
    ///   - item: the staged item.
    ///   - completedFileID: the completed upload's file ID, or nil.
    public init(item: ShareItem, completedFileID: String?) {
        self.item = item
        self.completedFileID = completedFileID
    }
}

/// A file of an entry the app reads.
public struct ShareInboxFile: Equatable, Sendable {
    /// The file inside the entry's `files` folder.
    public let url: URL
    /// The original name.
    public let name: String
    /// The declared type.
    public let mimeType: String
    /// The size in bytes.
    public let sizeBytes: Int64
    /// The ID of an upload the extension completed, or nil.
    public let completedFileID: String?

    /// Creates the file.
    ///
    /// - Parameters: the fields of the same names.
    public init(url: URL, name: String, mimeType: String, sizeBytes: Int64, completedFileID: String?) {
        self.url = url
        self.name = name
        self.mimeType = mimeType
        self.sizeBytes = sizeBytes
        self.completedFileID = completedFileID
    }
}

/// One share the extension could not send, as the app takes it over (brief M5-06 Design note 5).
public struct ShareInboxEntry: Equatable, Sendable {
    /// The `client_msg_id` the extension made at Send; the app sends with the same one.
    public let clientMsgID: String
    /// The instance the share belongs to.
    public let instanceID: String
    /// The target.
    public let target: ShareTarget
    /// The comment, the message text.
    public let comment: String
    /// The title of a new forum post, or nil.
    public let title: String?
    /// When the person tapped Send.
    public let createdAt: Date
    /// The files, in order.
    public let files: [ShareInboxFile]

    /// Creates the entry.
    ///
    /// - Parameters: the fields of the same names.
    public init(clientMsgID: String, instanceID: String, target: ShareTarget, comment: String, title: String?, createdAt: Date, files: [ShareInboxFile]) {
        self.clientMsgID = clientMsgID
        self.instanceID = instanceID
        self.target = target
        self.comment = comment
        self.title = title
        self.createdAt = createdAt
        self.files = files
    }
}

/// A failure of the inbox itself.
public enum ShareInboxError: Error, Equatable {
    /// The `client_msg_id` is not a UUID text, so it cannot name a folder.
    case invalidID
}

/// The share inbox and the staging folder in the App Group container (15 6.1, 12; brief M5-06
/// Design notes 2, 4 and 5).
///
/// Layout under the container root:
///
///     ShareStaging/<uuid>/                    copies of the shared items while the sheet is open
///     ShareInbox/<client_msg_id>/manifest.json
///     ShareInbox/<client_msg_id>/files/<n>-<safe name>
///
/// An entry is built in a hidden folder `ShareInbox/.tmp-<uuid>/` (files moved in, the manifest
/// written last) and then renamed to its `client_msg_id`: a reader sees a whole entry or none.
/// Readers ignore names starting with `.` and folders without `manifest.json`. The manifest
/// (version 1) holds the `client_msg_id`, the instance, the target, the comment, the title of a
/// new post, the time of Send and the files with the IDs of completed uploads; never a token.
///
/// Example:
///
///     let inbox = ShareInbox(root: container)
///     try inbox.write(clientMsgID: id, instanceID: instance, target: target, comment: text,
///                     title: nil, createdAt: now, files: items)
///     for entry in inbox.entries() { ... try inbox.delete(clientMsgID: entry.clientMsgID) }
public final class ShareInbox {
    /// The version of the manifest format.
    public static let manifestVersion = 1
    /// The most characters kept of a file name in the entry, so a long name cannot exceed the
    /// file system's name length with the number prefix; a storage detail, not a limit.
    static let storedNameLength = 100

    private let root: URL
    private let files: FileManager

    /// Creates the inbox in `root`.
    ///
    /// - Parameters:
    ///   - root: the App Group container (`FileManager.containerURL(forSecurityApplicationGroupIdentifier:)`).
    ///   - fileManager: the file manager.
    public init(root: URL, fileManager: FileManager = .default) {
        self.root = root
        self.files = fileManager
    }

    private var inboxFolder: URL { root.appendingPathComponent("ShareInbox", isDirectory: true) }
    private var stagingRoot: URL { root.appendingPathComponent("ShareStaging", isDirectory: true) }

    /// A new, empty staging folder for the copies of one share.
    ///
    /// - Returns: `ShareStaging/<uuid>/`.
    /// - Throws: when the folder cannot be created.
    public func stagingFolder() throws -> URL {
        let folder = stagingRoot.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try files.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder
    }

    /// Removes the staged copies of `items` and their staging folders; files outside the staging
    /// folder are never touched.
    ///
    /// - Parameter items: the items of one share.
    public func removeStaged(_ items: [ShareItem]) {
        let base = stagingRoot.standardizedFileURL.path + "/"
        for item in items {
            let file = item.fileURL.standardizedFileURL
            guard file.path.hasPrefix(base) else { continue }
            // `ShareStaging/<uuid>/...`: remove the share's whole folder.
            let relative = file.path.dropFirst(base.count)
            guard let folder = relative.split(separator: "/").first, !folder.hasPrefix(".") else { continue }
            try? files.removeItem(at: stagingRoot.appendingPathComponent(String(folder), isDirectory: true))
        }
    }

    /// Writes an entry; the files are moved from the staging folder into it.
    ///
    /// - Parameters:
    ///   - clientMsgID: the UUIDv7 made at Send; the entry's folder name.
    ///   - instanceID: the instance of the share.
    ///   - target: the target.
    ///   - comment: the message text.
    ///   - title: the title of a new forum post, or nil.
    ///   - createdAt: when the person tapped Send.
    ///   - files: the staged items with the IDs of completed uploads.
    /// - Throws: `ShareInboxError.invalidID`, or a file system failure; then no entry exists and
    ///   the temporary folder is gone.
    public func write(
        clientMsgID: String, instanceID: String, target: ShareTarget, comment: String, title: String?,
        createdAt: Date, files items: [ShareInboxItem]
    ) throws {
        try Self.check(clientMsgID)
        try files.createDirectory(at: inboxFolder, withIntermediateDirectories: true)
        let final = inboxFolder.appendingPathComponent(clientMsgID, isDirectory: true)
        let temporary = inboxFolder.appendingPathComponent(".tmp-\(UUID().uuidString)", isDirectory: true)
        do {
            try files.createDirectory(at: temporary.appendingPathComponent("files", isDirectory: true), withIntermediateDirectories: true)
            var listed: [[String: Any]] = []
            for (index, entry) in items.enumerated() {
                let stored = "\(index + 1)-\(Self.safeName(entry.item.name))"
                try files.moveItem(at: entry.item.fileURL, to: temporary.appendingPathComponent("files/\(stored)"))
                var file: [String: Any] = [
                    "file": "files/\(stored)", "name": entry.item.name, "mime_type": entry.item.mimeType,
                    "size_bytes": entry.item.sizeBytes,
                ]
                file["completed_file_id"] = entry.completedFileID
                listed.append(file)
            }
            var manifest: [String: Any] = [
                "version": Self.manifestVersion, "client_msg_id": clientMsgID, "instance_id": instanceID,
                "target": try Self.targetObject(target), "comment": comment,
                "created_at_ms": ExtensionSessionTokens.millis(createdAt), "files": listed,
            ]
            manifest["title"] = title
            let data = try JSONSerialization.data(withJSONObject: manifest, options: [.sortedKeys, .withoutEscapingSlashes])
            try data.write(to: temporary.appendingPathComponent("manifest.json"), options: .atomic)
            try files.moveItem(at: temporary, to: final)
        } catch {
            try? files.removeItem(at: temporary)
            throw error
        }
    }

    /// The finished entries, oldest first; damaged ones are skipped.
    ///
    /// - Returns: the entries.
    public func entries() -> [ShareInboxEntry] {
        guard let names = try? files.contentsOfDirectory(atPath: inboxFolder.path) else { return [] }
        return names.filter { !$0.hasPrefix(".") }
            .compactMap { read(inboxFolder.appendingPathComponent($0, isDirectory: true)) }
            .sorted { ($0.createdAt, $0.clientMsgID) < ($1.createdAt, $1.clientMsgID) }
    }

    /// Deletes the entry of `clientMsgID` with its files; a missing entry is not an error.
    ///
    /// - Parameter clientMsgID: the entry's ID.
    /// - Throws: `ShareInboxError.invalidID`, or a file system failure.
    public func delete(clientMsgID: String) throws {
        try Self.check(clientMsgID)
        let folder = inboxFolder.appendingPathComponent(clientMsgID, isDirectory: true)
        guard files.fileExists(atPath: folder.path) else { return }
        try files.removeItem(at: folder)
    }

    /// Deletes every entry, every half-written entry and every staging folder (the copies of
    /// a sheet whose process ended early): the end of an account (R-SY-14) leaves nothing of
    /// its shares in the App Group. Missing folders are not an error.
    ///
    /// - Throws: a file system failure.
    public func removeAll() throws {
        for folder in [inboxFolder, stagingRoot] where files.fileExists(atPath: folder.path) {
            try files.removeItem(at: folder)
        }
    }

    private func read(_ folder: URL) -> ShareInboxEntry? {
        guard let data = try? Data(contentsOf: folder.appendingPathComponent("manifest.json")),
              let object = ShareAPI.object(data),
              (object["version"] as? NSNumber)?.intValue == Self.manifestVersion,
              let id = object["client_msg_id"] as? String, id == folder.lastPathComponent,
              let instance = object["instance_id"] as? String,
              let targetObject = object["target"] as? [String: Any],
              let targetData = try? JSONSerialization.data(withJSONObject: targetObject),
              let target = try? JSONDecoder().decode(ShareTarget.self, from: targetData),
              let comment = object["comment"] as? String,
              let created = (object["created_at_ms"] as? NSNumber)?.int64Value,
              let listed = object["files"] as? [[String: Any]]
        else {
            return nil
        }
        var entryFiles: [ShareInboxFile] = []
        for file in listed {
            // Only `files/<name>`: a manifest can never point outside its entry.
            guard let path = file["file"] as? String, path.hasPrefix("files/"),
                  case let stored = String(path.dropFirst("files/".count)),
                  !stored.isEmpty, !stored.contains("/"), !stored.hasPrefix("."),
                  let name = file["name"] as? String, let mime = file["mime_type"] as? String,
                  let size = (file["size_bytes"] as? NSNumber)?.int64Value
            else {
                return nil
            }
            entryFiles.append(ShareInboxFile(
                url: folder.appendingPathComponent("files", isDirectory: true).appendingPathComponent(stored),
                name: name, mimeType: mime, sizeBytes: size, completedFileID: file["completed_file_id"] as? String))
        }
        return ShareInboxEntry(
            clientMsgID: id, instanceID: instance, target: target, comment: comment, title: object["title"] as? String,
            createdAt: ExtensionSessionTokens.date(millis: created), files: entryFiles)
    }

    /// The target JSON of the contract.
    private static func targetObject(_ target: ShareTarget) throws -> Any {
        try JSONSerialization.jsonObject(with: JSONEncoder().encode(target))
    }

    /// A UUID text: hex digits and hyphens only, so it names exactly one folder in the inbox.
    private static func check(_ id: String) throws {
        let allowed = Set("0123456789abcdefABCDEF-")
        guard !id.isEmpty, id.count <= 64, id.allSatisfy({ allowed.contains($0) }) else { throw ShareInboxError.invalidID }
    }

    /// `name` with only ASCII letters, digits, `.`, `_` and `-`, no leading dot, shortened.
    static func safeName(_ name: String) -> String {
        let mapped = String(name.unicodeScalars.map { scalar -> Character in
            let ascii = scalar.isASCII && (CharacterSet.alphanumerics.contains(scalar) || "._-".unicodeScalars.contains(scalar))
            return ascii ? Character(scalar) : "_"
        })
        let trimmed = String(mapped.drop { $0 == "." }.prefix(storedNameLength))
        return trimmed.isEmpty ? "file" : trimmed
    }
}

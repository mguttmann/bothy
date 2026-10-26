// SPDX-License-Identifier: Apache-2.0

import ClientKit
import Foundation
import UniformTypeIdentifiers

#if canImport(UIKit)
    import UIKit
#elseif canImport(AppKit)
    import AppKit
#endif

/// A value an attachment hands over instead of a file.
public enum ShareProvidedValue: Equatable, Sendable {
    /// Text, a selection of another app.
    case text(String)
    /// A link, or a file of another app by its URL.
    case url(URL)
    /// Bytes without a file, for example an image an app holds in memory.
    case data(Data)
}

/// One shared attachment: `ItemProviderAttachment` over `NSItemProvider` in the extension, a
/// fake in the tests.
public protocol ShareAttachment {
    /// The registered type identifiers, the most faithful first.
    var typeIdentifiers: [String] { get }
    /// The name the source app suggests, or nil.
    var suggestedName: String? { get }

    /// Loads a file representation of `type`.
    ///
    /// - Parameters:
    ///   - type: a type identifier of `typeIdentifiers`.
    ///   - handler: receives the file, which exists only while the handler runs (the system
    ///     deletes its temporary copy afterwards), or the failure.
    func loadFile(forType type: String, _ handler: @escaping (Result<URL, Error>) -> Void)

    /// Loads the value of `type`.
    ///
    /// - Parameter type: a type identifier of `typeIdentifiers`.
    /// - Returns: the text, URL or bytes.
    /// - Throws: when the attachment cannot hand it over.
    func loadValue(forType type: String) async throws -> ShareProvidedValue
}

/// Why an attachment could not be loaded.
public enum ShareLoadError: Error, Equatable {
    /// No type of the attachment can be sent.
    case unsupported
    /// The attachment gave nothing usable.
    case unreadable
    /// The item is a folder, not a file.
    case notAFile
}

/// `ShareAttachment` over an `NSItemProvider` of the extension's input items.
public final class ItemProviderAttachment: ShareAttachment {
    private let provider: NSItemProvider

    /// Wraps `provider`.
    ///
    /// - Parameter provider: an attachment of an `NSExtensionItem`.
    public init(_ provider: NSItemProvider) {
        self.provider = provider
    }

    /// See `ShareAttachment.typeIdentifiers`.
    public var typeIdentifiers: [String] { provider.registeredTypeIdentifiers }

    /// See `ShareAttachment.suggestedName`.
    public var suggestedName: String? { provider.suggestedName }

    /// See `ShareAttachment.loadFile(forType:_:)`.
    public func loadFile(forType type: String, _ handler: @escaping (Result<URL, Error>) -> Void) {
        _ = provider.loadFileRepresentation(forTypeIdentifier: type) { url, error in
            if let url {
                handler(.success(url))
            } else {
                handler(.failure(error ?? ShareLoadError.unreadable))
            }
        }
    }

    /// See `ShareAttachment.loadValue(forType:)`. A link type may arrive as the URL's data
    /// representation and is decoded; an image object an app holds in memory is handed over as
    /// PNG bytes.
    public func loadValue(forType type: String) async throws -> ShareProvidedValue {
        let isLink = UTType(type)?.conforms(to: .url) ?? false
        return try await withCheckedThrowingContinuation { continuation in
            provider.loadItem(forTypeIdentifier: type, options: nil) { item, error in
                if isLink, let data = item as? Data, let url = URL(dataRepresentation: data, relativeTo: nil) {
                    continuation.resume(returning: .url(url))
                } else if let value = Self.value(of: item) {
                    continuation.resume(returning: value)
                } else {
                    continuation.resume(throwing: error ?? ShareLoadError.unreadable)
                }
            }
        }
    }

    private static func value(of item: NSSecureCoding?) -> ShareProvidedValue? {
        switch item {
        case let url as URL:
            return .url(url)
        case let text as String:
            return .text(text)
        case let text as NSAttributedString:
            return .text(text.string)
        case let data as Data:
            return .data(data)
        default:
            return imageData(item).map(ShareProvidedValue.data)
        }
    }

    #if canImport(UIKit)
        private static func imageData(_ item: NSSecureCoding?) -> Data? {
            (item as? UIImage)?.pngData()
        }
    #elseif canImport(AppKit)
        private static func imageData(_ item: NSSecureCoding?) -> Data? {
            guard let image = item as? NSImage, let tiff = image.tiffRepresentation else { return nil }
            return NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:])
        }
    #endif
}

/// The file operations of the staging folder; `LiveStagingFileSystem` in the extension, a
/// recording wrapper in the tests.
public protocol ShareStagingFileSystem {
    /// A new, empty folder for the copies of one share.
    ///
    /// - Returns: the folder.
    /// - Throws: when it cannot be created.
    func makeFolder() throws -> URL

    /// Runs `body` with `url` under a coordinated read (`NSFileCoordinator`), so the file of
    /// another app is not read while that app writes it.
    ///
    /// - Parameters:
    ///   - url: the file.
    ///   - body: reads the file at the URL it is given.
    /// - Returns: what `body` returns.
    /// - Throws: a coordination failure or what `body` throws.
    func coordinatedRead<T>(_ url: URL, _ body: (URL) throws -> T) throws -> T

    /// Copies a file without reading it into memory.
    ///
    /// - Parameters:
    ///   - source: the file.
    ///   - destination: the new copy.
    /// - Throws: `ShareLoadError.notAFile` for a folder, or a file system failure.
    func copy(from source: URL, to destination: URL) throws

    /// Writes `data` to a new file in chunks.
    ///
    /// - Parameters:
    ///   - data: the bytes.
    ///   - destination: the new file.
    /// - Throws: a file system failure.
    func write(_ data: Data, to destination: URL) throws

    /// The size of a file.
    ///
    /// - Parameter url: the file.
    /// - Returns: its size in bytes.
    /// - Throws: when the file cannot be read.
    func size(of url: URL) throws -> Int64
}

/// The staging folder on disk: `FileManager` copies (a clone on APFS, else copied in blocks by
/// the system), `NSFileCoordinator` reads, chunked writes.
public final class LiveStagingFileSystem: ShareStagingFileSystem {
    /// The bytes written at once; a memory detail, not a limit.
    public static let defaultChunkBytes = 1 << 20

    private let folder: () throws -> URL
    private let chunkBytes: Int
    private let files = FileManager.default

    /// Creates the file system.
    ///
    /// - Parameters:
    ///   - makeFolder: makes the fresh folder of one share, `ShareInbox.stagingFolder` in the
    ///     App Group container.
    ///   - chunkBytes: the bytes written at once.
    public init(makeFolder: @escaping () throws -> URL, chunkBytes: Int = defaultChunkBytes) {
        self.folder = makeFolder
        self.chunkBytes = max(1, chunkBytes)
    }

    /// See `ShareStagingFileSystem.makeFolder()`.
    public func makeFolder() throws -> URL {
        try folder()
    }

    /// See `ShareStagingFileSystem.coordinatedRead(_:_:)`.
    public func coordinatedRead<T>(_ url: URL, _ body: (URL) throws -> T) throws -> T {
        var coordinationError: NSError?
        var result: Result<T, Error>?
        NSFileCoordinator(filePresenter: nil).coordinate(readingItemAt: url, options: [], error: &coordinationError) { readable in
            result = Result { try body(readable) }
        }
        if let coordinationError { throw coordinationError }
        guard let result else { throw ShareLoadError.unreadable }
        return try result.get()
    }

    /// See `ShareStagingFileSystem.copy(from:to:)`.
    public func copy(from source: URL, to destination: URL) throws {
        var isFolder: ObjCBool = false
        guard files.fileExists(atPath: source.path, isDirectory: &isFolder) else { throw ShareLoadError.unreadable }
        guard !isFolder.boolValue else { throw ShareLoadError.notAFile }
        try files.copyItem(at: source, to: destination)
    }

    /// See `ShareStagingFileSystem.write(_:to:)`.
    public func write(_ data: Data, to destination: URL) throws {
        try write(data, to: destination, observe: nil)
    }

    /// Writes `data` in chunks and reports the size of each one; the tests observe them.
    func write(_ data: Data, to destination: URL, observe: ((Int) -> Void)?) throws {
        guard files.createFile(atPath: destination.path, contents: nil) else { throw ShareLoadError.unreadable }
        let handle = try FileHandle(forWritingTo: destination)
        defer { try? handle.close() }
        var offset = data.startIndex
        while offset < data.endIndex {
            let end = min(offset + chunkBytes, data.endIndex)
            // A slice shares the bytes of `data`: no second copy in memory.
            try handle.write(contentsOf: data[offset..<end])
            observe?(end - offset)
            offset = end
        }
    }

    /// See `ShareStagingFileSystem.size(of:)`.
    public func size(of url: URL) throws -> Int64 {
        guard let size = try files.attributesOfItem(atPath: url.path)[.size] as? NSNumber else { throw ShareLoadError.unreadable }
        return size.int64Value
    }
}

/// What the loader made of the shared attachments.
public struct ShareLoadResult: Equatable, Sendable {
    /// The staged files, in the order shared.
    public var items: [ShareItem]
    /// The shared text and links, one per line; the sheet puts it into the comment.
    public var text: String
    /// How many attachments could not be loaded; the sheet shows it and keeps Send disabled, so
    /// nothing is dropped without the person seeing it.
    public var failures: Int

    /// Creates the result.
    ///
    /// - Parameters: the fields of the same names.
    public init(items: [ShareItem], text: String, failures: Int) {
        self.items = items
        self.text = text
        self.failures = failures
    }
}

/// Loads the shared attachments into the staging folder (15 12 and 14; brief M5-06 Design
/// note 2).
///
/// Each attachment is handled by its types:
///
/// - a file URL (Finder, Files) is copied with a coordinated read;
/// - a file type (image, movie, PDF, any data that is not text or a link) is loaded as a file
///   representation and copied while the system's temporary copy exists; a HEIC or HEIF photo is
///   written as JPEG by `JPEGConverter` instead (when the attachment also offers the system's
///   own JPEG, that one is taken and nothing is decoded here); when the file representation
///   fails, the value is taken (a file URL is copied, bytes are written in chunks);
/// - a link becomes text, unless it names a local file;
/// - text becomes text, unless the value is a file or bytes, which are staged as a file.
///
/// All files of one share go into one fresh folder of `makeFolder`, made at the first file, as
/// `<n>-<safe name>`; the item keeps the name the person knows. Nothing is read into memory as a
/// whole. An attachment that cannot be loaded is counted, never dropped silently.
///
/// Example:
///
///     let loader = ShareItemLoader(fileSystem: LiveStagingFileSystem(makeFolder: inbox.stagingFolder))
///     let result = await loader.load(attachments, texts: texts)
public final class ShareItemLoader {
    private let fileSystem: ShareStagingFileSystem
    private let converter: ImageConverting

    /// Creates the loader.
    ///
    /// - Parameters:
    ///   - fileSystem: the staging folder's file operations.
    ///   - converter: the JPEG conversion of photos.
    public init(fileSystem: ShareStagingFileSystem, converter: ImageConverting = JPEGConverter()) {
        self.fileSystem = fileSystem
        self.converter = converter
    }

    /// Loads `attachments` one after the other.
    ///
    /// - Parameters:
    ///   - attachments: the attachments of the input items, in order.
    ///   - texts: texts of the input items themselves (`attributedContentText`), first in the
    ///     message text.
    /// - Returns: the staged items, the text and the number of failures; never throws.
    public func load(_ attachments: [ShareAttachment], texts: [String] = []) async -> ShareLoadResult {
        var lines = texts.compactMap(Self.line)
        var items: [ShareItem] = []
        var failures = 0
        let folder = StagingFolder(fileSystem)
        for attachment in attachments {
            do {
                switch try await load(attachment, number: items.count + 1, folder: folder) {
                case let .file(item):
                    items.append(item)
                case let .text(text):
                    if let line = Self.line(text) { lines.append(line) }
                }
            } catch {
                failures += 1
            }
        }
        if items.isEmpty {
            // Every file failed: the folder made for them would stay behind empty.
            folder.removeIfMade()
        }
        return ShareLoadResult(items: items, text: lines.joined(separator: "\n"), failures: failures)
    }

    private enum Loaded {
        case file(ShareItem)
        case text(String)
    }

    /// How an attachment is loaded, by its types.
    private enum Plan {
        case fileURL
        case file(UTType)
        case link(UTType)
        case text(UTType)
    }

    private func load(_ attachment: ShareAttachment, number: Int, folder: StagingFolder) async throws -> Loaded {
        switch try Self.plan(attachment.typeIdentifiers) {
        case .fileURL:
            return try stage(try await attachment.loadValue(forType: UTType.fileURL.identifier), of: attachment, type: nil, number: number, folder: folder)
        case let .file(type):
            do {
                return .file(try await loadFile(attachment, type: type, number: number, folder: folder))
            } catch {
                return try stage(try await attachment.loadValue(forType: type.identifier), of: attachment, type: type, number: number, folder: folder)
            }
        case let .link(type), let .text(type):
            return try stage(try await attachment.loadValue(forType: type.identifier), of: attachment, type: type, number: number, folder: folder)
        }
    }

    private static func plan(_ identifiers: [String]) throws -> Plan {
        let types = identifiers.compactMap { UTType($0) }
        if types.contains(.fileURL) { return .fileURL }
        // File content: data that is neither a link nor plain text (an SVG is text and image).
        let files = types.filter {
            ($0.conforms(to: .data) || $0.conforms(to: .content)) && !$0.conforms(to: .url)
                && (!$0.conforms(to: .text) || $0.conforms(to: .image))
        }
        if let first = files.first {
            // The system's own JPEG of a photo needs no decode on this side (15 14).
            return .file(JPEGConverter.needsConversion(first) && files.contains(.jpeg) ? .jpeg : first)
        }
        if let link = types.first(where: { $0.conforms(to: .url) }) { return .link(link) }
        if let text = types.first(where: { $0.conforms(to: .text) }) { return .text(text) }
        throw ShareLoadError.unsupported
    }

    /// Loads the file representation and stages it while the temporary copy exists.
    private func loadFile(_ attachment: ShareAttachment, type: UTType, number: Int, folder: StagingFolder) async throws -> ShareItem {
        try await withCheckedThrowingContinuation { continuation in
            attachment.loadFile(forType: type.identifier) { result in
                continuation.resume(with: Result {
                    let file = try result.get()
                    let name = Self.name(attachment.suggestedName, fallback: file.lastPathComponent, type: type)
                    return try self.stageFile(file, name: name, type: type, number: number, folder: folder)
                })
            }
        }
    }

    /// Stages a value: a local file is copied, bytes are written, a link or text is text.
    private func stage(_ value: ShareProvidedValue, of attachment: ShareAttachment, type: UTType?, number: Int, folder: StagingFolder) throws -> Loaded {
        switch value {
        case let .url(url) where url.isFileURL:
            let name = url.lastPathComponent.isEmpty ? Self.name(attachment.suggestedName, fallback: "file", type: type) : url.lastPathComponent
            return .file(try stageFile(url, name: name, type: UTType(filenameExtension: url.pathExtension), number: number, folder: folder))
        case let .url(url):
            return .text(url.absoluteString)
        case let .text(text):
            return .text(text)
        case let .data(data):
            let name = Self.name(attachment.suggestedName, fallback: Self.fallbackName(type), type: type)
            let destination = try folder.url().appendingPathComponent(Self.stagedName(number, name))
            try fileSystem.write(data, to: destination)
            return .file(ShareItem(fileURL: destination, name: name, mimeType: Self.mimeType(type, name: name), sizeBytes: try fileSystem.size(of: destination)))
        }
    }

    /// Copies or converts a file into the staging folder under a coordinated read.
    private func stageFile(_ source: URL, name: String, type: UTType?, number: Int, folder: StagingFolder) throws -> ShareItem {
        let target = try folder.url()
        if let type, JPEGConverter.needsConversion(type) {
            let jpegName = (name as NSString).deletingPathExtension + ".jpg"
            let destination = target.appendingPathComponent(Self.stagedName(number, jpegName))
            do {
                try fileSystem.coordinatedRead(source) { try converter.convert(from: $0, to: destination) }
                return ShareItem(fileURL: destination, name: jpegName, mimeType: "image/jpeg", sizeBytes: try fileSystem.size(of: destination))
            } catch {
                // A photo ImageIO cannot convert still goes out, unchanged, as the file it is.
                try? FileManager.default.removeItem(at: destination)
            }
        }
        let destination = target.appendingPathComponent(Self.stagedName(number, name))
        try fileSystem.coordinatedRead(source) { try fileSystem.copy(from: $0, to: destination) }
        return ShareItem(fileURL: destination, name: name, mimeType: Self.mimeType(type, name: name), sizeBytes: try fileSystem.size(of: destination))
    }

    /// The trimmed text, or nil when nothing is left.
    private static func line(_ text: String) -> String? {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    /// The suggested name (else `fallback`) with the type's extension when it has none; a JPEG
    /// the system made of a photo named `IMG_0001.HEIC` becomes `IMG_0001.jpg`.
    private static func name(_ suggested: String?, fallback: String, type: UTType?) -> String {
        let base = suggested.flatMap(line) ?? fallback
        let fileExtension = (base as NSString).pathExtension
        guard let type else { return base }
        if type.conforms(to: .jpeg) {
            if ["jpg", "jpeg"].contains(fileExtension.lowercased()) { return base }
            return (fileExtension.isEmpty ? base : (base as NSString).deletingPathExtension) + ".jpg"
        }
        guard fileExtension.isEmpty else { return base }
        return type.preferredFilenameExtension.map { "\(base).\($0)" } ?? base
    }

    /// The name of bytes without a suggested name.
    private static func fallbackName(_ type: UTType?) -> String {
        guard let type else { return "file" }
        if type.conforms(to: .text) { return "text" }
        if type.conforms(to: .image) { return "image" }
        return "file"
    }

    /// The declared type of the upload: from the type, else from the name's extension.
    private static func mimeType(_ type: UTType?, name: String) -> String {
        type?.preferredMIMEType ?? UTType(filenameExtension: (name as NSString).pathExtension)?.preferredMIMEType
            ?? "application/octet-stream"
    }

    /// `<n>-<name>` with no path separator, colon or leading dot, so the copy stays a plain
    /// file inside the share's folder whatever the source app named it.
    static func stagedName(_ number: Int, _ name: String) -> String {
        let mapped = String(name.map { "/:\0".contains($0) ? "_" : $0 })
        let trimmed = String(mapped.drop { $0 == "." || $0 == " " })
        return "\(number)-\(trimmed.isEmpty ? "file" : trimmed)"
    }
}

/// The share's staging folder, made once at the first file.
private final class StagingFolder {
    private let fileSystem: ShareStagingFileSystem
    private let lock = NSLock()
    private var made: URL?

    init(_ fileSystem: ShareStagingFileSystem) {
        self.fileSystem = fileSystem
    }

    func url() throws -> URL {
        lock.lock()
        defer { lock.unlock() }
        if let made { return made }
        let folder = try fileSystem.makeFolder()
        made = folder
        return folder
    }

    func removeIfMade() {
        lock.lock()
        defer { lock.unlock() }
        if let made { try? FileManager.default.removeItem(at: made) }
        made = nil
    }
}

// SPDX-License-Identifier: Apache-2.0
//
// Test doubles of the share sheet tests (brief M5-06 Design notes 1 and 2): images generated in
// the test through a sequential data provider (so a large image never sits in memory while it is
// written), the process's memory figures, a gate the test opens itself, fake attachments, a
// recording staging file system, and fakes of the send and the search. No test depends on the
// wall clock: waits are gates, never times.

import ClientKit
import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

@testable import ClientKitShare

// MARK: Images

/// Feeds the rows of a generated image to Core Graphics on demand.
private final class PatternRows {
    let bytesPerRow: Int
    var position = 0

    init(bytesPerRow: Int) {
        self.bytesPerRow = bytesPerRow
    }
}

/// A `width` x `height` RGB image whose pixels are computed when the encoder asks for them, so
/// writing it never holds the decoded image.
func patternImage(width: Int, height: Int) -> CGImage {
    let bytesPerRow = width * 4
    var callbacks = CGDataProviderSequentialCallbacks(
        version: 0,
        getBytes: { info, buffer, count in
            let rows = Unmanaged<PatternRows>.fromOpaque(info!).takeUnretainedValue()
            let bytes = buffer.assumingMemoryBound(to: UInt8.self)
            for index in 0..<count {
                let position = rows.position + index
                let row = position / rows.bytesPerRow
                let column = (position % rows.bytesPerRow) / 4
                bytes[index] = UInt8(truncatingIfNeeded: column &* 3 &+ row &* 5 &+ (position % 4) &* 70)
            }
            rows.position += count
            return count
        },
        skipForward: { info, count in
            Unmanaged<PatternRows>.fromOpaque(info!).takeUnretainedValue().position += Int(count)
            return count
        },
        rewind: { info in Unmanaged<PatternRows>.fromOpaque(info!).takeUnretainedValue().position = 0 },
        releaseInfo: { info in Unmanaged<PatternRows>.fromOpaque(info!).release() })
    let rows = PatternRows(bytesPerRow: bytesPerRow)
    let provider = CGDataProvider(sequentialInfo: Unmanaged.passRetained(rows).toOpaque(), callbacks: &callbacks)!
    return CGImage(
        width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: bytesPerRow,
        space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.noneSkipLast.rawValue),
        provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent)!
}

/// Writes a generated image of `type` (PNG or HEIC, both written by ImageIO on macOS) with an
/// EXIF orientation and a GPS position, so a test sees what the conversion keeps.
func writeImage(_ url: URL, type: UTType, width: Int, height: Int, orientation: Int = 6) throws {
    guard let destination = CGImageDestinationCreateWithURL(url as CFURL, type.identifier as CFString, 1, nil) else {
        throw FixtureError.cannotWrite(type.identifier)
    }
    let properties: [CFString: Any] = [
        kCGImagePropertyOrientation: orientation,
        kCGImagePropertyGPSDictionary: [kCGImagePropertyGPSLatitude: 52.5, kCGImagePropertyGPSLatitudeRef: "N"],
    ]
    CGImageDestinationAddImage(destination, patternImage(width: width, height: height), properties as CFDictionary)
    guard CGImageDestinationFinalize(destination) else { throw FixtureError.cannotWrite(type.identifier) }
}

/// Writes a large HEIC through `sips` in a child process from a PNG written here band by band,
/// so the memory of the HEIC encoder never counts against this process's measurement.
func writeLargeHEIC(_ url: URL, side: Int) throws {
    let png = url.deletingPathExtension().appendingPathExtension("png")
    try writeImage(png, type: .png, width: side, height: side, orientation: 1)
    defer { try? FileManager.default.removeItem(at: png) }
    let sips = Process()
    sips.executableURL = URL(fileURLWithPath: "/usr/bin/sips")
    sips.arguments = ["-s", "format", "heic", png.path, "--out", url.path]
    sips.standardOutput = FileHandle.nullDevice
    sips.standardError = FileHandle.nullDevice
    try sips.run()
    sips.waitUntilExit()
    guard sips.terminationStatus == 0 else { throw FixtureError.cannotWrite("sips heic") }
}

/// The type, size, orientation and GPS presence of an image file.
struct ImageFacts: Equatable {
    var type: String
    var width: Int
    var height: Int
    var orientation: Int
    var hasGPS: Bool
}

func imageFacts(_ url: URL) -> ImageFacts? {
    guard let source = CGImageSourceCreateWithURL(url as CFURL, nil),
          let type = CGImageSourceGetType(source) as String?,
          let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any]
    else {
        return nil
    }
    return ImageFacts(
        type: type, width: (properties[kCGImagePropertyPixelWidth] as? Int) ?? 0,
        height: (properties[kCGImagePropertyPixelHeight] as? Int) ?? 0,
        orientation: (properties[kCGImagePropertyOrientation] as? Int) ?? 1,
        hasGPS: properties[kCGImagePropertyGPSDictionary] != nil)
}

/// The mean absolute difference of the RGB values of two image files of the same size, decoded
/// upright into sRGB; small for a faithful JPEG, large when rows or bands are out of place.
func meanDifference(_ first: URL, _ second: URL) -> Double? {
    func pixels(_ url: URL) -> (width: Int, height: Int, bytes: [UInt8])? {
        guard let source = CGImageSourceCreateWithURL(url as CFURL, nil),
              let image = CGImageSourceCreateImageAtIndex(source, 0, nil)
        else {
            return nil
        }
        var bytes = [UInt8](repeating: 0, count: image.width * image.height * 4)
        let drawn = bytes.withUnsafeMutableBytes { buffer -> Bool in
            guard let context = CGContext(
                data: buffer.baseAddress, width: image.width, height: image.height, bitsPerComponent: 8,
                bytesPerRow: image.width * 4, space: CGColorSpace(name: CGColorSpace.sRGB)!,
                bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)
            else {
                return false
            }
            context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
            return true
        }
        return drawn ? (image.width, image.height, bytes) : nil
    }
    guard let a = pixels(first), let b = pixels(second), a.width == b.width, a.height == b.height else { return nil }
    var total = 0
    var count = 0
    for index in stride(from: 0, to: a.bytes.count, by: 4) {
        for channel in 0..<3 {
            total += abs(Int(a.bytes[index + channel]) - Int(b.bytes[index + channel]))
            count += 1
        }
    }
    return Double(total) / Double(max(1, count))
}

enum FixtureError: Error {
    case cannotWrite(String)
}

// MARK: Memory

/// What the memory probe measured while it loaded one file in its own process.
struct ProbeFigures {
    var name: String
    var mimeType: String
    var size: Int64
    /// Growth of the `phys_footprint` high-water mark over the probe's start, in bytes.
    var footprintGrowth: UInt64
    /// Growth of the resident high-water mark over the probe's start, in bytes.
    var residentGrowth: UInt64
    var footprintPeak: UInt64
    var residentMax: UInt64
}

/// Runs `ShareMemoryProbe` (built next to the test bundle) on `file`, staging into `folder`, so
/// the figures belong to that load alone and not to the tests running in this process.
func runMemoryProbe(_ file: URL, staging folder: URL) throws -> ProbeFigures {
    let products = Bundle(for: SheetHost.self).bundleURL.deletingLastPathComponent()
    let probe = Process()
    probe.executableURL = products.appendingPathComponent("ShareMemoryProbe")
    probe.arguments = [file.path, folder.path]
    let output = Pipe()
    probe.standardOutput = output
    probe.standardError = FileHandle.nullDevice
    try probe.run()
    let data = output.fileHandleForReading.readDataToEndOfFile()
    probe.waitUntilExit()
    guard probe.terminationStatus == 0 else { throw FixtureError.cannotWrite("probe exit \(probe.terminationStatus)") }
    var values: [String: String] = [:]
    for pair in String(decoding: data, as: UTF8.self).split(whereSeparator: \.isWhitespace) {
        let parts = pair.split(separator: "=", maxSplits: 1).map(String.init)
        if parts.count == 2 { values[parts[0]] = parts[1] }
    }
    func number(_ key: String) -> UInt64 { UInt64(values[key] ?? "") ?? 0 }
    let footprintPeak = number("footprint_peak")
    let residentMax = number("resident_max")
    return ProbeFigures(
        name: values["name"] ?? "", mimeType: values["mime"] ?? "", size: Int64(number("size")),
        footprintGrowth: footprintPeak - min(footprintPeak, number("footprint_before")),
        residentGrowth: residentMax - min(residentMax, number("resident_max_before")),
        footprintPeak: footprintPeak, residentMax: residentMax)
}

/// A size in MB for the printed figures.
func megabytes(_ bytes: UInt64) -> String {
    String(format: "%.1f MB", Double(bytes) / 1_048_576)
}

// MARK: Waiting

/// Records every wait of the view model and holds it until the test opens the gate; an event,
/// not a time. `reached(_:)` returns once the given number of waits has started.
final class Gate {
    private let lock = NSLock()
    private var isOpen = false
    private var waiters: [CheckedContinuation<Void, Never>] = []
    private var started: [ShareViewModel.Wait?] = []
    private var watchers: [(count: Int, continuation: CheckedContinuation<Void, Never>)] = []

    /// The view model's waits that started, in order.
    var waits: [ShareViewModel.Wait] {
        lock.lock()
        defer { lock.unlock() }
        return started.compactMap { $0 }
    }

    /// A wait of a fake (the send), recorded without a kind.
    func hold() async {
        await wait(nil)
    }

    /// The view model's wait: recorded, then held until `open`.
    func wait(_ kind: ShareViewModel.Wait?) async {
        let ready: [CheckedContinuation<Void, Never>] = lock.withLock {
            started.append(kind)
            let due = watchers.filter { $0.count <= started.count }
            watchers.removeAll { $0.count <= started.count }
            return due.map(\.continuation)
        }
        ready.forEach { $0.resume() }
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let resumeNow = lock.withLock { () -> Bool in
                if isOpen { return true }
                waiters.append(continuation)
                return false
            }
            if resumeNow { continuation.resume() }
        }
    }

    /// Returns once `count` waits have started.
    func reached(_ count: Int) async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let resumeNow = lock.withLock { () -> Bool in
                if started.count >= count { return true }
                watchers.append((count, continuation))
                return false
            }
            if resumeNow { continuation.resume() }
        }
    }

    /// Lets every held and every later wait go on.
    func open() {
        let held: [CheckedContinuation<Void, Never>] = lock.withLock {
            isOpen = true
            defer { waiters = [] }
            return waiters
        }
        held.forEach { $0.resume() }
    }
}

// MARK: Send and search

/// A send that records the drafts and answers with `outcome`, optionally held by a gate.
final class FakeSender: ShareSending {
    private let lock = NSLock()
    private var drafts: [ShareDraft] = []
    var outcome: ShareOutcome
    let hold: Gate?

    init(outcome: ShareOutcome, hold: Gate? = nil) {
        self.outcome = outcome
        self.hold = hold
    }

    var sent: [ShareDraft] {
        lock.lock()
        defer { lock.unlock() }
        return drafts
    }

    func send(_ draft: ShareDraft) async -> ShareOutcome {
        lock.withLock { drafts.append(draft) }
        if let hold { await hold.hold() }
        return outcome
    }
}

/// A search that records its queries and answers from a table.
final class FakeSearch: TargetSearching {
    private let lock = NSLock()
    private var asked: [String] = []
    let answers: [String: [ShareTarget]]
    let failing: Bool

    init(answers: [String: [ShareTarget]], failing: Bool = false) {
        self.answers = answers
        self.failing = failing
    }

    var queries: [String] {
        lock.lock()
        defer { lock.unlock() }
        return asked
    }

    func search(_ query: String) async throws -> [ShareTarget] {
        lock.withLock { asked.append(query) }
        if failing { throw ShareError.transient }
        return answers[query] ?? []
    }
}

// MARK: Attachments and staging

/// An attachment with fixed types and values; `file` is handed to `loadFile` as the system's
/// temporary copy, which only exists during the handler.
final class FakeAttachment: ShareAttachment {
    let typeIdentifiers: [String]
    let suggestedName: String?
    let file: URL?
    let value: ShareProvidedValue?
    private(set) var fileLoads: [String] = []
    private(set) var valueLoads: [String] = []

    init(types: [UTType], name: String? = nil, file: URL? = nil, value: ShareProvidedValue? = nil) {
        typeIdentifiers = types.map(\.identifier)
        suggestedName = name
        self.file = file
        self.value = value
    }

    func loadFile(forType type: String, _ handler: @escaping (Result<URL, Error>) -> Void) {
        fileLoads.append(type)
        guard let file else {
            handler(.failure(FixtureError.cannotWrite(type)))
            return
        }
        handler(.success(file))
    }

    func loadValue(forType type: String) async throws -> ShareProvidedValue {
        valueLoads.append(type)
        guard let value else { throw FixtureError.cannotWrite(type) }
        return value
    }
}

/// The live staging file system over a temporary folder, recording the coordinated reads and
/// the size of every chunk written.
final class RecordingStaging: ShareStagingFileSystem {
    let live: LiveStagingFileSystem
    let root: URL
    private(set) var folders: [URL] = []
    private(set) var coordinated: [URL] = []
    private(set) var chunks: [Int] = []

    init(root: URL, chunkBytes: Int = LiveStagingFileSystem.defaultChunkBytes) {
        self.root = root
        var made: [URL] = []
        live = LiveStagingFileSystem(makeFolder: {
            let folder = root.appendingPathComponent("ShareStaging/\(made.count + 1)", isDirectory: true)
            made.append(folder)
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            return folder
        }, chunkBytes: chunkBytes)
    }

    func makeFolder() throws -> URL {
        let folder = try live.makeFolder()
        folders.append(folder)
        return folder
    }

    func coordinatedRead<T>(_ url: URL, _ body: (URL) throws -> T) throws -> T {
        coordinated.append(url)
        return try live.coordinatedRead(url, body)
    }

    func copy(from source: URL, to destination: URL) throws {
        try live.copy(from: source, to: destination)
    }

    func write(_ data: Data, to destination: URL) throws {
        try live.write(data, to: destination, observe: { self.chunks.append($0) })
    }

    func size(of url: URL) throws -> Int64 {
        try live.size(of: url)
    }
}

/// A converter that always fails, so the loader's fallback to the unchanged file shows.
struct FailingConverter: ImageConverting {
    func convert(from source: URL, to destination: URL) throws {
        throw JPEGConverterError.unreadable
    }
}

/// A fresh temporary folder of one test; removed by the test's `defer`.
func temporaryFolder() throws -> URL {
    let folder = FileManager.default.temporaryDirectory.appendingPathComponent("clientkitshare-\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    return folder
}

// MARK: Targets

let instanceID = "0192f4a0-7b1c-7000-8000-00000000a11e"
let threadTarget = ShareTarget(kind: .thread, channelID: "c-1", threadID: "t-1", path: "server > Gateway: resume")
let forumTarget = ShareTarget(kind: .newPost, channelID: "c-2", threadID: nil, path: "requests")
let channelTarget = ShareTarget(kind: .channel, channelID: "c-3", threadID: "m-3", path: "general")

/// Share settings with `client.share.recent_targets` and the instance limits of a test.
func settings(recentMax: Int = 5, maxUploadBytes: Int64? = nil, maxAttachments: Int? = nil) -> ShareSettings {
    ShareSettings(instanceID: instanceID, recentMax: recentMax, maxUploadBytes: maxUploadBytes, maxAttachments: maxAttachments)
}

/// A staged item of `size` bytes; the file need not exist for the view model.
func item(_ name: String, size: Int64, mime: String = "image/jpeg") -> ShareItem {
    ShareItem(fileURL: URL(fileURLWithPath: "/staging/\(name)"), name: name, mimeType: mime, sizeBytes: size)
}

/// Strings of a test: the key itself, or a short template for the keys with arguments, so a
/// test reads what the sheet would show.
let testTemplates: [String: String] = [
    "SHARE_NEW_POST": "%1$@ (new post)",
    "SHARE_MORE_FILES_ONE": "and 1 more file",
    "SHARE_MORE_FILES_OTHER": "and %1$@ more files",
    "SHARE_FILE_TOO_LARGE": "%1$@ over %2$@",
    "SHARE_TOO_MANY_FILES": "at most %1$@",
    "SHARE_REFUSED": "Not sent: %1$@",
    "SHARE_REASON_STATUS": "error %1$@",
]
let testStrings = ShareStrings(lookup: { testTemplates[$0] ?? $0 })

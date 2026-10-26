// SPDX-License-Identifier: Apache-2.0
//
// Loading the shared items (brief M5-06 Design note 2; 15 12 and 14): files copied into a fresh
// staging folder of the App Group with coordinated reads, photos as JPEG, text and links into the
// message text, data written in chunks, nothing read into memory as a whole.

import ClientKit
import Foundation
import Testing
import UniformTypeIdentifiers

@testable import ClientKitShare

@Test func testFilesAreCopiedIntoOneFreshStagingFolder() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let pdf = root.appendingPathComponent("tmp-1234.pdf")
    try Data(repeating: 0x25, count: 3000).write(to: pdf)
    let png = root.appendingPathComponent("tmp-5678.png")
    try writeImage(png, type: .png, width: 40, height: 30, orientation: 1)
    let staging = RecordingStaging(root: root)

    let result = await ShareItemLoader(fileSystem: staging).load([
        FakeAttachment(types: [.pdf], name: "Report Q3.pdf", file: pdf),
        FakeAttachment(types: [.png], name: "screenshot", file: png),
    ])

    #expect(result.failures == 0)
    #expect(result.text.isEmpty)
    #expect(staging.folders.count == 1)
    let folder = try #require(staging.folders.first)
    #expect(result.items.map(\.name) == ["Report Q3.pdf", "screenshot.png"])
    #expect(result.items.map(\.mimeType) == ["application/pdf", "image/png"])
    #expect(result.items[0].sizeBytes == 3000)
    #expect(result.items[1].sizeBytes == Int64(try Data(contentsOf: png).count))
    for staged in result.items {
        #expect(staged.fileURL.deletingLastPathComponent().standardizedFileURL == folder.standardizedFileURL)
        #expect(FileManager.default.fileExists(atPath: staged.fileURL.path))
    }
    // A PNG stays unchanged: the server cleans it losslessly (12 7.1).
    #expect(try Data(contentsOf: result.items[1].fileURL) == Data(contentsOf: png))
    // Every read of a file of another process is coordinated.
    #expect(staging.coordinated.map(\.lastPathComponent) == ["tmp-1234.pdf", "tmp-5678.png"])
}

@Test func testHEICPhotoBecomesJPEG() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let heic = root.appendingPathComponent("provider.heic")
    try writeImage(heic, type: .heic, width: 64, height: 48)
    let staging = RecordingStaging(root: root)

    let result = await ShareItemLoader(fileSystem: staging).load([FakeAttachment(types: [.heic], name: "IMG_0001.HEIC", file: heic)])

    let photo = try #require(result.items.first)
    #expect(photo.name == "IMG_0001.jpg")
    #expect(photo.mimeType == "image/jpeg")
    #expect(imageFacts(photo.fileURL)?.type == UTType.jpeg.identifier)
    #expect(photo.sizeBytes == Int64(try Data(contentsOf: photo.fileURL).count))
}

@Test func testSystemJPEGIsPreferredOverHEIC() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let jpeg = root.appendingPathComponent("provider.jpg")
    try writeImage(jpeg, type: .jpeg, width: 20, height: 10)
    let attachment = FakeAttachment(types: [.heic, .jpeg], name: "IMG_0002", file: jpeg)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root)).load([attachment])

    // The system's own JPEG representation needs no decode on our side (15 14).
    #expect(attachment.fileLoads == [UTType.jpeg.identifier])
    #expect(result.items.map(\.name) == ["IMG_0002.jpg"])
    #expect(try Data(contentsOf: result.items[0].fileURL) == Data(contentsOf: jpeg))
}

@Test func testSystemJPEGOfAHEICNameIsNamedJPG() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let jpeg = root.appendingPathComponent("provider.jpg")
    try writeImage(jpeg, type: .jpeg, width: 8, height: 8)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root))
        .load([FakeAttachment(types: [.heic, .jpeg], name: "IMG_0004.HEIC", file: jpeg)])

    #expect(result.items.map(\.name) == ["IMG_0004.jpg"])
    #expect(result.items.map(\.mimeType) == ["image/jpeg"])
}

@Test func testFailedConversionSendsThePhotoUnchanged() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let heic = root.appendingPathComponent("provider.heic")
    try writeImage(heic, type: .heic, width: 16, height: 16)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root), converter: FailingConverter())
        .load([FakeAttachment(types: [.heic], name: "IMG_0003.HEIC", file: heic)])

    #expect(result.failures == 0)
    #expect(result.items.map(\.name) == ["IMG_0003.HEIC"])
    #expect(result.items.map(\.mimeType) == ["image/heic"])
}

@Test func testTextAndLinksBecomeMessageText() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let staging = RecordingStaging(root: root)

    let result = await ShareItemLoader(fileSystem: staging).load(
        [
            FakeAttachment(types: [.url], value: .url(URL(string: "https://chat.example.com/docs")!)),
            FakeAttachment(types: [.plainText], value: .text("See error above")),
        ],
        texts: ["Example page"])

    #expect(result.items.isEmpty)
    #expect(result.failures == 0)
    #expect(result.text == "Example page\nhttps://chat.example.com/docs\nSee error above")
    // No file, no staging folder.
    #expect(staging.folders.isEmpty)
}

@Test func testFileURLsAreCopiedWithCoordination() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let document = root.appendingPathComponent("Notes from Sam.txt")
    try Data("Agenda".utf8).write(to: document)
    let staging = RecordingStaging(root: root)

    let result = await ShareItemLoader(fileSystem: staging).load([
        FakeAttachment(types: [.fileURL], value: .url(document)),
        // A text type whose value is a file (a document shared from a file app).
        FakeAttachment(types: [.plainText], value: .url(document)),
    ])

    #expect(result.items.map(\.name) == ["Notes from Sam.txt", "Notes from Sam.txt"])
    #expect(result.items.map(\.mimeType) == ["text/plain", "text/plain"])
    #expect(Set(result.items.map(\.fileURL)).count == 2)
    #expect(staging.coordinated == [document, document])
    #expect(result.text.isEmpty)
}

@Test func testDataOnlyItemsAreWrittenInChunks() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let bytes = Data((0..<10_000).map { UInt8(truncatingIfNeeded: $0) })
    let staging = RecordingStaging(root: root, chunkBytes: 4096)

    let result = await ShareItemLoader(fileSystem: staging).load([
        FakeAttachment(types: [.pdf], name: "scan", value: .data(bytes)),
        FakeAttachment(types: [.plainText], value: .data(Data("plain".utf8))),
    ])

    #expect(result.items.map(\.name) == ["scan.pdf", "text.txt"])
    #expect(try Data(contentsOf: result.items[0].fileURL) == bytes)
    #expect(result.items[0].sizeBytes == 10_000)
    #expect(staging.chunks == [4096, 4096, 1808, 5])
}

@Test func testUnreadableItemsAreCountedNotHidden() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let folder = root.appendingPathComponent("A folder", isDirectory: true)
    try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root)).load([
        // Neither a file nor a value.
        FakeAttachment(types: [.pdf], name: "lost.pdf"),
        // A type the sheet cannot send.
        FakeAttachment(types: [UTType("com.example.unknown-thing") ?? .item]),
        // A folder is no file to upload.
        FakeAttachment(types: [.fileURL], value: .url(folder)),
        FakeAttachment(types: [.plainText], value: .text("kept")),
    ])

    #expect(result.failures == 3)
    #expect(result.items.isEmpty)
    #expect(result.text == "kept")
}

@Test func testNamesAreSafeInTheStagingFolder() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("x.pdf")
    try Data("x".utf8).write(to: file)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root)).load([
        FakeAttachment(types: [.pdf], name: "../../etc/passwd.pdf", file: file),
        FakeAttachment(types: [.pdf], name: ".hidden.pdf", file: file),
    ])

    // The shown name stays as the person knows it; the staged path never leaves the folder.
    #expect(result.items.map(\.name) == ["../../etc/passwd.pdf", ".hidden.pdf"])
    for staged in result.items {
        #expect(staged.fileURL.standardizedFileURL.path.hasPrefix(root.appendingPathComponent("ShareStaging/1/").standardizedFileURL.path))
        #expect(!staged.fileURL.lastPathComponent.hasPrefix("."))
    }
}

@Test func testRealItemProvidersLoad() async throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let file = root.appendingPathComponent("Plan.pdf")
    try Data(repeating: 0x41, count: 512).write(to: file)
    let fileProvider = try #require(NSItemProvider(contentsOf: file))
    let textProvider = NSItemProvider(item: "Hello from Alex" as NSString, typeIdentifier: UTType.plainText.identifier)
    let linkProvider = NSItemProvider(item: URL(string: "https://chat.example.com/t/1")! as NSURL, typeIdentifier: UTType.url.identifier)

    let result = await ShareItemLoader(fileSystem: RecordingStaging(root: root)).load(
        [fileProvider, textProvider, linkProvider].map(ItemProviderAttachment.init))

    #expect(result.failures == 0)
    #expect(result.items.map(\.name) == ["Plan.pdf"])
    #expect(result.items.first?.sizeBytes == 512)
    #expect(result.text == "Hello from Alex\nhttps://chat.example.com/t/1")
}

/// Copying a large file into the staging folder never holds it in memory (15 12); measured by
/// `ShareMemoryProbe` in its own process.
@Test func testLargeFileCopyMemory() throws {
    let root = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: root) }
    let large = root.appendingPathComponent("video.mov")
    let size = 256 * 1_048_576
    FileManager.default.createFile(atPath: large.path, contents: nil)
    let handle = try FileHandle(forWritingTo: large)
    let block = Data(repeating: 0x5a, count: 1_048_576)
    for _ in 0..<(size / block.count) { try handle.write(contentsOf: block) }
    try handle.close()

    let figures = try runMemoryProbe(large, staging: root.appendingPathComponent("staging", isDirectory: true))

    #expect(figures.name == "video.mov" && figures.size == Int64(size))
    print(
        "share-memory: copy of a \(size / 1_048_576) MB file; phys_footprint peak \(megabytes(figures.footprintPeak)) "
            + "(+\(megabytes(figures.footprintGrowth))), resident maximum \(megabytes(figures.residentMax)) (+\(megabytes(figures.residentGrowth)))")
    #expect(figures.footprintGrowth < UInt64(size / 8))
    #expect(figures.residentGrowth < UInt64(size / 8))
}

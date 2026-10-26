// SPDX-License-Identifier: Apache-2.0
//
// The JPEG conversion of photos (brief M5-06 Design note 2; 15 14; 12 7.1; ADR 0047): HEIC
// becomes JPEG with the orientation kept and the pixels decoded band by band, never as a whole.

import CoreGraphics
import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers

@testable import ClientKitShare

@Test func testOnlyHEIFPhotosNeedConversion() {
    #expect(JPEGConverter.needsConversion(.heic))
    #expect(JPEGConverter.needsConversion(.heif))
    // The server cleans these losslessly or re-encodes them itself (12 7.1): unchanged.
    for kept in [UTType.jpeg, .png, .gif, .webP, .svg, .pdf, .rawImage, .plainText, .mpeg4Movie] {
        #expect(!JPEGConverter.needsConversion(kept), "\(kept.identifier)")
    }
    // An image sequence (burst, animation) is a file the person sends on purpose.
    #expect(!JPEGConverter.needsConversion(UTType("public.heics")!))
}

@Test func testConvertsHEICToJPEGKeepingOrientation() throws {
    let folder = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: folder) }
    let source = folder.appendingPathComponent("IMG_0001.HEIC")
    try writeImage(source, type: .heic, width: 640, height: 480, orientation: 6)
    let output = folder.appendingPathComponent("IMG_0001.jpg")

    try JPEGConverter().convert(from: source, to: output)

    let facts = try #require(imageFacts(output))
    #expect(facts.type == UTType.jpeg.identifier)
    #expect(facts.width == 640 && facts.height == 480)
    // The pixels stay as stored and the orientation tag goes along (the server keeps it).
    #expect(facts.orientation == 6)
    // Nothing else of the original metadata: the location never leaves the device.
    #expect(!facts.hasGPS)
}

@Test func testConvertsPNGAndEveryBandSize() throws {
    let folder = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: folder) }
    let source = folder.appendingPathComponent("drawing.png")
    // A height that is no multiple of the band, so the last band is short.
    try writeImage(source, type: .png, width: 300, height: 257, orientation: 1)
    for rows in [1, 16, 256, 1000] {
        let output = folder.appendingPathComponent("drawing-\(rows).jpg")
        try JPEGConverter(bandRows: rows).convert(from: source, to: output)
        let facts = try #require(imageFacts(output))
        #expect(facts == ImageFacts(type: UTType.jpeg.identifier, width: 300, height: 257, orientation: 1, hasGPS: false))
        // Every band lands at its rows: the pixels match the source up to the JPEG's loss.
        let difference = try #require(meanDifference(source, output))
        #expect(difference < 8, "band rows \(rows): mean difference \(difference)")
    }
}

@Test func testConversionOfANonImageFails() throws {
    let folder = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: folder) }
    let source = folder.appendingPathComponent("note.heic")
    try Data("not an image".utf8).write(to: source)
    let output = folder.appendingPathComponent("note.jpg")
    #expect(throws: JPEGConverterError.unreadable) { try JPEGConverter().convert(from: source, to: output) }
    #expect(!FileManager.default.fileExists(atPath: output.path))
}

/// The resident memory of converting a large photo stays far below its decoded size, because the
/// pixels go to the encoder band by band (brief M5-06 Design note 2: record the measured peak).
/// Measured by `ShareMemoryProbe` in its own process, through the loader as the extension runs it.
@Test func testLargePhotoConversionMemoryStaysFarBelowDecodedSize() throws {
    let folder = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: folder) }
    let side = 8000
    let decodedBytes = UInt64(side * side * 4)
    let source = folder.appendingPathComponent("large.heic")
    try writeLargeHEIC(source, side: side)

    let figures = try runMemoryProbe(source, staging: folder.appendingPathComponent("staging", isDirectory: true))

    #expect(figures.name == "large.jpg" && figures.mimeType == "image/jpeg")
    let output = folder.appendingPathComponent("staging/1-large.jpg")
    #expect(imageFacts(output).map { $0.width == side && $0.height == side } == true)
    print(
        "share-memory: HEIC \(side)x\(side) to JPEG, decoded \(megabytes(decodedBytes)); "
            + "phys_footprint peak \(megabytes(figures.footprintPeak)) (+\(megabytes(figures.footprintGrowth))), "
            + "resident maximum \(megabytes(figures.residentMax)) (+\(megabytes(figures.residentGrowth)))")
    // A whole decode into the process adds at least the decoded size; banding keeps the
    // resident pixels below half of it. The footprint is recorded, not asserted: it also counts
    // the decoder's own surfaces, which depend on the system's HEIC decoder (about 64 MB on
    // macOS 26, about 282 MB on the macOS 15 CI runner for this image; docs/decisions.d/M5-06.md
    // D-M5-06-2), so a bound on it would test the host, not the converter.
    #expect(figures.residentGrowth < decodedBytes / 2)
}

@Test func testThumbnailIsSmallAndUpright() throws {
    let folder = try temporaryFolder()
    defer { try? FileManager.default.removeItem(at: folder) }
    let source = folder.appendingPathComponent("wide.heic")
    try writeImage(source, type: .heic, width: 400, height: 200, orientation: 6)
    // Orientation 6 turns the stored 400 x 200 pixels upright into 200 x 400.
    let thumbnail = try #require(ShareThumbnail.make(source, maxPixelSize: 100))
    #expect(thumbnail.width == 50 && thumbnail.height == 100)
    #expect(ShareThumbnail.make(folder.appendingPathComponent("missing.png"), maxPixelSize: 100) == nil)
}

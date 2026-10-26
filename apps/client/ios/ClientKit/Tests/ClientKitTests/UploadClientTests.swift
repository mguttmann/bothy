// SPDX-License-Identifier: Apache-2.0
//
// The upload protocol of docs/spec/12-files.md 3 in the Share Extension (brief M5-06 Design
// note 4; 15 12, 14): the file is hashed and sent part by part from disk, never whole in memory.

import CryptoKit
import Foundation
import Testing

@testable import ClientKit

@Test func testUploadStreamsFromFile() async throws {
    let world = ShareWorld()
    world.chat.partSize = 1024
    // Several parts, the last one shorter.
    let bytes = pattern(4 * 1024 + 300)
    let item = try world.stage("report.pdf", bytes: bytes, mime: "application/pdf")
    let uploads = world.uploader(hashChunkBytes: 256)

    let fileID = try await uploads.upload(item, channelID: channelID)

    let create = try #require(world.chat.requests("POST", "/channels/\(channelID)/uploads").first)
    let declared = body(create)
    #expect(declared["name"] as? String == "report.pdf")
    #expect(declared["size_bytes"] as? Int == bytes.count)
    #expect(declared["sha256"] as? String == hex(bytes))
    #expect(declared["declared_mime_type"] as? String == "application/pdf")
    #expect(create.value(forHTTPHeaderField: "Idempotency-Key")?.count == 36)

    // Parts in order, each with its digest and the listed header, each no larger than a part.
    let parts = world.chat.requests.filter { $0.httpMethod == "PUT" }
    #expect(parts.map { $0.url!.lastPathComponent } == ["1", "2", "3", "4", "5"])
    for (index, part) in parts.enumerated() {
        let expected = bytes.subdata(in: (index * 1024)..<min(bytes.count, (index + 1) * 1024))
        #expect(part.httpBody == expected)
        #expect(part.value(forHTTPHeaderField: "Content-Digest") == "sha-256=:\(Data(SHA256.hash(data: expected)).base64EncodedString()):")
        #expect(part.value(forHTTPHeaderField: "Content-Type") == "application/octet-stream")
        #expect(part.value(forHTTPHeaderField: "Authorization") == "DPoP \(childAccess)")
        #expect(part.url?.absoluteString.hasPrefix("https://chat.example.com/api/v1/uploads/") == true)
    }
    // Memory: no read and no body is larger than the part size, whatever the file size.
    #expect(world.reader.largest <= 1024)
    #expect(world.chat.requests.map { $0.httpBody?.count ?? 0 }.max()! <= 1024)

    let complete = try #require(world.chat.requests("POST", "/uploads/\(fileID)/complete").first)
    let listed = try #require(body(complete)["parts"] as? [[String: Any]])
    #expect(listed.compactMap { $0["part_number"] as? Int } == [1, 2, 3, 4, 5])
    #expect(listed.compactMap { $0["etag"] as? String } == (1...5).compactMap { world.chat.confirmed(fileID)[$0] })
    #expect(complete.value(forHTTPHeaderField: "Idempotency-Key") != create.value(forHTTPHeaderField: "Idempotency-Key"))

    // Resumption: part 3 fails once; `getUpload` names the confirmed parts and only the others
    // are sent again.
    let resumed = ShareWorld()
    resumed.chat.partSize = 1024
    var failed = false
    resumed.chat.override = { request in
        guard request.httpMethod == "PUT", request.url?.lastPathComponent == "3", !failed else { return nil }
        failed = true
        return .init(status: 503)
    }
    let again = try resumed.stage("report.pdf", bytes: bytes)
    let resumedID = try await resumed.uploader().upload(again, channelID: channelID)
    let sent = resumed.chat.requests.filter { $0.httpMethod == "PUT" || $0.httpMethod == "GET" }
        .map { "\($0.httpMethod!) \($0.url!.lastPathComponent)" }
    #expect(sent == ["PUT 1", "PUT 2", "PUT 3", "GET \(resumedID)", "PUT 3", "PUT 4", "PUT 5"])
    #expect(resumed.chat.requests("POST", "/uploads/\(resumedID)/complete").count == 1)
}

@Test func testResumptionSendsOnlyPartsTheServerLacks() async throws {
    // The answer of the failed part was lost although the server stored it: `getUpload` confirms
    // part 2, so it is not sent again.
    let world = ShareWorld()
    world.chat.partSize = 1024
    var lost = false
    world.chat.override = { request in
        guard request.httpMethod == "PUT", request.url?.lastPathComponent == "2", !lost else { return nil }
        lost = true
        // Let the route store the part, then fail the answer, as a dropped connection does.
        _ = await world.chat.answer(request)
        throw URLError(.networkConnectionLost)
    }
    let item = try world.stage("a.bin", bytes: pattern(3 * 1024))
    let fileID = try await world.uploader().upload(item, channelID: channelID)
    let sent = world.chat.requests.filter { $0.httpMethod == "PUT" }.map { $0.url!.lastPathComponent }
    #expect(sent == ["1", "2", "3"])
    #expect(world.chat.confirmed(fileID).count == 3)

    // A second failure after the resumption ends the upload as transient.
    let flaky = ShareWorld()
    flaky.chat.partSize = 1024
    flaky.chat.override = { request in request.httpMethod == "PUT" && request.url?.lastPathComponent == "2" ? .init(status: 502) : nil }
    let other = try flaky.stage("a.bin", bytes: pattern(3 * 1024))
    await #expect(throws: ShareError.transient) { try await flaky.uploader().upload(other, channelID: channelID) }
    #expect(flaky.chat.requests("GET", "").count == 1)
}

@Test func testPresignedPartsCarryNoToken() async throws {
    let world = ShareWorld()
    world.chat.partSize = 1024
    world.chat.mode = "presigned"
    let bytes = pattern(2 * 1024 + 10)
    let item = try world.stage("clip.mov", bytes: bytes, mime: "video/quicktime")
    let fileID = try await world.uploader().upload(item, channelID: channelID)

    let parts = world.chat.requests.filter { $0.httpMethod == "PUT" }
    #expect(parts.count == 3)
    for part in parts {
        #expect(part.url?.host == "objects.example.com")
        #expect(part.value(forHTTPHeaderField: "Authorization") == nil)
        #expect(part.value(forHTTPHeaderField: "DPoP") == nil)
        #expect(part.value(forHTTPHeaderField: "Content-Digest")?.hasPrefix("sha-256=:") == true)
    }
    let complete = try #require(world.chat.requests("POST", "/uploads/\(fileID)/complete").first)
    let etags = (body(complete)["parts"] as? [[String: Any]])?.compactMap { $0["etag"] as? String }
    #expect(etags == ["\"s3-1\"", "\"s3-2\"", "\"s3-3\""])
}

@Test func testUploadRefusalCarriesStatusAndCode() async throws {
    let world = ShareWorld()
    world.chat.override = { request in
        request.url?.path.hasSuffix("/uploads") == true ? problem(413, "payload_too_large") : nil
    }
    let item = try world.stage("big.bin", bytes: pattern(10))
    await #expect(throws: ShareError.refused(status: 413, code: "payload_too_large")) {
        try await world.uploader().upload(item, channelID: channelID)
    }
}

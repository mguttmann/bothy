import Foundation
import HelperCore
import XCTest

/// The channel codec against the shared vectors of 27 9.4 (brief item 14).
final class ChannelCodecTests: XCTestCase {
    struct Vector {
        let name: String
        let type: Int
        let object: JSONValue?
        let bytes: Data?
        let frame: Data
    }

    /// Reads `services/host/testdata/computer/channel-vectors.json` relative to this file, so the
    /// test reads the one file every codec of the project reads.
    static func vectors() throws -> [Vector] {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<5 { url.deleteLastPathComponent() }  // up to services/host
        url.appendPathComponent("testdata/computer/channel-vectors.json")
        let root = try JSONValue.parse(Data(contentsOf: url))
        guard let list = root["vectors"]?.arrayValue else {
            throw XCTSkip("vectors file has no list")
        }
        return try list.map { item in
            Vector(
                name: item["name"]?.stringValue ?? "?",
                type: Int(item["type"]?.intValue ?? 0),
                object: item["object"],
                bytes: try item["bytes_hex"]?.stringValue.map(hexData),
                frame: try hexData(XCTUnwrap(item["frame_hex"]?.stringValue)))
        }
    }

    static func hexData(_ hex: String) throws -> Data {
        var data = Data()
        var index = hex.startIndex
        while index < hex.endIndex {
            let next = hex.index(index, offsetBy: 2)
            guard let byte = UInt8(hex[index..<next], radix: 16) else {
                throw ChannelError.invalidJSON
            }
            data.append(byte)
            index = next
        }
        return data
    }

    let codec = FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes)

    func testEveryVectorDecodesToItsObjectOrBytes() throws {
        let vectors = try Self.vectors()
        XCTAssertGreaterThanOrEqual(vectors.count, 19)
        for vector in vectors {
            let frame = try codec.decodeSingle(vector.frame)
            switch vector.type {
            case 1: XCTAssertEqual(frame, .message(try XCTUnwrap(vector.object)), vector.name)
            case 2: XCTAssertEqual(frame, .image(try XCTUnwrap(vector.bytes)), vector.name)
            default: XCTFail("unknown type in \(vector.name)")
            }
        }
    }

    func testOwnOutputDecodesBackEqual() throws {
        for vector in try Self.vectors() {
            let frame: Frame =
                vector.type == 1
                ? .message(try XCTUnwrap(vector.object)) : .image(try XCTUnwrap(vector.bytes))
            let encoded = try codec.encode(frame)
            XCTAssertEqual(try codec.decodeSingle(encoded), frame, vector.name)
        }
    }

    func testImageVectorEncodesByteExact() throws {
        let vector = try XCTUnwrap(Self.vectors().first { $0.type == 2 })
        XCTAssertEqual(try codec.encode(.image(try XCTUnwrap(vector.bytes))), vector.frame)
    }

    func testStreamDecoderReadsEveryVectorFromSmallChunks() throws {
        let vectors = try Self.vectors()
        var stream = Data()
        for vector in vectors { stream.append(vector.frame) }
        var decoder = FrameDecoder(maxFrameBytes: HostConfig.defaultFrameMaxBytes)
        var frames: [Frame] = []
        var offset = 0
        while offset < stream.count {
            let end = min(offset + 7, stream.count)
            decoder.append(stream.subdata(in: offset..<end))
            while let frame = try decoder.next() { frames.append(frame) }
            offset = end
        }
        XCTAssertEqual(frames.count, vectors.count)
    }

    func testFrameOverTheLimitIsRefused() throws {
        let hello = try XCTUnwrap(Self.vectors().first { $0.name == "hello" })
        let length = hello.frame.count - 4
        let exact = FrameCodec(maxFrameBytes: length)
        XCTAssertNoThrow(try exact.decodeSingle(hello.frame))
        let small = FrameCodec(maxFrameBytes: length - 1)
        XCTAssertThrowsError(try small.decodeSingle(hello.frame)) { error in
            XCTAssertEqual(error as? ChannelError, .frameTooLarge(length: length, limit: length - 1))
        }
        XCTAssertThrowsError(try small.encode(.message(try XCTUnwrap(hello.object))))
        // The stream decoder refuses on the length alone, before the content arrives.
        var decoder = FrameDecoder(maxFrameBytes: 8)
        decoder.append(Data([0, 0, 0, 9]))
        XCTAssertThrowsError(try decoder.next())
        XCTAssertThrowsError(try FrameCodec(maxFrameBytes: 4).encode(.image(Data(count: 4))))
    }

    func testMalformedFramesAreRefused() {
        XCTAssertThrowsError(try codec.decodeSingle(Data([0, 0, 0, 0])))  // no type byte
        XCTAssertThrowsError(try codec.decodeSingle(Data([0, 0, 0, 1, 7])))  // unknown type
        XCTAssertThrowsError(try codec.decodeSingle(Data([0, 0, 0, 3, 1, 0x5B, 0x5D])))  // array
        XCTAssertThrowsError(try codec.decodeSingle(Data([0, 0, 0, 2, 1, 0x7B])))  // broken JSON
        XCTAssertThrowsError(try codec.decodeSingle(Data([0, 0, 0, 5, 2])))  // truncated
    }

    func testIntegersAndFractionsStayApart() throws {
        let value = try JSONValue.parse(Data(#"{"a":400,"b":640.5,"c":true}"#.utf8))
        XCTAssertEqual(value["a"], .int(400))
        XCTAssertEqual(value["b"], .double(640.5))
        XCTAssertEqual(value["c"], .bool(true))
        XCTAssertEqual(JSONValue.double(400), JSONValue.int(400))
        XCTAssertNotEqual(JSONValue.bool(true), JSONValue.int(1))
    }
}

/// The frame limit comes from the host service's configuration file (D-CU-02-8).
final class HostConfigTests: XCTestCase {
    func testReadsTheKeyFromTheHostServiceSection() {
        let yaml = """
            # host configuration
            server:
              url: https://bothy.example.com
            host_service:
              session_users: [alex]
              computer_frame_max_bytes: 1048576 # one MiB
              computer_use: true
            """
        XCTAssertEqual(HostConfig.frameMaxBytes(fromYAML: yaml), 1_048_576)
    }

    func testFallsBackToTheDefaultOfChapter27() {
        XCTAssertEqual(HostConfig.defaultFrameMaxBytes, 67_108_864)
        XCTAssertEqual(HostConfig.frameMaxBytes(fromYAML: ""), HostConfig.defaultFrameMaxBytes)
        let other = "other:\n  computer_frame_max_bytes: 5\nhost_service:\n  computer_use: true\n"
        XCTAssertEqual(HostConfig.frameMaxBytes(fromYAML: other), HostConfig.defaultFrameMaxBytes)
        let bad = "host_service:\n  computer_frame_max_bytes: lots\n"
        XCTAssertEqual(HostConfig.frameMaxBytes(fromYAML: bad), HostConfig.defaultFrameMaxBytes)
        XCTAssertEqual(
            HostConfig.frameMaxBytes(configPath: "/nonexistent/config.yaml"),
            HostConfig.defaultFrameMaxBytes)
    }
}

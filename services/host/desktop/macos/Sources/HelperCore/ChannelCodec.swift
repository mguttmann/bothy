// SPDX-License-Identifier: Apache-2.0

import Foundation

/// One frame of the helper channel (docs/spec/27-computer-use.md section 9.4).
public enum Frame: Equatable, Sendable {
    /// Type `0x01`: one UTF-8 JSON object.
    case message(JSONValue)
    /// Type `0x02`: the bytes of one image, sent directly after the `done` that announces it.
    case image(Data)
}

/// Why a frame could not be read or written.
public enum ChannelError: Error, Equatable {
    /// The length field (type byte and content) exceeds `host_service.computer_frame_max_bytes`.
    case frameTooLarge(length: Int, limit: Int)
    /// A length of zero: a frame needs at least its type byte.
    case emptyFrame
    /// A type byte other than `0x01` or `0x02`.
    case unknownType(UInt8)
    /// The content of a `0x01` frame is not valid UTF-8 JSON.
    case invalidJSON
    /// The content of a `0x01` frame is valid JSON but not an object.
    case notAnObject
    /// The bytes end before the frame does, or more bytes follow a single frame.
    case truncated
}

/// Encodes and decodes whole frames: a 4-byte big-endian length of type byte and content, the
/// type byte and the content.
///
/// The limit applies to the length field, as in 27 9.4, so it bounds the memory one frame of the
/// peer can make the helper allocate.
///
/// Example:
/// ```swift
/// let codec = FrameCodec(maxFrameBytes: HostConfig.defaultFrameMaxBytes)
/// let bytes = try codec.encode(.message(["op": "release"]))
/// ```
public struct FrameCodec: Sendable {
    /// The longest length field accepted, from `host_service.computer_frame_max_bytes`.
    public let maxFrameBytes: Int

    /// Creates a codec.
    /// - Parameter maxFrameBytes: the value of `host_service.computer_frame_max_bytes`.
    public init(maxFrameBytes: Int) {
        self.maxFrameBytes = maxFrameBytes
    }

    /// Encodes one frame.
    ///
    /// - Parameter frame: the message or image.
    /// - Returns: the bytes of the frame, length field included.
    /// - Throws: `ChannelError.frameTooLarge` when type byte and content exceed the limit, and
    ///   `JSONValue.Failure` for a message JSON cannot carry.
    public func encode(_ frame: Frame) throws -> Data {
        let type: UInt8
        let content: Data
        switch frame {
        case .message(let object):
            guard case .object = object else { throw ChannelError.notAnObject }
            type = 0x01
            content = try object.serialized()
        case .image(let bytes):
            type = 0x02
            content = bytes
        }
        let length = content.count + 1
        guard length <= maxFrameBytes, length <= Int(UInt32.max) else {
            throw ChannelError.frameTooLarge(length: length, limit: maxFrameBytes)
        }
        var data = Data(capacity: length + 4)
        withUnsafeBytes(of: UInt32(length).bigEndian) { data.append(contentsOf: $0) }
        data.append(type)
        data.append(content)
        return data
    }

    /// Decodes exactly one frame.
    ///
    /// - Parameter data: the bytes of one frame, length field included, nothing after it.
    /// - Returns: the frame.
    /// - Throws: a `ChannelError` for a malformed, unknown, truncated or too large frame.
    public func decodeSingle(_ data: Data) throws -> Frame {
        var decoder = FrameDecoder(maxFrameBytes: maxFrameBytes)
        decoder.append(data)
        guard let frame = try decoder.next(), decoder.bufferedByteCount == 0 else {
            throw ChannelError.truncated
        }
        return frame
    }
}

/// Reads frames from a byte stream that arrives in arbitrary pieces.
///
/// A too large length is refused as soon as the length field is complete, so the helper never
/// buffers more than one allowed frame of the peer.
///
/// Example:
/// ```swift
/// var decoder = FrameDecoder(maxFrameBytes: limit)
/// decoder.append(chunk)
/// while let frame = try decoder.next() { handle(frame) }
/// ```
public struct FrameDecoder: Sendable {
    /// The longest length field accepted, from `host_service.computer_frame_max_bytes`.
    public let maxFrameBytes: Int
    private var buffer = Data()

    /// Creates an empty decoder.
    /// - Parameter maxFrameBytes: the value of `host_service.computer_frame_max_bytes`.
    public init(maxFrameBytes: Int) {
        self.maxFrameBytes = maxFrameBytes
    }

    /// The number of bytes received but not yet returned as a frame.
    public var bufferedByteCount: Int { buffer.count }

    /// Adds received bytes.
    /// - Parameter data: the next bytes of the stream.
    public mutating func append(_ data: Data) {
        buffer.append(data)
    }

    /// Returns the next complete frame.
    ///
    /// - Returns: the frame, or nil while the buffered bytes do not hold a complete one.
    /// - Throws: a `ChannelError`; after an error the stream is unusable and the connection
    ///   must be closed.
    public mutating func next() throws -> Frame? {
        guard buffer.count >= 4 else { return nil }
        let start = buffer.startIndex
        let length =
            Int(buffer[start]) << 24 | Int(buffer[start + 1]) << 16 | Int(buffer[start + 2]) << 8
            | Int(buffer[start + 3])
        guard length > 0 else { throw ChannelError.emptyFrame }
        guard length <= maxFrameBytes else {
            throw ChannelError.frameTooLarge(length: length, limit: maxFrameBytes)
        }
        guard buffer.count >= 4 + length else { return nil }
        let type = buffer[start + 4]
        let content = buffer.subdata(in: (start + 5)..<(start + 4 + length))
        buffer = buffer.subdata(in: (start + 4 + length)..<buffer.endIndex)
        switch type {
        case 0x01:
            let value: JSONValue
            do {
                value = try JSONValue.parse(content)
            } catch {
                throw ChannelError.invalidJSON
            }
            guard case .object = value else { throw ChannelError.notAnObject }
            return .message(value)
        case 0x02:
            return .image(content)
        default:
            throw ChannelError.unknownType(type)
        }
    }
}

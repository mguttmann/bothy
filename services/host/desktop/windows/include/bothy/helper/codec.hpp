// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The frame codec of the local channel between the host service and the helper
/// (docs/spec/27-computer-use.md section 9.4): a 4-byte big-endian length that counts the type byte
/// and the content, one type byte, and the content. Type 0x01 carries one UTF-8 JSON object, type
/// 0x02 the bytes of one image. The length is at most `host_service.computer_frame_max_bytes`
/// (D-CU-02-8).

#include <cstddef>
#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "bothy/helper/json.hpp"

namespace bothy::helper {

/// Type byte of a frame that carries one JSON object.
inline constexpr std::uint8_t kFrameObject = 0x01;
/// Type byte of a frame that carries the bytes of one image.
inline constexpr std::uint8_t kFrameImage = 0x02;

/// The default of `host_service.computer_frame_max_bytes` from docs/spec/27-computer-use.md
/// section 18, used when the host service's configuration file is not readable or does not set the
/// key (D-CU-02-8). Keep it equal to the specification's default.
inline constexpr std::uint64_t kDefaultFrameMaxBytes = 67108864;

/// One decoded frame.
struct Frame {
  /// kFrameObject or kFrameImage.
  std::uint8_t type = kFrameObject;
  /// The object of a kFrameObject frame; null for an image frame.
  json::Value object;
  /// The bytes of a kFrameImage frame; empty for an object frame.
  std::vector<std::uint8_t> bytes;
};

/// Why a frame could not be read. Every error ends the connection: after a bad frame the reader
/// cannot know where the next one starts.
enum class CodecError {
  None,         ///< no error
  Empty,        ///< a length of 0 (a frame needs at least its type byte)
  TooLarge,     ///< a length above the frame limit
  UnknownType,  ///< a type byte other than 0x01 and 0x02
  BadObject,    ///< type 0x01 whose content is not one valid UTF-8 JSON object
};

/// @return a stable lower-case name of an error for logs ("too_large", ...).
const char* codec_error_name(CodecError error);

/// Encodes one JSON object frame.
/// @param object the object (must be a JSON object).
/// @param max_length the frame limit (`host_service.computer_frame_max_bytes`).
/// @return the frame bytes, or std::nullopt when the value is not an object or the frame would
///         exceed the limit.
std::optional<std::vector<std::uint8_t>> encode_object(const json::Value& object,
                                                       std::uint64_t max_length);

/// Encodes one image frame.
/// @param bytes the encoded image (JPEG or PNG).
/// @param max_length the frame limit.
/// @return the frame bytes, or std::nullopt when the frame would exceed the limit.
std::optional<std::vector<std::uint8_t>> encode_image(const std::vector<std::uint8_t>& bytes,
                                                      std::uint64_t max_length);

/// @return true when a frame with `content_size` bytes of content fits the limit.
/// @param content_size the size of the content without length and type byte.
/// @param max_length the frame limit.
bool frame_fits(std::uint64_t content_size, std::uint64_t max_length);

/// Incremental frame reader for a byte stream. It refuses an oversized frame as soon as the length
/// prefix arrives, before any buffer for the content is reserved, so a peer cannot make the helper
/// allocate more than the limit.
///
/// Example:
/// @code
///   FrameReader reader(kDefaultFrameMaxBytes);
///   std::vector<Frame> frames;
///   if (reader.feed(data, size, frames) != CodecError::None) { /* close the connection */ }
/// @endcode
class FrameReader {
 public:
  /// @param max_length the frame limit (`host_service.computer_frame_max_bytes`).
  explicit FrameReader(std::uint64_t max_length);

  /// Feeds bytes read from the stream.
  /// @param data the bytes.
  /// @param size their number.
  /// @param out receives every frame completed by these bytes, in order.
  /// @return CodecError::None, or the first error; after an error the reader stays failed and
  ///         returns the same error for every later call.
  CodecError feed(const std::uint8_t* data, std::size_t size, std::vector<Frame>& out);

  /// @return true when no partial frame is buffered (a clean place to end the stream).
  bool idle() const;

 private:
  CodecError finish_frame(std::vector<Frame>& out);

  std::uint64_t max_length_;
  std::uint8_t header_[4] = {0, 0, 0, 0};
  std::size_t header_have_ = 0;
  std::uint64_t length_ = 0;
  std::vector<std::uint8_t> body_;
  CodecError failed_ = CodecError::None;
};

/// Decodes exactly one complete frame; trailing bytes are an error.
/// @param bytes the frame bytes.
/// @param max_length the frame limit.
/// @param error receives the reason on failure; may be null. Trailing or missing bytes report
///        CodecError::BadObject for want of a better class; tests use it only on whole frames.
/// @return the frame, or std::nullopt.
std::optional<Frame> decode_frame(const std::vector<std::uint8_t>& bytes, std::uint64_t max_length,
                                  CodecError* error = nullptr);

}  // namespace bothy::helper

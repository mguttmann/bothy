// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/codec.hpp"

#include <string_view>

namespace bothy::helper {

namespace {

// The length prefix is 32 bits; a limit above it could never be reached.
constexpr std::uint64_t kLengthCeiling = 0xFFFFFFFFu;

std::vector<std::uint8_t> frame_bytes(std::uint8_t type, const std::uint8_t* content,
                                      std::size_t size) {
  std::uint64_t length = static_cast<std::uint64_t>(size) + 1;
  std::vector<std::uint8_t> out;
  out.reserve(size + 5);
  out.push_back(static_cast<std::uint8_t>(length >> 24));
  out.push_back(static_cast<std::uint8_t>(length >> 16));
  out.push_back(static_cast<std::uint8_t>(length >> 8));
  out.push_back(static_cast<std::uint8_t>(length));
  out.push_back(type);
  out.insert(out.end(), content, content + size);
  return out;
}

}  // namespace

const char* codec_error_name(CodecError error) {
  switch (error) {
    case CodecError::None: return "none";
    case CodecError::Empty: return "empty";
    case CodecError::TooLarge: return "too_large";
    case CodecError::UnknownType: return "unknown_type";
    case CodecError::BadObject: return "bad_object";
  }
  return "unknown";
}

bool frame_fits(std::uint64_t content_size, std::uint64_t max_length) {
  std::uint64_t limit = max_length < kLengthCeiling ? max_length : kLengthCeiling;
  return content_size < limit;  // the length adds the type byte
}

std::optional<std::vector<std::uint8_t>> encode_object(const json::Value& object,
                                                       std::uint64_t max_length) {
  if (!object.is_object()) return std::nullopt;
  std::string text = json::serialize(object);
  if (!frame_fits(text.size(), max_length)) return std::nullopt;
  return frame_bytes(kFrameObject, reinterpret_cast<const std::uint8_t*>(text.data()), text.size());
}

std::optional<std::vector<std::uint8_t>> encode_image(const std::vector<std::uint8_t>& bytes,
                                                      std::uint64_t max_length) {
  if (!frame_fits(bytes.size(), max_length)) return std::nullopt;
  return frame_bytes(kFrameImage, bytes.data(), bytes.size());
}

FrameReader::FrameReader(std::uint64_t max_length) : max_length_(max_length) {}

bool FrameReader::idle() const { return header_have_ == 0 && body_.empty() && failed_ == CodecError::None; }

CodecError FrameReader::finish_frame(std::vector<Frame>& out) {
  Frame f;
  f.type = body_[0];
  if (f.type == kFrameImage) {
    f.bytes.assign(body_.begin() + 1, body_.end());
  } else if (f.type == kFrameObject) {
    std::string_view text(reinterpret_cast<const char*>(body_.data()) + 1, body_.size() - 1);
    std::optional<json::Value> v = json::parse(text);
    if (!v || !v->is_object()) return CodecError::BadObject;
    f.object = std::move(*v);
  } else {
    return CodecError::UnknownType;
  }
  out.push_back(std::move(f));
  body_.clear();
  header_have_ = 0;
  return CodecError::None;
}

CodecError FrameReader::feed(const std::uint8_t* data, std::size_t size, std::vector<Frame>& out) {
  std::size_t i = 0;
  while (failed_ == CodecError::None && i < size) {
    if (header_have_ < 4) {
      header_[header_have_++] = data[i++];
      if (header_have_ < 4) continue;
      length_ = (static_cast<std::uint64_t>(header_[0]) << 24) |
                (static_cast<std::uint64_t>(header_[1]) << 16) |
                (static_cast<std::uint64_t>(header_[2]) << 8) | header_[3];
      if (length_ == 0) {
        failed_ = CodecError::Empty;
      } else if (length_ > max_length_) {
        failed_ = CodecError::TooLarge;
      } else {
        body_.reserve(static_cast<std::size_t>(length_));
      }
      continue;
    }
    std::size_t want = static_cast<std::size_t>(length_) - body_.size();
    std::size_t take = size - i < want ? size - i : want;
    body_.insert(body_.end(), data + i, data + i + take);
    i += take;
    if (body_.size() == length_) failed_ = finish_frame(out);
  }
  return failed_;
}

std::optional<Frame> decode_frame(const std::vector<std::uint8_t>& bytes, std::uint64_t max_length,
                                  CodecError* error) {
  FrameReader reader(max_length);
  std::vector<Frame> frames;
  CodecError e = reader.feed(bytes.data(), bytes.size(), frames);
  if (e == CodecError::None && (frames.size() != 1 || !reader.idle())) e = CodecError::BadObject;
  if (error != nullptr) *error = e;
  if (e != CodecError::None) return std::nullopt;
  return std::move(frames.front());
}

}  // namespace bothy::helper

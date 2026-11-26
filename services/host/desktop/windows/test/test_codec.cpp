// SPDX-License-Identifier: Apache-2.0

// Tests of the channel codec against the shared vectors (brief CU-02 item 14).

#include <cstdint>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>

#include "bothy/helper/codec.hpp"
#include "bothy/helper/json.hpp"
#include "check.hpp"

using namespace bothy::helper;

#ifndef CHANNEL_VECTORS_PATH
#error "CMake passes CHANNEL_VECTORS_PATH, the path of channel-vectors.json"
#endif

namespace {

std::vector<std::uint8_t> from_hex(const std::string& hex) {
  std::vector<std::uint8_t> out;
  auto nibble = [](char c) -> int {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
  };
  for (std::size_t i = 0; i + 1 < hex.size(); i += 2) {
    int hi = nibble(hex[i]);
    int lo = nibble(hex[i + 1]);
    if (hi < 0 || lo < 0) return {};
    out.push_back(static_cast<std::uint8_t>(hi * 16 + lo));
  }
  return out;
}

json::Value load_vectors() {
  std::ifstream in(CHANNEL_VECTORS_PATH, std::ios::binary);
  REQUIRE(in.good());
  std::stringstream buf;
  buf << in.rdbuf();
  auto v = json::parse(buf.str());
  REQUIRE(v.has_value());
  const json::Value* list = v->find("vectors");
  REQUIRE(list && list->is_array() && !list->as_array().empty());
  return *list;
}

}  // namespace

CHECK_TEST(codec_reads_every_vector) {
  json::Value vectors = load_vectors();
  int checked = 0;
  for (const json::Value& vec : vectors.as_array()) {
    const std::string name = vec.find("name")->as_string();
    std::vector<std::uint8_t> bytes = from_hex(vec.find("frame_hex")->as_string());
    REQUIRE(!bytes.empty());
    CodecError err = CodecError::None;
    auto frame = decode_frame(bytes, kDefaultFrameMaxBytes, &err);
    if (!frame) {
      check::fail(__FILE__, __LINE__, "vector not decoded: " + name);
      continue;
    }
    CHECK_EQ(static_cast<int>(frame->type), static_cast<int>(vec.find("type")->as_integer()));
    if (frame->type == kFrameObject) {
      if (!(frame->object == *vec.find("object")))
        check::fail(__FILE__, __LINE__, "object differs: " + name);
    } else {
      CHECK(frame->bytes == from_hex(vec.find("bytes_hex")->as_string()));
    }
    ++checked;
  }
  CHECK_EQ(checked, static_cast<int>(vectors.as_array().size()));
}

CHECK_TEST(codec_own_output_reads_back_equal) {
  json::Value vectors = load_vectors();
  for (const json::Value& vec : vectors.as_array()) {
    const std::string name = vec.find("name")->as_string();
    std::optional<std::vector<std::uint8_t>> encoded;
    if (vec.find("type")->as_integer() == kFrameObject) {
      encoded = encode_object(*vec.find("object"), kDefaultFrameMaxBytes);
    } else {
      encoded = encode_image(from_hex(vec.find("bytes_hex")->as_string()), kDefaultFrameMaxBytes);
    }
    REQUIRE(encoded.has_value());
    auto frame = decode_frame(*encoded, kDefaultFrameMaxBytes);
    REQUIRE(frame.has_value());
    if (frame->type == kFrameObject) {
      if (!(frame->object == *vec.find("object")))
        check::fail(__FILE__, __LINE__, "own output differs: " + name);
    } else {
      CHECK(frame->bytes == from_hex(vec.find("bytes_hex")->as_string()));
      // An image frame has one encoding only, so it must equal the vector byte for byte.
      CHECK(*encoded == from_hex(vec.find("frame_hex")->as_string()));
    }
  }
}

CHECK_TEST(codec_reads_a_stream_split_anywhere) {
  json::Value vectors = load_vectors();
  std::vector<std::uint8_t> stream;
  std::size_t count = 0;
  for (const json::Value& vec : vectors.as_array()) {
    auto bytes = from_hex(vec.find("frame_hex")->as_string());
    stream.insert(stream.end(), bytes.begin(), bytes.end());
    ++count;
  }
  FrameReader reader(kDefaultFrameMaxBytes);
  std::vector<Frame> frames;
  for (std::uint8_t b : stream) REQUIRE(reader.feed(&b, 1, frames) == CodecError::None);
  CHECK_EQ(frames.size(), count);
  CHECK(reader.idle());
}

CHECK_TEST(codec_refuses_frames_over_the_limit) {
  // The length counts type byte and content: 5 is the largest a limit of 5 allows.
  std::vector<std::uint8_t> four = {1, 2, 3, 4};
  CHECK(encode_image(four, 5).has_value());
  CHECK(!encode_image(four, 4).has_value());
  CHECK(frame_fits(4, 5));
  CHECK(!frame_fits(5, 5));

  json::Value obj = json::Value::object({{"op", "release"}});
  CHECK(encode_object(obj, 17).has_value());   // {"op":"release"} is 16 bytes
  CHECK(!encode_object(obj, 16).has_value());

  // A reader refuses on the length prefix alone, before the content arrives.
  FrameReader reader(16);
  std::vector<Frame> frames;
  std::uint8_t header[] = {0x00, 0x00, 0x00, 0x11};
  CHECK(reader.feed(header, sizeof header, frames) == CodecError::TooLarge);
  std::uint8_t more[] = {0x01};
  CHECK(reader.feed(more, 1, frames) == CodecError::TooLarge);
  CHECK(frames.empty());

  std::uint8_t huge[] = {0xff, 0xff, 0xff, 0xff};
  FrameReader big(kDefaultFrameMaxBytes);
  CHECK(big.feed(huge, sizeof huge, frames) == CodecError::TooLarge);
}

CHECK_TEST(codec_refuses_malformed_frames) {
  auto refuse = [](std::vector<std::uint8_t> bytes, CodecError want) {
    FrameReader reader(kDefaultFrameMaxBytes);
    std::vector<Frame> frames;
    CodecError got = reader.feed(bytes.data(), bytes.size(), frames);
    CHECK(got == want);
    CHECK(frames.empty());
  };
  refuse({0, 0, 0, 0}, CodecError::Empty);
  refuse({0, 0, 0, 1, 0x03}, CodecError::UnknownType);
  refuse({0, 0, 0, 3, 0x01, '[', ']'}, CodecError::BadObject);     // not an object
  refuse({0, 0, 0, 2, 0x01, '{'}, CodecError::BadObject);
  refuse({0, 0, 0, 4, 0x01, '"', 0xC3, '"'}, CodecError::BadObject);  // invalid UTF-8
  std::vector<std::uint8_t> empty_object = {0, 0, 0, 3, 0x01, '{', '}'};
  CHECK(decode_frame(empty_object, kDefaultFrameMaxBytes).has_value());
  std::vector<std::uint8_t> trailing = {0, 0, 0, 3, 0x01, '{', '}', 0};
  CHECK(!decode_frame(trailing, kDefaultFrameMaxBytes).has_value());
  CHECK(!encode_object(json::Value(json::Value::Array{}), kDefaultFrameMaxBytes).has_value());
  CHECK(std::string(codec_error_name(CodecError::TooLarge)) == "too_large");
}

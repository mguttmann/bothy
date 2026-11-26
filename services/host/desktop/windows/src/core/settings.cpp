// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/settings.hpp"

#include <charconv>

#include "bothy/helper/codec.hpp"
#include "bothy/helper/json.hpp"

namespace bothy::helper {

EnablementState parse_enablement(std::string_view text) {
  std::optional<json::Value> v = json::parse(text);
  if (!v || !v->is_object() || v->as_object().size() != 3) return {};
  const json::Value* version = v->find("v");
  const json::Value* enabled = v->find("enabled");
  const json::Value* generation = v->find("generation");
  if (version == nullptr || !version->is_integer() || version->as_integer() != 1) return {};
  if (enabled == nullptr || !enabled->is_bool()) return {};
  if (generation == nullptr || !generation->is_integer() || generation->as_integer() < 0) return {};
  return EnablementState{enabled->as_bool(), generation->as_integer()};
}

std::string serialize_enablement(const EnablementState& state) {
  json::Value v = json::Value::object(
      {{"v", 1}, {"enabled", state.enabled}, {"generation", state.generation}});
  return json::serialize(v) + "\n";
}

namespace {

std::string_view trim(std::string_view s) {
  while (!s.empty() && (s.front() == ' ' || s.front() == '\t')) s.remove_prefix(1);
  while (!s.empty() && (s.back() == ' ' || s.back() == '\t' || s.back() == '\r')) s.remove_suffix(1);
  return s;
}

std::string_view strip_comment(std::string_view s) {
  // A ` #` starts a comment in YAML; values of this key are plain integers, so no quoting matters.
  std::size_t hash = s.find('#');
  if (hash == 0) return {};
  if (hash != std::string_view::npos && (s[hash - 1] == ' ' || s[hash - 1] == '\t'))
    s = s.substr(0, hash);
  return trim(s);
}

std::size_t indent_of(std::string_view line) {
  std::size_t n = 0;
  while (n < line.size() && line[n] == ' ') ++n;
  return n;
}

}  // namespace

std::uint64_t frame_max_from_host_config(std::string_view yaml) {
  bool in_block = false;
  std::size_t block_indent = 0;
  std::size_t pos = 0;
  while (pos <= yaml.size()) {
    std::size_t end = yaml.find('\n', pos);
    if (end == std::string_view::npos) end = yaml.size();
    std::string_view line = yaml.substr(pos, end - pos);
    pos = end + 1;
    std::string_view content = strip_comment(line);
    if (content.empty()) {
      if (pos > yaml.size()) break;
      continue;
    }
    std::size_t indent = indent_of(line);
    if (indent == 0) {
      in_block = content == "host_service:";
      block_indent = 0;
      continue;
    }
    if (!in_block) continue;
    if (block_indent == 0) block_indent = indent;
    if (indent != block_indent) continue;  // a deeper level belongs to another key
    constexpr std::string_view key = "computer_frame_max_bytes:";
    if (content.substr(0, key.size()) != key) continue;
    std::string_view value = trim(content.substr(key.size()));
    std::uint64_t n = 0;
    auto r = std::from_chars(value.data(), value.data() + value.size(), n);
    if (r.ec != std::errc() || r.ptr != value.data() + value.size() || n == 0) return kDefaultFrameMaxBytes;
    return n;
  }
  return kDefaultFrameMaxBytes;
}

}  // namespace bothy::helper

// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/json.hpp"

#include <charconv>
#include <cmath>
#include <limits>
#include <stdexcept>

#include "bothy/helper/utf.hpp"

namespace bothy::helper::json {

namespace {

// The channel nests at most three levels; the bound only protects the stack against a hostile
// frame, it limits nothing the protocol uses.
constexpr int kMaxDepth = 64;

enum Index { kNull, kBool, kInt, kDec, kStr, kArr, kObj };

}  // namespace

Value Value::object(std::initializer_list<Member> members) {
  Value v;
  v.data_ = Object{};
  for (const Member& m : members) v.set(m.first, m.second);
  return v;
}

Value::Kind Value::kind() const {
  switch (data_.index()) {
    case kNull: return Kind::Null;
    case kBool: return Kind::Bool;
    case kInt: return Kind::Integer;
    case kDec: return Kind::Decimal;
    case kStr: return Kind::String;
    case kArr: return Kind::Array;
    default: return Kind::Object;
  }
}

bool Value::is_integer() const {
  if (data_.index() == kInt) return true;
  if (data_.index() != kDec) return false;
  double d = std::get<double>(data_);
  // 2^63 is exactly representable; the range check keeps the cast in as_integer() defined.
  return std::isfinite(d) && std::trunc(d) == d && d >= -0x1p63 &&
         d < 0x1p63;
}

bool Value::as_bool() const { return std::get<bool>(data_); }

std::int64_t Value::as_integer() const {
  if (data_.index() == kInt) return std::get<std::int64_t>(data_);
  if (!is_integer()) throw std::logic_error("json: not an integer");
  return static_cast<std::int64_t>(std::get<double>(data_));
}

double Value::as_number() const {
  if (data_.index() == kInt) return static_cast<double>(std::get<std::int64_t>(data_));
  return std::get<double>(data_);
}

const std::string& Value::as_string() const { return std::get<std::string>(data_); }
const Value::Array& Value::as_array() const { return std::get<Array>(data_); }
const Value::Object& Value::as_object() const { return std::get<Object>(data_); }

const Value* Value::find(std::string_view name) const {
  if (data_.index() != kObj) return nullptr;
  for (const Member& m : std::get<Object>(data_))
    if (m.first == name) return &m.second;
  return nullptr;
}

Value& Value::set(std::string name, Value value) {
  if (data_.index() == kNull) data_ = Object{};
  Object& obj = std::get<Object>(data_);
  for (Member& m : obj) {
    if (m.first == name) {
      m.second = std::move(value);
      return *this;
    }
  }
  obj.emplace_back(std::move(name), std::move(value));
  return *this;
}

bool Value::operator==(const Value& other) const {
  if (is_number() && other.is_number()) {
    if (data_.index() == kInt && other.data_.index() == kInt)
      return std::get<std::int64_t>(data_) == std::get<std::int64_t>(other.data_);
    if (is_integer() && other.is_integer()) return as_integer() == other.as_integer();
    return as_number() == other.as_number();
  }
  if (data_.index() != other.data_.index()) return false;
  switch (data_.index()) {
    case kNull: return true;
    case kBool: return as_bool() == other.as_bool();
    case kStr: return as_string() == other.as_string();
    case kArr: return as_array() == other.as_array();
    case kObj: {
      const Object& a = as_object();
      const Object& b = other.as_object();
      if (a.size() != b.size()) return false;
      for (const Member& m : a) {
        const Value* o = other.find(m.first);
        if (o == nullptr || !(m.second == *o)) return false;
      }
      return true;
    }
    default: return false;
  }
}

namespace {

class Parser {
 public:
  Parser(std::string_view text, std::string* error) : s_(text), error_(error) {}

  std::optional<Value> run() {
    skip_ws();
    std::optional<Value> v = value(0);
    if (!v) return std::nullopt;
    skip_ws();
    if (pos_ != s_.size()) return fail("trailing characters");
    return v;
  }

 private:
  std::nullopt_t fail(const char* why) {
    if (error_ != nullptr && error_->empty())
      *error_ = std::string(why) + " at byte " + std::to_string(pos_);
    return std::nullopt;
  }

  void skip_ws() {
    while (pos_ < s_.size() &&
           (s_[pos_] == ' ' || s_[pos_] == '\t' || s_[pos_] == '\n' || s_[pos_] == '\r'))
      ++pos_;
  }

  bool literal(std::string_view word) {
    if (s_.substr(pos_, word.size()) != word) return false;
    pos_ += word.size();
    return true;
  }

  std::optional<Value> value(int depth) {
    if (depth > kMaxDepth) return fail("nesting too deep");
    if (pos_ >= s_.size()) return fail("unexpected end");
    char c = s_[pos_];
    if (c == '{') return object(depth);
    if (c == '[') return array(depth);
    if (c == '"') {
      std::optional<std::string> str = string();
      if (!str) return std::nullopt;
      return Value(std::move(*str));
    }
    if (c == 't') return literal("true") ? std::optional<Value>(Value(true)) : fail("bad literal");
    if (c == 'f') return literal("false") ? std::optional<Value>(Value(false)) : fail("bad literal");
    if (c == 'n') return literal("null") ? std::optional<Value>(Value()) : fail("bad literal");
    if (c == '-' || (c >= '0' && c <= '9')) return number();
    return fail("unexpected character");
  }

  std::optional<Value> object(int depth) {
    ++pos_;
    Value obj = Value::object();
    skip_ws();
    if (pos_ < s_.size() && s_[pos_] == '}') {
      ++pos_;
      return obj;
    }
    while (true) {
      skip_ws();
      if (pos_ >= s_.size() || s_[pos_] != '"') return fail("expected member name");
      std::optional<std::string> name = string();
      if (!name) return std::nullopt;
      if (obj.find(*name) != nullptr) return fail("duplicate member");
      skip_ws();
      if (pos_ >= s_.size() || s_[pos_] != ':') return fail("expected colon");
      ++pos_;
      skip_ws();
      std::optional<Value> v = value(depth + 1);
      if (!v) return std::nullopt;
      obj.set(std::move(*name), std::move(*v));
      skip_ws();
      if (pos_ >= s_.size()) return fail("unexpected end");
      if (s_[pos_] == ',') {
        ++pos_;
        continue;
      }
      if (s_[pos_] == '}') {
        ++pos_;
        return obj;
      }
      return fail("expected comma or closing brace");
    }
  }

  std::optional<Value> array(int depth) {
    ++pos_;
    Value::Array items;
    skip_ws();
    if (pos_ < s_.size() && s_[pos_] == ']') {
      ++pos_;
      return Value(std::move(items));
    }
    while (true) {
      skip_ws();
      std::optional<Value> v = value(depth + 1);
      if (!v) return std::nullopt;
      items.push_back(std::move(*v));
      skip_ws();
      if (pos_ >= s_.size()) return fail("unexpected end");
      if (s_[pos_] == ',') {
        ++pos_;
        continue;
      }
      if (s_[pos_] == ']') {
        ++pos_;
        return Value(std::move(items));
      }
      return fail("expected comma or closing bracket");
    }
  }

  bool hex4(char32_t& out) {
    if (s_.size() - pos_ < 4) return false;
    out = 0;
    for (int k = 0; k < 4; ++k) {
      char c = s_[pos_++];
      out <<= 4;
      if (c >= '0' && c <= '9') out |= static_cast<char32_t>(c - '0');
      else if (c >= 'a' && c <= 'f') out |= static_cast<char32_t>(c - 'a' + 10);
      else if (c >= 'A' && c <= 'F') out |= static_cast<char32_t>(c - 'A' + 10);
      else return false;
    }
    return true;
  }

  std::optional<std::string> string() {
    ++pos_;  // opening quote
    std::string out;
    std::size_t run_start = pos_;
    while (true) {
      if (pos_ >= s_.size()) return fail("unterminated string");
      auto c = static_cast<unsigned char>(s_[pos_]);
      if (c == '"') {
        out.append(s_.substr(run_start, pos_ - run_start));
        ++pos_;
        break;
      }
      if (c < 0x20) return fail("control character in string");
      if (c != '\\') {
        ++pos_;
        continue;
      }
      out.append(s_.substr(run_start, pos_ - run_start));
      ++pos_;
      if (pos_ >= s_.size()) return fail("unterminated escape");
      char e = s_[pos_++];
      switch (e) {
        case '"': out.push_back('"'); break;
        case '\\': out.push_back('\\'); break;
        case '/': out.push_back('/'); break;
        case 'b': out.push_back('\b'); break;
        case 'f': out.push_back('\f'); break;
        case 'n': out.push_back('\n'); break;
        case 'r': out.push_back('\r'); break;
        case 't': out.push_back('\t'); break;
        case 'u': {
          char32_t cp = 0;
          if (!hex4(cp)) return fail("bad unicode escape");
          if (cp >= 0xDC00 && cp <= 0xDFFF) return fail("lone low surrogate");
          if (cp >= 0xD800 && cp <= 0xDBFF) {
            char32_t lo = 0;
            if (s_.substr(pos_, 2) != "\\u") return fail("lone high surrogate");
            pos_ += 2;
            if (!hex4(lo) || lo < 0xDC00 || lo > 0xDFFF) return fail("lone high surrogate");
            cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
          }
          append_utf8(out, cp);
          break;
        }
        default: return fail("unknown escape");
      }
      run_start = pos_;
    }
    if (!is_valid_utf8(out)) return fail("invalid UTF-8");
    return out;
  }

  std::optional<Value> number() {
    std::size_t start = pos_;
    bool decimal = false;
    if (s_[pos_] == '-') ++pos_;
    if (pos_ >= s_.size()) return fail("bad number");
    if (s_[pos_] == '0') {
      ++pos_;
    } else if (s_[pos_] >= '1' && s_[pos_] <= '9') {
      while (pos_ < s_.size() && s_[pos_] >= '0' && s_[pos_] <= '9') ++pos_;
    } else {
      return fail("bad number");
    }
    if (pos_ < s_.size() && s_[pos_] == '.') {
      decimal = true;
      ++pos_;
      std::size_t digits = pos_;
      while (pos_ < s_.size() && s_[pos_] >= '0' && s_[pos_] <= '9') ++pos_;
      if (pos_ == digits) return fail("bad fraction");
    }
    if (pos_ < s_.size() && (s_[pos_] == 'e' || s_[pos_] == 'E')) {
      decimal = true;
      ++pos_;
      if (pos_ < s_.size() && (s_[pos_] == '+' || s_[pos_] == '-')) ++pos_;
      std::size_t digits = pos_;
      while (pos_ < s_.size() && s_[pos_] >= '0' && s_[pos_] <= '9') ++pos_;
      if (pos_ == digits) return fail("bad exponent");
    }
    const char* first = s_.data() + start;
    const char* last = s_.data() + pos_;
    if (!decimal) {
      std::int64_t i = 0;
      auto r = std::from_chars(first, last, i);
      if (r.ec == std::errc() && r.ptr == last) return Value(i);
      // Too large for 64 bits: keep it as a decimal rather than refusing valid JSON.
    }
    double d = 0;
    auto r = std::from_chars(first, last, d);
    if (r.ptr != last || !std::isfinite(d)) return fail("number out of range");
    return Value(d);
  }

  std::string_view s_;
  std::string* error_;
  std::size_t pos_ = 0;
};

void write_string(std::string& out, const std::string& s) {
  static const char* hex = "0123456789abcdef";
  out.push_back('"');
  for (char ch : s) {
    auto c = static_cast<unsigned char>(ch);
    switch (c) {
      case '"': out += "\\\""; break;
      case '\\': out += "\\\\"; break;
      case '\n': out += "\\n"; break;
      case '\r': out += "\\r"; break;
      case '\t': out += "\\t"; break;
      default:
        if (c < 0x20) {
          out += "\\u00";
          out.push_back(hex[c >> 4]);
          out.push_back(hex[c & 0xF]);
        } else {
          out.push_back(ch);
        }
    }
  }
  out.push_back('"');
}

void write(std::string& out, const Value& v) {
  switch (v.kind()) {
    case Value::Kind::Null: out += "null"; break;
    case Value::Kind::Bool: out += v.as_bool() ? "true" : "false"; break;
    case Value::Kind::Integer: out += std::to_string(v.as_integer()); break;
    case Value::Kind::Decimal: {
      double d = v.as_number();
      if (!std::isfinite(d)) {
        out += "null";
      } else if (v.is_integer()) {
        out += std::to_string(v.as_integer());
      } else {
        char buf[32];
        auto r = std::to_chars(buf, buf + sizeof buf, d);
        out.append(buf, r.ptr);
      }
      break;
    }
    case Value::Kind::String: write_string(out, v.as_string()); break;
    case Value::Kind::Array: {
      out.push_back('[');
      bool first = true;
      for (const Value& item : v.as_array()) {
        if (!first) out.push_back(',');
        first = false;
        write(out, item);
      }
      out.push_back(']');
      break;
    }
    case Value::Kind::Object: {
      out.push_back('{');
      bool first = true;
      for (const Value::Member& m : v.as_object()) {
        if (!first) out.push_back(',');
        first = false;
        write_string(out, m.first);
        out.push_back(':');
        write(out, m.second);
      }
      out.push_back('}');
      break;
    }
  }
}

}  // namespace

std::optional<Value> parse(std::string_view text, std::string* error) {
  return Parser(text, error).run();
}

std::string serialize(const Value& value) {
  std::string out;
  write(out, value);
  return out;
}

}  // namespace bothy::helper::json

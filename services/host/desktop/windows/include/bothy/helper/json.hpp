// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// A small JSON value, parser and serializer for the control frames of the helper channel
/// (docs/spec/27-computer-use.md section 9.4). Standard C++ only, so the channel logic builds and
/// tests on every developer machine; the helper needs no downloaded JSON library (D-CU-02-2).

#include <cstdint>
#include <optional>
#include <string>
#include <string_view>
#include <type_traits>
#include <utility>
#include <variant>
#include <vector>

namespace bothy::helper::json {

/// One JSON value: null, a boolean, an integer, a decimal number, a string (UTF-8), an array or an
/// object. Objects keep their members in insertion order, because the order is visible on the
/// wire, but two objects compare equal when they hold the same members in any order (the channel
/// vectors allow encoders to order members freely).
///
/// Example:
/// @code
///   json::Value v = json::Value::object({{"op", "state"}, {"locked", true}});
///   std::string text = json::serialize(v);   // {"op":"state","locked":true}
/// @endcode
class Value {
 public:
  /// The kind of a value. Integers and decimals are separate so that integer members such as
  /// `seq` keep their exact value; they still compare equal by numeric value.
  enum class Kind { Null, Bool, Integer, Decimal, String, Array, Object };

  /// The elements of an array.
  using Array = std::vector<Value>;
  /// One member of an object: name and value.
  using Member = std::pair<std::string, Value>;
  /// The members of an object in insertion order.
  using Object = std::vector<Member>;

  /// Creates null.
  Value() = default;
  /// Creates null.
  Value(std::nullptr_t) {}  // NOLINT(google-explicit-constructor): JSON literals read naturally
  /// Creates a boolean.
  Value(bool b) : data_(b) {}  // NOLINT(google-explicit-constructor)
  /// Creates an integer from any integral type except bool.
  /// @param i the value; unsigned values above the signed 64-bit range wrap and are not used here.
  template <typename T, std::enable_if_t<std::is_integral_v<T> && !std::is_same_v<T, bool>, int> = 0>
  Value(T i) : data_(static_cast<std::int64_t>(i)) {}  // NOLINT(google-explicit-constructor)
  /// Creates a decimal number. Non-finite values cannot be written as JSON; serialize() writes
  /// them as null.
  Value(double d) : data_(d) {}  // NOLINT(google-explicit-constructor)
  /// Creates a string; the text must be UTF-8.
  Value(const char* s) : data_(std::string(s)) {}  // NOLINT(google-explicit-constructor)
  /// Creates a string; the text must be UTF-8.
  Value(std::string s) : data_(std::move(s)) {}  // NOLINT(google-explicit-constructor)
  /// Creates an array.
  Value(Array a) : data_(std::move(a)) {}  // NOLINT(google-explicit-constructor)

  /// Creates an object from members in the given order.
  /// @param members the members; a repeated name keeps the last value (set() semantics).
  /// @return the object.
  static Value object(std::initializer_list<Member> members = {});

  /// @return the kind of this value.
  Kind kind() const;
  /// @return true for null.
  bool is_null() const { return kind() == Kind::Null; }
  /// @return true for a boolean.
  bool is_bool() const { return kind() == Kind::Bool; }
  /// @return true for an integer, or a decimal whose value is a whole number in the 64-bit range.
  bool is_integer() const;
  /// @return true for an integer or a decimal.
  bool is_number() const { return kind() == Kind::Integer || kind() == Kind::Decimal; }
  /// @return true for a string.
  bool is_string() const { return kind() == Kind::String; }
  /// @return true for an array.
  bool is_array() const { return kind() == Kind::Array; }
  /// @return true for an object.
  bool is_object() const { return kind() == Kind::Object; }

  /// @return the boolean. Precondition: is_bool().
  bool as_bool() const;
  /// @return the integer value. Precondition: is_integer().
  std::int64_t as_integer() const;
  /// @return the numeric value as a double. Precondition: is_number().
  double as_number() const;
  /// @return the string. Precondition: is_string().
  const std::string& as_string() const;
  /// @return the elements. Precondition: is_array().
  const Array& as_array() const;
  /// @return the members. Precondition: is_object().
  const Object& as_object() const;

  /// Looks up a member of an object.
  /// @param name the member name.
  /// @return the member's value, or nullptr when this is not an object or has no such member.
  const Value* find(std::string_view name) const;

  /// Sets a member of an object, replacing an existing one of the same name in place. A null value
  /// becomes an object first, so building an object member by member needs no constructor call.
  /// @param name the member name.
  /// @param value the new value.
  /// @return this value, for chaining.
  Value& set(std::string name, Value value);

  /// Deep equality: numbers by numeric value, objects regardless of member order.
  bool operator==(const Value& other) const;
  /// @return !(*this == other).
  bool operator!=(const Value& other) const { return !(*this == other); }

 private:
  std::variant<std::nullptr_t, bool, std::int64_t, double, std::string, Array, Object> data_;
};

/// Parses exactly one JSON text (RFC 8259). Strings must be valid UTF-8; `\u` escapes are decoded,
/// surrogate pairs included, and a lone surrogate is an error. Duplicate member names are an error,
/// because the receiver could not tell which one the sender meant. Numbers without a fraction or
/// exponent that fit 64 bits become integers, all others decimals.
/// @param text the JSON text; leading and trailing whitespace is allowed.
/// @param error receives a short reason with the byte offset on failure (never the input itself,
///        which may carry typed text); may be null.
/// @return the value, or std::nullopt on any syntax error.
std::optional<Value> parse(std::string_view text, std::string* error = nullptr);

/// Serializes a value as compact JSON (no whitespace). Non-ASCII text is written as UTF-8,
/// control characters as `\u00XX`. Integers are written exactly; decimals in the shortest form that
/// reads back to the same double, so a whole decimal such as 2.0 is written `2`.
/// @param value the value.
/// @return the JSON text.
std::string serialize(const Value& value);

}  // namespace bothy::helper::json

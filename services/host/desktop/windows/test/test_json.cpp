// SPDX-License-Identifier: Apache-2.0

// Tests of the JSON value, parser and serializer.

#include <string>

#include "bothy/helper/json.hpp"
#include "check.hpp"

using bothy::helper::json::parse;
using bothy::helper::json::serialize;
using bothy::helper::json::Value;

CHECK_TEST(json_parses_scalars_and_containers) {
  auto v = parse(R"( {"a": [1, -2, 3.5, 1e2, true, false, null], "b": {"c": "d"}} )");
  REQUIRE(v.has_value());
  REQUIRE(v->is_object());
  const Value* a = v->find("a");
  REQUIRE(a && a->is_array());
  REQUIRE(a->as_array().size() == 7);
  CHECK(a->as_array()[0].kind() == Value::Kind::Integer);
  CHECK_EQ(a->as_array()[1].as_integer(), -2);
  CHECK(a->as_array()[2].kind() == Value::Kind::Decimal);
  CHECK_EQ(a->as_array()[2].as_number(), 3.5);
  CHECK(a->as_array()[3].kind() == Value::Kind::Decimal);
  CHECK(a->as_array()[3].is_integer());
  CHECK_EQ(a->as_array()[3].as_integer(), 100);
  CHECK(a->as_array()[4].as_bool());
  CHECK(!a->as_array()[5].as_bool());
  CHECK(a->as_array()[6].is_null());
  CHECK_EQ(v->find("b")->find("c")->as_string(), std::string("d"));
}

CHECK_TEST(json_decodes_escapes_and_surrogate_pairs) {
  auto v = parse(R"("a\"b\\c\/d\b\f\n\r\t \u00e9\u4e16\ud83d\ude00")");
  REQUIRE(v.has_value());
  CHECK_EQ(v->as_string(), std::string("a\"b\\c/d\b\f\n\r\t \xC3\xA9\xE4\xB8\x96\xF0\x9F\x98\x80"));
}

CHECK_TEST(json_refuses_invalid_text) {
  const char* bad[] = {
      "",
      "{",
      "{\"a\":1,}",
      "[1,]",
      "{\"a\":1}x",
      "{\"a\":1,\"a\":2}",       // duplicate member
      "\"\\ud83d\"",             // lone high surrogate
      "\"\\ude00\"",             // lone low surrogate
      "\"\\ud83d\\u0041\"",      // high surrogate without low
      "\"a\x01\"",               // raw control character
      "\"\xC3\x28\"",            // invalid UTF-8
      "\"\xC0\xAF\"",            // overlong UTF-8
      "\"\\x\"",                 // unknown escape
      "01",                      // leading zero
      "1.",                      // fraction without digits
      "-",                       // sign only
      "tru",                     // truncated literal
      "NaN",
  };
  for (const char* text : bad) {
    std::string error;
    bool refused = !parse(text, &error).has_value();
    CHECK(refused);
    if (refused) CHECK(!error.empty());
  }
}

CHECK_TEST(json_refuses_deep_nesting) {
  std::string deep(10000, '[');
  CHECK(!parse(deep).has_value());
}

CHECK_TEST(json_serializes_and_reads_back) {
  Value v = Value::object({{"op", "do"},
                           {"seq", 4},
                           {"x", 640.5},
                           {"text", "na\xC3\xAFve \xF0\x9F\x98\x80 \"q\" \\ \n\x01"},
                           {"list", Value::Array{Value(1), Value(true), Value(nullptr)}},
                           {"o", Value::object({{"k", -7}})}});
  std::string text = serialize(v);
  auto back = parse(text);
  REQUIRE(back.has_value());
  CHECK(*back == v);
  CHECK(text.find("\\u0001") != std::string::npos);
  CHECK(text.find('\n') == std::string::npos);
}

CHECK_TEST(json_writes_whole_decimals_as_integers) {
  CHECK_EQ(serialize(Value(2.0)), std::string("2"));
  CHECK_EQ(serialize(Value(1.25)), std::string("1.25"));
  CHECK_EQ(serialize(Value(640.5)), std::string("640.5"));
  CHECK_EQ(serialize(Value(-9007199254740993LL)), std::string("-9007199254740993"));
}

CHECK_TEST(json_equality_ignores_member_order_and_number_kind) {
  auto a = parse(R"({"x":1,"y":{"p":2.0,"q":[1,2]}})");
  auto b = parse(R"({"y":{"q":[1,2],"p":2},"x":1.0})");
  REQUIRE(a && b);
  CHECK(*a == *b);
  auto c = parse(R"({"y":{"q":[2,1],"p":2},"x":1})");
  REQUIRE(c);
  CHECK(*a != *c);
  CHECK(Value(1) != Value(true));
  CHECK(Value("1") != Value(1));
}

CHECK_TEST(json_set_replaces_in_place) {
  Value v;
  v.set("a", 1).set("b", 2).set("a", 3);
  REQUIRE(v.is_object());
  REQUIRE(v.as_object().size() == 2);
  CHECK_EQ(v.as_object()[0].first, std::string("a"));
  CHECK_EQ(v.find("a")->as_integer(), 3);
}

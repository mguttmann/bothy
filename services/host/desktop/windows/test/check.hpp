// SPDX-License-Identifier: Apache-2.0
#pragma once

/// @file
/// The project's own test runner (D-CU-02-2): a test list and a few check macros, registered with
/// CTest one test per CHECK_TEST. No downloaded framework, so the tests build without network and
/// add no dependency to the license list.
///
/// Example:
/// @code
///   CHECK_TEST(adds_numbers) {
///     CHECK_EQ(1 + 1, 2);
///   }
/// @endcode
/// The test binary runs one test by name (`helper_core_tests adds_numbers`), all tests without an
/// argument, and prints the names with `--list`. CMakeLists.txt reads the CHECK_TEST lines of the
/// test sources and registers each name with CTest.

#include <exception>
#include <iostream>
#include <sstream>
#include <string>
#include <string_view>
#include <vector>

namespace check {

/// One registered test.
struct Case {
  const char* name;  ///< the name given to CHECK_TEST
  void (*fn)();      ///< the test body
};

/// @return the list of registered tests, in registration order.
inline std::vector<Case>& registry() {
  static std::vector<Case> cases;
  return cases;
}

/// Registers a test at static initialization; used by CHECK_TEST.
struct Registrar {
  /// @param name the test name.
  /// @param fn the test body.
  Registrar(const char* name, void (*fn)()) { registry().push_back({name, fn}); }
};

/// Thrown by REQUIRE to leave a test after a failed precondition.
struct Abort {};

/// @return the number of failed checks of the running test.
inline int& failures() {
  static int count = 0;
  return count;
}

/// Records one failed check.
/// @param file the source file.
/// @param line the line.
/// @param message what failed.
inline void fail(const char* file, int line, const std::string& message) {
  ++failures();
  std::cerr << file << ":" << line << ": " << message << "\n";
}

/// Describes a value for a failure message when it can be streamed, else says it cannot.
/// @param value the value.
/// @return the text.
template <typename T>
std::string describe(const T& value) {
  if constexpr (requires(std::ostream& o, const T& v) { o << v; }) {
    std::ostringstream out;
    out << value;
    return out.str();
  } else {
    return "<value>";
  }
}

/// Runs the tests.
/// @param argc argument count of main.
/// @param argv arguments: none (all tests), `--list`, or test names.
/// @return 0 when every selected test passed, 1 otherwise or for an unknown name.
inline int run(int argc, char** argv) {
  std::vector<std::string_view> wanted(argv + 1, argv + argc);
  if (wanted.size() == 1 && wanted[0] == "--list") {
    for (const Case& c : registry()) std::cout << c.name << "\n";
    return 0;
  }
  int failed = 0;
  int ran = 0;
  for (std::string_view name : wanted) {
    bool known = false;
    for (const Case& c : registry()) known = known || name == c.name;
    if (!known) {
      std::cerr << "unknown test: " << name << "\n";
      return 1;
    }
  }
  for (const Case& c : registry()) {
    bool selected = wanted.empty();
    for (std::string_view name : wanted) selected = selected || name == c.name;
    if (!selected) continue;
    ++ran;
    failures() = 0;
    try {
      c.fn();
    } catch (const Abort&) {
      // REQUIRE already recorded the failure.
    } catch (const std::exception& e) {
      fail(c.name, 0, std::string("exception: ") + e.what());
    }
    std::cout << (failures() == 0 ? "PASS " : "FAIL ") << c.name << "\n";
    if (failures() != 0) ++failed;
  }
  std::cout << ran - failed << " passed, " << failed << " failed\n";
  return failed == 0 ? 0 : 1;
}

}  // namespace check

/// Defines and registers a test. The name must be a C identifier.
#define CHECK_TEST(name)                                                   \
  static void check_test_##name();                                         \
  static const ::check::Registrar check_registrar_##name(#name, &check_test_##name); \
  static void check_test_##name()

/// Records a failure when the condition is false and goes on.
#define CHECK(cond)                                                       \
  do {                                                                    \
    if (!(cond)) ::check::fail(__FILE__, __LINE__, "CHECK(" #cond ")");   \
  } while (0)

/// Records a failure with both values when they differ and goes on.
#define CHECK_EQ(a, b)                                                               \
  do {                                                                               \
    const auto& check_a_ = (a);                                                      \
    const auto& check_b_ = (b);                                                      \
    if (!(check_a_ == check_b_))                                                     \
      ::check::fail(__FILE__, __LINE__,                                              \
                    "CHECK_EQ(" #a ", " #b "): " + ::check::describe(check_a_) +     \
                        " != " + ::check::describe(check_b_));                       \
  } while (0)

/// Records a failure and leaves the test when the condition is false.
#define REQUIRE(cond)                                                       \
  do {                                                                      \
    if (!(cond)) {                                                          \
      ::check::fail(__FILE__, __LINE__, "REQUIRE(" #cond ")");              \
      throw ::check::Abort{};                                               \
    }                                                                       \
  } while (0)

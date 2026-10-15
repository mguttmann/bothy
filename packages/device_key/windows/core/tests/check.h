// SPDX-License-Identifier: Apache-2.0
//
// A minimal test harness for the CTest suite of device_key_core (brief M4-11, D-M4-11-3).
//
// No third-party framework: the suite is one executable that runs the test named by its first
// argument, and CTest registers one test per name. A failed CHECK prints the file, line and
// condition and marks the test failed; SKIP prints the reason and ends the test with the exit
// code 77, which CTest reports as skipped (SKIP_RETURN_CODE). Exceptions are not used, because
// the Flutter build of the plugin compiles the standard library without them.

#ifndef DEVICE_KEY_CORE_TESTS_CHECK_H_
#define DEVICE_KEY_CORE_TESTS_CHECK_H_

#include <cstdio>
#include <functional>
#include <string>
#include <vector>

namespace device_key::testing {

/// The exit code CTest maps to "skipped" (`SKIP_RETURN_CODE` in tests/CMakeLists.txt).
inline constexpr int kSkipExitCode = 77;

/// The state of the running test: failed checks and a skip reason.
struct TestState {
  /// The number of failed checks so far.
  int failures = 0;
  /// Set by SKIP; a non-empty reason ends the test as skipped.
  std::string skip_reason;
};

/// Returns the state of the running test (one test per process run).
inline TestState& State() {
  static TestState state;
  return state;
}

/// One registered test: its name and body.
struct TestCase {
  /// The name CTest passes as the first argument.
  const char* name;
  /// The test body.
  std::function<void()> body;
};

/// Returns the registry every TEST adds itself to.
inline std::vector<TestCase>& Registry() {
  static std::vector<TestCase> tests;
  return tests;
}

/// Adds a test to the registry at static initialization (used by TEST).
struct Registrar {
  /// Registers [body] under [name].
  Registrar(const char* name, std::function<void()> body) {
    Registry().push_back({name, std::move(body)});
  }
};

/// Records a failed check with its location.
inline void Fail(const char* file, int line, const char* condition) {
  std::fprintf(stderr, "%s:%d: CHECK failed: %s\n", file, line, condition);
  ++State().failures;
}

}  // namespace device_key::testing

/// Defines and registers a test named `name`.
#define TEST(name)                                                         \
  static void name();                                                      \
  static ::device_key::testing::Registrar name##_registrar(#name, &name); \
  static void name()

/// Records a failure when `condition` is false and continues the test.
#define CHECK(condition)                                              \
  do {                                                                \
    if (!(condition)) ::device_key::testing::Fail(__FILE__, __LINE__, #condition); \
  } while (0)

/// Records a failure and leaves the test when `condition` is false.
#define REQUIRE(condition)                                            \
  do {                                                                \
    if (!(condition)) {                                               \
      ::device_key::testing::Fail(__FILE__, __LINE__, #condition);    \
      return;                                                         \
    }                                                                 \
  } while (0)

/// Ends the test as skipped with `reason` (a string literal).
#define SKIP(reason)                                       \
  do {                                                     \
    ::device_key::testing::State().skip_reason = (reason); \
    return;                                                \
  } while (0)

#endif  // DEVICE_KEY_CORE_TESTS_CHECK_H_

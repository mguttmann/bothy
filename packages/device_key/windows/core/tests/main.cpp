// SPDX-License-Identifier: Apache-2.0
//
// Runs the test named by the first argument (tests/CMakeLists.txt registers one CTest test per
// name). Leftover test keys and markers are deleted before and after the test, so a failed test
// never leaves a key in the key storage of the build host.

#include <cstdio>
#include <cstring>

#include "check.h"
#include "test_support.h"

int main(int argc, char** argv) {
  using device_key::testing::Registry;
  using device_key::testing::State;
  if (argc != 2) {
    std::fprintf(stderr, "usage: device_key_core_tests <test name>\n");
    return 2;
  }
  for (const auto& test : Registry()) {
    if (std::strcmp(test.name, argv[1]) != 0) continue;
    device_key::testing::DeleteLeftovers();
    test.body();
    device_key::testing::DeleteLeftovers();
    if (State().failures > 0) {
      std::fprintf(stderr, "%s: %d failed checks\n", test.name, State().failures);
      return 1;
    }
    if (!State().skip_reason.empty()) {
      std::printf("SKIP %s: %s\n", test.name, State().skip_reason.c_str());
      return device_key::testing::kSkipExitCode;
    }
    std::printf("PASS %s\n", test.name);
    return 0;
  }
  std::fprintf(stderr, "unknown test %s\n", argv[1]);
  return 2;
}

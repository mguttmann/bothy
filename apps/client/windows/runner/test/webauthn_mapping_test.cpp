// SPDX-License-Identifier: Apache-2.0
//
// Tests of webauthn_mapping.h with the host C++ compiler (run_tests.sh, `make runner-test`).
// The header has no Windows dependency, so the test runs on any host; webauthn_bridge.cpp
// checks the constants against webauthn.h at compile time.
#include "../webauthn_mapping.h"

#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

namespace {

int failures = 0;

void Expect(const char* name, bool ok) {
  if (!ok) {
    ++failures;
    std::printf("FAIL %s\n", name);
  } else {
    std::printf("ok   %s\n", name);
  }
}

}  // namespace

int main() {
  using webauthn_mapping::IsCancelled;
  using webauthn_mapping::TransportNames;
  using webauthn_mapping::UserVerification;

  Expect("required", UserVerification("required") == 1);
  Expect("preferred", UserVerification("preferred") == 2);
  Expect("discouraged", UserVerification("discouraged") == 3);
  Expect("unknown is preferred", UserVerification("") == 2);

  Expect("internal", TransportNames(0x10) == std::vector<std::string>{"internal"});
  Expect("usb and hybrid",
         TransportNames(0x21) == std::vector<std::string>{"usb", "hybrid"});
  Expect("test transport has no name", TransportNames(0x08).empty());

  Expect("NTE_USER_CANCELLED", IsCancelled(0x80090036u));
  Expect("ERROR_CANCELLED", IsCancelled(0x800704C7u));
  Expect("NTE_NOT_FOUND is a failure", !IsCancelled(0x80090011u));
  Expect("ERROR_TIMEOUT is a failure", !IsCancelled(0x800705B4u));

  return failures == 0 ? 0 : 1;
}

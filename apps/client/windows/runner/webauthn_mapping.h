// SPDX-License-Identifier: Apache-2.0
//
// The pure mappings of the WebAuthn bridge (webauthn_bridge.h) between the strings of the
// channel `bothy/webauthn` and the constants of the Win32 WebAuthn API (webauthn.h of the
// Windows SDK). Header-only and without <windows.h>, so the host compiler can test it
// (test/run_tests.sh); webauthn_bridge.cpp checks every constant here against the SDK header
// with static_assert.
#ifndef RUNNER_WEBAUTHN_MAPPING_H_
#define RUNNER_WEBAUTHN_MAPPING_H_

#include <cstdint>
#include <string>
#include <vector>

namespace webauthn_mapping {

// WEBAUTHN_USER_VERIFICATION_REQUIREMENT_*.
constexpr uint32_t kUserVerificationRequired = 1;
constexpr uint32_t kUserVerificationPreferred = 2;
constexpr uint32_t kUserVerificationDiscouraged = 3;

// WEBAUTHN_CTAP_TRANSPORT_*.
constexpr uint32_t kTransportUsb = 0x01;
constexpr uint32_t kTransportNfc = 0x02;
constexpr uint32_t kTransportBle = 0x04;
constexpr uint32_t kTransportInternal = 0x10;
constexpr uint32_t kTransportHybrid = 0x20;
constexpr uint32_t kTransportSmartCard = 0x40;

// NTE_USER_CANCELLED and HRESULT_FROM_WIN32(ERROR_CANCELLED) (ERROR_CANCELLED is 1223).
constexpr uint32_t kUserCancelled = 0x80090036u;
constexpr uint32_t kWin32Cancelled = 0x800704C7u;

// The requirement for the WebAuthn value |value| ("required", "preferred", "discouraged");
// any other value is "preferred", the WebAuthn default.
inline uint32_t UserVerification(const std::string& value) {
  if (value == "required") return kUserVerificationRequired;
  if (value == "discouraged") return kUserVerificationDiscouraged;
  return kUserVerificationPreferred;
}

// The WebAuthn transport names (AuthenticatorTransport) of the bits in |transports|, in a
// fixed order; the test transport has no name and is left out.
inline std::vector<std::string> TransportNames(uint32_t transports) {
  std::vector<std::string> names;
  if ((transports & kTransportUsb) != 0) names.push_back("usb");
  if ((transports & kTransportNfc) != 0) names.push_back("nfc");
  if ((transports & kTransportBle) != 0) names.push_back("ble");
  if ((transports & kTransportSmartCard) != 0) names.push_back("smart-card");
  if ((transports & kTransportHybrid) != 0) names.push_back("hybrid");
  if ((transports & kTransportInternal) != 0) names.push_back("internal");
  return names;
}

// Whether the HRESULT |hr| (as its bits) means the person cancelled or declined the Windows
// Hello prompt; the channel reports it with the code "cancelled".
inline bool IsCancelled(uint32_t hr) {
  return hr == kUserCancelled || hr == kWin32Cancelled;
}

}  // namespace webauthn_mapping

#endif  // RUNNER_WEBAUTHN_MAPPING_H_

// SPDX-License-Identifier: Apache-2.0

#include "bothy/helper/trust.hpp"

#include <cctype>

namespace bothy::helper {

namespace {

bool same_sid(std::string_view a, std::string_view b) {
  if (a.size() != b.size()) return false;
  for (std::size_t i = 0; i < a.size(); ++i) {
    if (std::toupper(static_cast<unsigned char>(a[i])) != std::toupper(static_cast<unsigned char>(b[i])))
      return false;
  }
  return true;
}

}  // namespace

bool pipe_server_allowed(std::string_view server_sid, std::string_view own_sid,
                         std::string_view service_sid) {
  if (server_sid.empty() || own_sid.empty()) return false;
  if (same_sid(server_sid, own_sid)) return false;
  if (same_sid(server_sid, kLocalSystemSid)) return true;
  return !service_sid.empty() && same_sid(server_sid, service_sid);
}

}  // namespace bothy::helper

// SPDX-License-Identifier: Apache-2.0
//
// Deep links of the URL scheme of the build branding on Windows (docs/spec/
// 15-client-architektur.md 11, docs/DECISIONS.md D-M4-18-4): the runner registers the scheme
// for the current user, a second start with a link hands the link to the running window over
// WM_COPYDATA and exits, and the window forwards links to Dart over the method channel
// `bothy/deep_links`. Only the Dart side parses a link (lib/app/deep_links.dart).
#ifndef RUNNER_DEEP_LINK_H_
#define RUNNER_DEEP_LINK_H_

#include <windows.h>

#include <optional>
#include <string>
#include <vector>

namespace deep_link {

// The dwData of the WM_COPYDATA message that carries a link (UTF-8, no terminator).
constexpr ULONG_PTR kCopyDataId = 0x424F5448;

// The first argument that starts with "<scheme>:" of the build branding, if any.
std::optional<std::string> FindLink(const std::vector<std::string>& arguments);

// Sends |link| to the window of a running instance and brings it to the front; false when no
// instance runs.
bool ForwardToRunningInstance(const std::string& link);

// Registers the scheme under HKEY_CURRENT_USER\Software\Classes for this executable, so the
// system starts the app with the link as its argument; no administrator rights are needed.
void RegisterUrlScheme();

}  // namespace deep_link

#endif  // RUNNER_DEEP_LINK_H_

// SPDX-License-Identifier: Apache-2.0
//
// Passkeys over Windows Hello for the Windows app (docs/spec/08-identity.md 2.5 and 6.2,
// docs/spec/15-client-architecture.md 8): the window serves the method channel
// `bothy/webauthn` through the Win32 WebAuthn API (webauthn.h, webauthn.lib). Dart is the
// WebAuthn client (`WindowsPasskeys` in lib/platform/windows_passkeys.dart): it writes the
// client data and every JSON form; the bridge only fills the Win32 structs and returns the
// bytes Windows produced.
//
// Methods: `available` (bool), `makeCredential` and `getAssertion` (maps of bytes). A cancelled
// or declined prompt answers with the error code "cancelled", any other failure with "failed"
// and the WebAuthn error name. The two ceremonies block until the person answers the Windows
// Hello prompt, so they run on a worker thread, and the result returns to the platform thread
// through the window message kDoneMessage.
#ifndef RUNNER_WEBAUTHN_BRIDGE_H_
#define RUNNER_WEBAUTHN_BRIDGE_H_

#include <windows.h>

#include <flutter/encodable_value.h>
#include <flutter/method_call.h>
#include <flutter/method_result.h>

#include <memory>

namespace webauthn_bridge {

// The window message that carries a finished ceremony; its LPARAM goes to Complete.
constexpr UINT kDoneMessage = WM_APP + 0x57;

// Serves |call| of the channel `bothy/webauthn` with |window| as the owner of the Windows
// Hello prompt; `available` answers at once, the ceremonies post kDoneMessage to |window|.
void Handle(HWND window,
            const flutter::MethodCall<flutter::EncodableValue>& call,
            std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

// Answers the call of the ceremony whose kDoneMessage carried |lparam|; on the platform
// thread, from the window's message handler.
void Complete(LPARAM lparam);

}  // namespace webauthn_bridge

#endif  // RUNNER_WEBAUTHN_BRIDGE_H_

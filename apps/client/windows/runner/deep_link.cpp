// SPDX-License-Identifier: Apache-2.0
#include "deep_link.h"

#include <algorithm>
#include <cctype>

#include "branding.h"

namespace deep_link {

namespace {

// The class name of the runner window (win32_window.cpp).
constexpr const wchar_t kWindowClassName[] = L"FLUTTER_RUNNER_WIN32_WINDOW";

std::string Lower(std::string text) {
  std::transform(text.begin(), text.end(), text.begin(),
                 [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
  return text;
}

bool SetString(HKEY root, const std::wstring& path, const wchar_t* name,
               const std::wstring& value) {
  const DWORD bytes = static_cast<DWORD>((value.size() + 1) * sizeof(wchar_t));
  return ::RegSetKeyValueW(root, path.c_str(), name, REG_SZ, value.c_str(), bytes) ==
         ERROR_SUCCESS;
}

}  // namespace

std::optional<std::string> FindLink(const std::vector<std::string>& arguments) {
  const std::string prefix = Lower(std::string(BRANDING_URL_SCHEME) + ":");
  for (const std::string& argument : arguments) {
    if (Lower(argument.substr(0, prefix.size())) == prefix) {
      return argument;
    }
  }
  return std::nullopt;
}

bool ForwardToRunningInstance(const std::string& link) {
  HWND window = ::FindWindowW(kWindowClassName, BRANDING_APP_NAME_W);
  if (window == nullptr) {
    return false;
  }
  COPYDATASTRUCT data;
  data.dwData = kCopyDataId;
  data.cbData = static_cast<DWORD>(link.size());
  data.lpData = const_cast<char*>(link.data());
  ::SendMessageW(window, WM_COPYDATA, 0, reinterpret_cast<LPARAM>(&data));
  ::SetForegroundWindow(window);
  return true;
}

void RegisterUrlScheme() {
  wchar_t executable[MAX_PATH];
  const DWORD length = ::GetModuleFileNameW(nullptr, executable, MAX_PATH);
  if (length == 0 || length == MAX_PATH) {
    return;
  }
  const std::wstring key = std::wstring(L"Software\\Classes\\") + BRANDING_URL_SCHEME_W;
  const std::wstring command = L"\"" + std::wstring(executable, length) + L"\" \"%1\"";
  SetString(HKEY_CURRENT_USER, key, nullptr, std::wstring(L"URL:") + BRANDING_APP_NAME_W);
  SetString(HKEY_CURRENT_USER, key, L"URL Protocol", L"");
  SetString(HKEY_CURRENT_USER, key + L"\\shell\\open\\command", nullptr, command);
}

}  // namespace deep_link

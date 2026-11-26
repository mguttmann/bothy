// SPDX-License-Identifier: Apache-2.0

// The enablement state file (D-CU-02-4) in the user's local application data, which the user's
// profile ACL closes to other users.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <shlobj.h>

#include "bothy/helper/settings.hpp"

namespace bothy::helper::win32 {

std::wstring FileEnablementStore::folder() {
  PWSTR base = nullptr;
  if (FAILED(SHGetKnownFolderPath(FOLDERID_LocalAppData, KF_FLAG_CREATE, nullptr, &base))) return {};
  std::wstring path = std::wstring(base) + L"\\bothy-computer-helper";
  CoTaskMemFree(base);
  return path;
}

EnablementState FileEnablementStore::load() {
  std::wstring dir = folder();
  if (dir.empty()) return {};
  HANDLE f = CreateFileW((dir + L"\\state.json").c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr,
                         OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
  if (f == INVALID_HANDLE_VALUE) return {};
  char buf[4096];
  DWORD read = 0;
  // The file holds one short object; anything longer is not a file this helper wrote.
  bool ok = ReadFile(f, buf, sizeof buf, &read, nullptr) && read < sizeof buf;
  CloseHandle(f);
  if (!ok) return {};
  return parse_enablement(std::string_view(buf, read));
}

bool FileEnablementStore::save(const EnablementState& state) {
  std::wstring dir = folder();
  if (dir.empty()) return false;
  if (!CreateDirectoryW(dir.c_str(), nullptr) && GetLastError() != ERROR_ALREADY_EXISTS) return false;
  std::wstring final_path = dir + L"\\state.json";
  std::wstring temp_path = dir + L"\\state.json.tmp";
  std::string text = serialize_enablement(state);
  HANDLE f = CreateFileW(temp_path.c_str(), GENERIC_WRITE, 0, nullptr, CREATE_ALWAYS,
                         FILE_ATTRIBUTE_NORMAL, nullptr);
  if (f == INVALID_HANDLE_VALUE) return false;
  DWORD written = 0;
  bool ok = WriteFile(f, text.data(), static_cast<DWORD>(text.size()), &written, nullptr) &&
            written == text.size() && FlushFileBuffers(f);
  CloseHandle(f);
  // The rename replaces the old file in one step, so a crash leaves either the old or the new state.
  if (!ok || !MoveFileExW(temp_path.c_str(), final_path.c_str(),
                          MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH)) {
    DeleteFileW(temp_path.c_str());
    return false;
  }
  return true;
}

}  // namespace bothy::helper::win32

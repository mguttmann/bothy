// SPDX-License-Identifier: Apache-2.0

// Facts about the running process and the host service's configuration file.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <shlobj.h>

#include <string>

#include "bothy/helper/settings.hpp"

namespace bothy::helper::win32 {

std::wstring own_executable_path() {
  std::wstring buf(MAX_PATH, L'\0');
  while (true) {
    DWORD n = GetModuleFileNameW(nullptr, buf.data(), static_cast<DWORD>(buf.size()));
    if (n == 0) return {};
    if (n < buf.size()) {
      buf.resize(n);
      return buf;
    }
    buf.resize(buf.size() * 2);  // the path was truncated; long paths exist with the opt-in
  }
}

std::string current_account() {
  wchar_t name[257];  // UNLEN + 1
  DWORD size = static_cast<DWORD>(sizeof name / sizeof name[0]);
  if (!GetUserNameW(name, &size) || size == 0) return {};
  return to_utf8(std::wstring(name, size - 1));
}

std::uint64_t read_frame_max() {
  PWSTR program_files = nullptr;
  if (FAILED(SHGetKnownFolderPath(FOLDERID_ProgramFiles, 0, nullptr, &program_files)))
    return kDefaultFrameMaxBytes;
  std::wstring path = std::wstring(program_files) + L"\\bothy-host\\config.yaml";
  CoTaskMemFree(program_files);
  HANDLE f = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE, nullptr,
                         OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
  if (f == INVALID_HANDLE_VALUE) return kDefaultFrameMaxBytes;  // not readable for this user
  std::string text;
  LARGE_INTEGER size{};
  // The configuration file is small; a file that claims to be huge is not read.
  if (GetFileSizeEx(f, &size) && size.QuadPart > 0 && size.QuadPart < (1 << 24)) {
    text.resize(static_cast<std::size_t>(size.QuadPart));
    DWORD read = 0;
    if (!ReadFile(f, text.data(), static_cast<DWORD>(text.size()), &read, nullptr)) read = 0;
    text.resize(read);
  }
  CloseHandle(f);
  return frame_max_from_host_config(text);
}

}  // namespace bothy::helper::win32

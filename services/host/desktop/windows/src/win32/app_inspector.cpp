// SPDX-License-Identifier: Apache-2.0

// Identity of the frontmost app: foreground window, process, executable, version resource, signer.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <memory>

namespace bothy::helper::win32 {

namespace {

std::wstring file_name_of(const std::wstring& path) {
  std::size_t slash = path.find_last_of(L"\\/");
  return slash == std::wstring::npos ? path : path.substr(slash + 1);
}

// FileDescription of the version resource in the file's first language, else the file name without
// its extension; this is the name a person sees in the task manager.
std::wstring display_name(const std::wstring& path) {
  DWORD ignored = 0;
  DWORD size = GetFileVersionInfoSizeW(path.c_str(), &ignored);
  if (size != 0) {
    std::unique_ptr<BYTE[]> data(new BYTE[size]);
    if (GetFileVersionInfoW(path.c_str(), 0, size, data.get())) {
      struct Translation {
        WORD language;
        WORD codepage;
      }* tr = nullptr;
      UINT len = 0;
      if (VerQueryValueW(data.get(), L"\\VarFileInfo\\Translation", reinterpret_cast<void**>(&tr), &len) &&
          len >= sizeof(Translation)) {
        wchar_t key[64];
        wsprintfW(key, L"\\StringFileInfo\\%04x%04x\\FileDescription", tr->language, tr->codepage);
        wchar_t* value = nullptr;
        UINT chars = 0;
        if (VerQueryValueW(data.get(), key, reinterpret_cast<void**>(&value), &chars) && chars > 1)
          return std::wstring(value, chars - 1);
      }
    }
  }
  std::wstring name = file_name_of(path);
  std::size_t dot = name.find_last_of(L'.');
  return dot == std::wstring::npos ? name : name.substr(0, dot);
}

}  // namespace

AppIdentity Win32AppInspector::frontmost() {
  HWND window = GetForegroundWindow();
  if (window == nullptr) return {};
  DWORD pid = 0;
  GetWindowThreadProcessId(window, &pid);
  if (pid == 0) return {};
  HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
  if (process == nullptr) return {};  // a protected or elevated process; the identity stays empty
  std::wstring path(32768, L'\0');
  DWORD size = static_cast<DWORD>(path.size());
  bool ok = QueryFullProcessImageNameW(process, 0, path.data(), &size) != 0;
  CloseHandle(process);
  if (!ok) return {};
  path.resize(size);

  WIN32_FILE_ATTRIBUTE_DATA attrs{};
  std::uint64_t file_size = 0;
  std::uint64_t write_time = 0;
  if (GetFileAttributesExW(path.c_str(), GetFileExInfoStandard, &attrs)) {
    file_size = (static_cast<std::uint64_t>(attrs.nFileSizeHigh) << 32) | attrs.nFileSizeLow;
    write_time = (static_cast<std::uint64_t>(attrs.ftLastWriteTime.dwHighDateTime) << 32) |
                 attrs.ftLastWriteTime.dwLowDateTime;
  }
  auto it = cache_.find(path);
  if (it != cache_.end() && it->second.size == file_size && it->second.write_time == write_time)
    return it->second.identity;

  AppIdentity identity{to_utf8(file_name_of(path)), to_utf8(display_name(path)), signer_subject(path)};
  cache_[path] = Cached{file_size, write_time, identity};
  return identity;
}

}  // namespace bothy::helper::win32

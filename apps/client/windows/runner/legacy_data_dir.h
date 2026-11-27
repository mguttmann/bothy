// SPDX-License-Identifier: Apache-2.0
//
// One-time move of the app's data folder after the version resource got the brand name
// (docs/decisions.d/FIX-windows-data-dir.md). path_provider_windows, and with it
// flutter_secure_storage_windows and the local databases, put the app's data under
// %APPDATA%\<CompanyName>\<ProductName> of the version resource of client.exe. Until
// AUD-PKG-P02/16 these were "com.example" and "client"; since then both are BRANDING_APP_NAME,
// so an updated app looked into an empty folder, found no device and showed the pairing screen.
// Before Flutter starts, the runner copies the old folder to the new one when the new one does
// not exist yet. The old folder stays as it is. Header-only and without <windows.h>, so the host
// compiler can test it (test/legacy_data_dir_test.cpp).
#ifndef RUNNER_LEGACY_DATA_DIR_H_
#define RUNNER_LEGACY_DATA_DIR_H_

#include <filesystem>
#include <string>
#include <system_error>

namespace legacy_data_dir {

// The folder names of the version resource before AUD-PKG-P02/16.
inline constexpr const wchar_t* kLegacyCompany = L"com.example";
inline constexpr const wchar_t* kLegacyProduct = L"client";

// |raw| as path_provider_windows makes it a folder name (_sanitizedDirectoryName): the
// characters <>:"/\|?* become '_', trailing white space and trailing dots go, at most 255
// characters. An empty result means path_provider uses no such folder level.
inline std::wstring SanitizedDirectoryName(std::wstring raw) {
  for (wchar_t& c : raw) {
    if (std::wstring(L"<>:\"/\\|?*").find(c) != std::wstring::npos) c = L'_';
  }
  while (!raw.empty() && (raw.back() == L' ' || raw.back() == L'\t' || raw.back() == L'\r' ||
                          raw.back() == L'\n' || raw.back() == L'\v' || raw.back() == L'\f')) {
    raw.pop_back();
  }
  while (!raw.empty() && raw.back() == L'.') raw.pop_back();
  if (raw.size() > 255) raw.resize(255);
  return raw;
}

enum class Result {
  kNoLegacy,      // no old folder of this app: nothing to do
  kTargetExists,  // the new folder exists already: it is never overwritten
  kCopied,        // the old folder was copied to the new one
  kFailed,        // the copy failed; the new folder was not created
};

// Copies |roaming|\com.example\client to |roaming|\<company>\<product> (both sanitized like
// path_provider_windows) when the old folder is this app's (it holds flutter_secure_storage.dat
// and the folder instances) and the new folder does not exist. The copy goes to a sibling
// "<product>.migrating" first and is renamed when complete, so an interrupted start leaves no
// half folder that path_provider would take as the app's data. The old folder is not changed.
inline Result Migrate(const std::filesystem::path& roaming, const std::wstring& company,
                      const std::wstring& product) {
  namespace fs = std::filesystem;
  std::error_code ec;
  const std::wstring company_dir = SanitizedDirectoryName(company);
  const std::wstring product_dir = SanitizedDirectoryName(product);
  if (product_dir.empty()) return Result::kNoLegacy;
  const fs::path legacy = roaming / kLegacyCompany / kLegacyProduct;
  const fs::path target =
      company_dir.empty() ? roaming / product_dir : roaming / company_dir / product_dir;
  if (legacy.lexically_normal() == target.lexically_normal()) return Result::kNoLegacy;
  if (!fs::is_regular_file(legacy / L"flutter_secure_storage.dat", ec) ||
      !fs::is_directory(legacy / L"instances", ec)) {
    return Result::kNoLegacy;
  }
  if (fs::exists(target, ec) || ec) return Result::kTargetExists;

  fs::path staging = target;
  staging += L".migrating";
  fs::remove_all(staging, ec);  // the rest of an interrupted start
  if (ec) return Result::kFailed;
  fs::create_directories(target.parent_path(), ec);
  if (ec) return Result::kFailed;
  fs::copy(legacy, staging, fs::copy_options::recursive | fs::copy_options::copy_symlinks, ec);
  if (!ec) fs::rename(staging, target, ec);
  if (ec) {
    std::error_code ignored;
    fs::remove_all(staging, ignored);
    return Result::kFailed;
  }
  return Result::kCopied;
}

}  // namespace legacy_data_dir

#endif  // RUNNER_LEGACY_DATA_DIR_H_

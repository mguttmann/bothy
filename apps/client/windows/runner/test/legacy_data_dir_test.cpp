// SPDX-License-Identifier: Apache-2.0
//
// Tests of legacy_data_dir.h with the host C++ compiler (run_tests.sh, `make runner-test`;
// docs/decisions.d/FIX-windows-data-dir.md). The folders are real, in a temporary folder that
// stands in for %APPDATA%.
#include "../legacy_data_dir.h"

#include <chrono>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <iterator>
#include <string>

namespace fs = std::filesystem;
using legacy_data_dir::Migrate;
using legacy_data_dir::Result;
using legacy_data_dir::SanitizedDirectoryName;

namespace {

int failures = 0;

void Expect(const char* name, bool ok) {
  if (!ok) ++failures;
  std::printf("%s %s\n", ok ? "ok  " : "FAIL", name);
}

void Write(const fs::path& file, const std::string& text) {
  fs::create_directories(file.parent_path());
  std::ofstream(file, std::ios::binary) << text;
}

std::string Read(const fs::path& file) {
  std::ifstream in(file, std::ios::binary);
  return std::string(std::istreambuf_iterator<char>(in), std::istreambuf_iterator<char>());
}

// A fresh stand-in for %APPDATA% with the old folder of the app: the secure store, a database
// with its WAL and a file of an instance.
fs::path Roaming(const char* name, bool with_legacy = true) {
  const fs::path root = fs::temp_directory_path() /
                        ("legacy_data_dir_test-" + std::to_string(std::chrono::steady_clock::now()
                                                                      .time_since_epoch()
                                                                      .count()) +
                         "-" + name);
  fs::create_directories(root);
  if (with_legacy) {
    const fs::path legacy = root / "com.example" / "client";
    Write(legacy / "flutter_secure_storage.dat", "secrets");
    Write(legacy / "instances" / "i1" / "local.db", "db");
    Write(legacy / "instances" / "i1" / "local.db-wal", "wal");
    Write(legacy / "instances" / "i1" / "files" / "f1" / "medium", "image");
  }
  return root;
}

void TestCopiesTheOldFolder() {
  const fs::path roaming = Roaming("copy");
  const Result result = Migrate(roaming, L"Brand", L"Brand");
  const fs::path target = roaming / "Brand" / "Brand";
  Expect("copy: result kCopied", result == Result::kCopied);
  Expect("copy: secure store copied", Read(target / "flutter_secure_storage.dat") == "secrets");
  Expect("copy: database and WAL copied",
         Read(target / "instances" / "i1" / "local.db") == "db" &&
             Read(target / "instances" / "i1" / "local.db-wal") == "wal");
  Expect("copy: nested files copied",
         Read(target / "instances" / "i1" / "files" / "f1" / "medium") == "image");
  Expect("copy: old folder kept",
         Read(roaming / "com.example" / "client" / "flutter_secure_storage.dat") == "secrets");
  Expect("copy: no staging folder left", !fs::exists(roaming / "Brand" / "Brand.migrating"));
  // The next start finds the new folder and does nothing, even after the app changed it.
  Write(target / "flutter_secure_storage.dat", "newer");
  Expect("copy: second start leaves the new folder",
         Migrate(roaming, L"Brand", L"Brand") == Result::kTargetExists &&
             Read(target / "flutter_secure_storage.dat") == "newer");
  fs::remove_all(roaming);
}

void TestNeverOverwritesAnExistingFolder() {
  const fs::path roaming = Roaming("exists");
  Write(roaming / "Brand" / "Brand" / "flutter_secure_storage.dat", "paired anew");
  Expect("exists: result kTargetExists", Migrate(roaming, L"Brand", L"Brand") ==
                                             Result::kTargetExists);
  Expect("exists: new folder unchanged",
         Read(roaming / "Brand" / "Brand" / "flutter_secure_storage.dat") == "paired anew");
  fs::remove_all(roaming);
}

void TestNothingWithoutTheAppsOldFolder() {
  const fs::path none = Roaming("none", false);
  Expect("none: result kNoLegacy", Migrate(none, L"Brand", L"Brand") == Result::kNoLegacy);
  Expect("none: no new folder", !fs::exists(none / "Brand"));
  fs::remove_all(none);
  // Another app of the Flutter template under com.example\client: no store of this app.
  const fs::path other = Roaming("other", false);
  Write(other / "com.example" / "client" / "settings.json", "{}");
  Expect("other: result kNoLegacy", Migrate(other, L"Brand", L"Brand") == Result::kNoLegacy);
  Expect("other: no new folder", !fs::exists(other / "Brand"));
  fs::remove_all(other);
}

void TestSameNamesAsBefore() {
  const fs::path roaming = Roaming("same");
  Expect("same: result kNoLegacy",
         Migrate(roaming, L"com.example", L"client") == Result::kNoLegacy);
  fs::remove_all(roaming);
}

void TestStagingOfAnInterruptedStart() {
  const fs::path roaming = Roaming("staging");
  Write(roaming / "Brand" / "Brand.migrating" / "half", "x");
  Expect("staging: result kCopied", Migrate(roaming, L"Brand", L"Brand") == Result::kCopied);
  Expect("staging: half copy gone", !fs::exists(roaming / "Brand" / "Brand" / "half") &&
                                        !fs::exists(roaming / "Brand" / "Brand.migrating"));
  fs::remove_all(roaming);
}

void TestSanitizedNames() {
  const fs::path roaming = Roaming("sanitized");
  Expect("sanitized: result kCopied",
         Migrate(roaming, L"A:B Co. ", L"Chat?..") == Result::kCopied);
  Expect("sanitized: folder as path_provider names it",
         fs::exists(roaming / "A_B Co" / "Chat_" / "flutter_secure_storage.dat"));
  fs::remove_all(roaming);
  Expect("sanitized: banned characters", SanitizedDirectoryName(L"a<b>c\"d/e\\f|g?h*") ==
                                             L"a_b_c_d_e_f_g_h_");
  Expect("sanitized: empty", SanitizedDirectoryName(L"...").empty());
  Expect("sanitized: at most 255", SanitizedDirectoryName(std::wstring(300, L'x')).size() == 255);
}

}  // namespace

int main() {
  TestCopiesTheOldFolder();
  TestNeverOverwritesAnExistingFolder();
  TestNothingWithoutTheAppsOldFolder();
  TestSameNamesAsBefore();
  TestStagingOfAnInterruptedStart();
  TestSanitizedNames();
  return failures == 0 ? 0 : 1;
}

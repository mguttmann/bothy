// SPDX-License-Identifier: Apache-2.0
//
// The CTest suite of device_key_core (brief M4-11, "Tests"). Every test key carries the prefix
// `device-key-test-`; main() deletes leftovers before and after each test, also after a failure.
// No test signs with a key that could show a prompt: the step-up key is never created on Windows
// (D-M4-11-2) and the DPoP key signs with NCRYPT_SILENT_FLAG.

#include <cstring>
#include <string>

#include "check.h"
#include "fake_key_provider.h"
#include "test_support.h"

namespace dk = device_key;
namespace dkt = device_key::testing;
using dk::ErrorCode;
using dk::KeyInfo;
using dk::KeyPurpose;
using dk::ProviderKind;

namespace {

constexpr const char* kScopeA = "https://chat.example.com";
constexpr const char* kScopeB = "https://sam.example.com";

/// A store over [provider] with the test prefix and the real Credential Manager markers.
struct Fixture {
  explicit Fixture(dk::KeyProvider& provider) : store(provider, markers, dkt::kTestPrefix) {}
  dk::CredentialMarkerStore markers;
  dk::DeviceKeyStore store;
};

std::wstring NameOf(dk::DeviceKeyStore& store, const char* scope, KeyPurpose purpose) {
  std::wstring name;
  CHECK(store.KeyName(scope, purpose, &name).ok());
  return name;
}

bool SameKey(const KeyInfo& a, const KeyInfo& b) { return a.x == b.x && a.y == b.y; }

}  // namespace

TEST(creates_two_distinct_keys_per_scope) {
  dk::CngKeyProvider cng;
  Fixture f(cng);
  KeyInfo first;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &first).ok());
  CHECK(first.x.size() == 32 && first.y.size() == 32);
  CHECK(!first.requires_user_presence);
  KeyInfo second;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &second).ok());
  CHECK(SameKey(first, second));
  CHECK(first.hardware_bound == second.hardware_bound);
  // The two purposes have distinct names, so they can never share a key.
  CHECK(NameOf(f.store, kScopeA, KeyPurpose::kDpop) !=
        NameOf(f.store, kScopeA, KeyPurpose::kStepUp));
  std::wstring name = NameOf(f.store, kScopeA, KeyPurpose::kDpop);
  CHECK(name.rfind(std::wstring(dkt::kTestPrefix) + L".", 0) == 0);
  CHECK(name.size() > 5 && name.substr(name.size() - 5) == L".dpop");
  CHECK(NameOf(f.store, kScopeA, KeyPurpose::kStepUp).find(L".step_up") != std::wstring::npos);
  CHECK(name.find(L"example") == std::wstring::npos);
}

TEST(scopes_are_separated) {
  dk::CngKeyProvider cng;
  Fixture f(cng);
  KeyInfo a;
  KeyInfo b;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &a).ok());
  REQUIRE(f.store.Ensure(kScopeB, KeyPurpose::kDpop, &b).ok());
  CHECK(!SameKey(a, b));
  REQUIRE(f.store.Delete(kScopeA).ok());
  std::optional<KeyInfo> after;
  REQUIRE(f.store.Info(kScopeB, KeyPurpose::kDpop, &after).ok());
  REQUIRE(after.has_value());
  CHECK(SameKey(*after, b));
  REQUIRE(f.store.Info(kScopeA, KeyPurpose::kDpop, &after).ok());
  CHECK(!after.has_value());
}

TEST(signature_is_raw_and_verifies) {
  dk::CngKeyProvider cng;
  Fixture f(cng);
  KeyInfo info;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  dk::Bytes data = dkt::ToBytes("eyJhbGciOiJFUzI1NiJ9.eyJodG0iOiJHRVQifQ");
  dk::Bytes signature;
  REQUIRE(f.store.Sign(kScopeA, KeyPurpose::kDpop, data, &signature).ok());
  CHECK(signature.size() == 64);
  CHECK(dkt::Verifies(info, data, signature));
  CHECK(!dkt::Verifies(info, dkt::ToBytes("other data"), signature));
  dk::Bytes missing;
  CHECK(f.store.Sign(kScopeB, KeyPurpose::kDpop, data, &missing).code ==
        ErrorCode::kNotAvailable);
}

TEST(tpm_path_when_available) {
  if (!dkt::PlatformAvailable()) SKIP("platform crypto provider not available on this host");
  dk::CngKeyProvider cng;
  Fixture f(cng);
  KeyInfo info;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  CHECK(info.hardware_bound);
  dk::NcryptHandle key;
  REQUIRE(dkt::OpenRealKey(ProviderKind::kPlatform, NameOf(f.store, kScopeA, KeyPurpose::kDpop),
                           &key) == ERROR_SUCCESS);
  dk::Bytes blob;
  CHECK(cng.ExportKey(key.get(), BCRYPT_ECCPRIVATE_BLOB, &blob) != ERROR_SUCCESS);
  dk::Bytes signature;
  dk::Bytes data = dkt::ToBytes("tpm");
  REQUIRE(f.store.Sign(kScopeA, KeyPurpose::kDpop, data, &signature).ok());
  CHECK(dkt::Verifies(info, data, signature));
}

TEST(software_fallback_without_tpm) {
  for (dkt::PlatformMode mode : {dkt::PlatformMode::kMissing, dkt::PlatformMode::kWithoutP256}) {
    dkt::DeleteLeftovers();
    dkt::FakeKeyProvider fake(mode);
    Fixture f(fake);
    KeyInfo info;
    REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
    CHECK(!info.hardware_bound);
    REQUIRE(fake.creates.size() == 1);
    CHECK(fake.creates[0] == ProviderKind::kSoftware);
    dk::NcryptHandle key;
    CHECK(dkt::OpenRealKey(ProviderKind::kSoftware, NameOf(f.store, kScopeA, KeyPurpose::kDpop),
                           &key) == ERROR_SUCCESS);
    dk::Bytes data = dkt::ToBytes("fallback");
    dk::Bytes signature;
    REQUIRE(f.store.Sign(kScopeA, KeyPurpose::kDpop, data, &signature).ok());
    CHECK(dkt::Verifies(info, data, signature));
  }
}

TEST(keys_are_not_exportable) {
  dk::CngKeyProvider cng;
  struct Case {
    ProviderKind kind;
    dkt::PlatformMode mode;
  };
  for (Case c : {Case{ProviderKind::kSoftware, dkt::PlatformMode::kMissing},
                 Case{ProviderKind::kPlatform, dkt::PlatformMode::kReal}}) {
    if (c.kind == ProviderKind::kPlatform && !dkt::PlatformAvailable()) {
      std::fprintf(stderr, "platform part skipped: platform crypto provider not available\n");
      continue;
    }
    dkt::DeleteLeftovers();
    dkt::FakeKeyProvider fake(c.mode);
    Fixture f(fake);
    KeyInfo info;
    REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
    dk::NcryptHandle key;
    REQUIRE(dkt::OpenRealKey(c.kind, NameOf(f.store, kScopeA, KeyPurpose::kDpop), &key) ==
            ERROR_SUCCESS);
    DWORD policy = 0xFFFFFFFF;
    REQUIRE(cng.GetDwordProperty(key.get(), NCRYPT_EXPORT_POLICY_PROPERTY, &policy) ==
            ERROR_SUCCESS);
    CHECK(policy == 0);
    DWORD type = 0xFFFFFFFF;
    if (cng.GetDwordProperty(key.get(), NCRYPT_KEY_TYPE_PROPERTY, &type) == ERROR_SUCCESS) {
      CHECK((type & NCRYPT_MACHINE_KEY_FLAG) == 0);
    }
    dk::Bytes blob;
    CHECK(cng.ExportKey(key.get(), BCRYPT_ECCPRIVATE_BLOB, &blob) != ERROR_SUCCESS);
    CHECK(cng.ExportKey(key.get(), BCRYPT_ECCPUBLIC_BLOB, &blob) == ERROR_SUCCESS);
  }
}

TEST(step_up_binding) {
  // D-M4-11-2: Windows has no system-enforced confirmation per use for a P-256 key, so the
  // step-up key is not offered and nothing is created for it.
  dkt::FakeKeyProvider fake;
  Fixture f(fake);
  KeyInfo info;
  CHECK(f.store.Ensure(kScopeA, KeyPurpose::kStepUp, &info).code == ErrorCode::kNotAvailable);
  CHECK(fake.creates.empty());
  std::optional<KeyInfo> found;
  REQUIRE(f.store.Info(kScopeA, KeyPurpose::kStepUp, &found).ok());
  CHECK(!found.has_value());
  dk::Bytes signature;
  CHECK(f.store.Sign(kScopeA, KeyPurpose::kStepUp, dkt::ToBytes("x"), &signature).code ==
        ErrorCode::kNotAvailable);
  std::wstring name = NameOf(f.store, kScopeA, KeyPurpose::kStepUp);
  dk::NcryptHandle key;
  CHECK(dkt::OpenRealKey(ProviderKind::kSoftware, name, &key) != ERROR_SUCCESS);
  CHECK(dkt::OpenRealKey(ProviderKind::kPlatform, name, &key) != ERROR_SUCCESS);
  // The DPoP key needs no confirmation: it signs only with NCRYPT_SILENT_FLAG, so a provider
  // that wanted to show a dialog would fail the call instead.
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  CHECK(!info.requires_user_presence);
  REQUIRE(f.store.Sign(kScopeA, KeyPurpose::kDpop, dkt::ToBytes("x"), &signature).ok());
  REQUIRE(fake.sign_flags.size() == 1);
  CHECK((fake.sign_flags[0] & NCRYPT_SILENT_FLAG) != 0);
}

TEST(existing_key_is_not_recreated_in_other_provider) {
  // A software key created while the TPM was hidden stays in the software provider once the
  // TPM is visible again, with its marker and without it (a crash between create and marker).
  dkt::FakeKeyProvider without_tpm(dkt::PlatformMode::kMissing);
  Fixture before(without_tpm);
  KeyInfo created;
  REQUIRE(before.store.Ensure(kScopeA, KeyPurpose::kDpop, &created).ok());
  REQUIRE(!created.hardware_bound);
  std::wstring name = NameOf(before.store, kScopeA, KeyPurpose::kDpop);

  dkt::FakeKeyProvider with_tpm(dkt::PlatformMode::kReal);
  Fixture after(with_tpm);
  KeyInfo reopened;
  REQUIRE(after.store.Ensure(kScopeA, KeyPurpose::kDpop, &reopened).ok());
  CHECK(SameKey(created, reopened));
  CHECK(!reopened.hardware_bound);
  CHECK(with_tpm.creates.empty());

  REQUIRE(after.markers.Remove(name) == ERROR_SUCCESS);
  REQUIRE(after.store.Ensure(kScopeA, KeyPurpose::kDpop, &reopened).ok());
  CHECK(SameKey(created, reopened));
  CHECK(!reopened.hardware_bound);
  CHECK(with_tpm.creates.empty());
  std::optional<ProviderKind> marker;
  REQUIRE(after.markers.Read(name, &marker) == ERROR_SUCCESS);
  CHECK(marker == ProviderKind::kSoftware);
  dk::NcryptHandle key;
  CHECK(dkt::OpenRealKey(ProviderKind::kPlatform, name, &key) != ERROR_SUCCESS);
}

TEST(marker_without_key_is_invalidated) {
  dkt::FakeKeyProvider fake(dkt::PlatformMode::kMissing);
  Fixture f(fake);
  KeyInfo info;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  dk::NcryptHandle key;
  REQUIRE(dkt::OpenRealKey(ProviderKind::kSoftware, NameOf(f.store, kScopeA, KeyPurpose::kDpop),
                           &key) == ERROR_SUCCESS);
  dk::CngKeyProvider cng;
  REQUIRE(cng.DeleteKey(&key) == ERROR_SUCCESS);  // lost behind the plugin, as after a TPM clear
  CHECK(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).code == ErrorCode::kKeyInvalidated);
  std::optional<KeyInfo> found;
  CHECK(f.store.Info(kScopeA, KeyPurpose::kDpop, &found).code == ErrorCode::kKeyInvalidated);
  dk::Bytes signature;
  CHECK(f.store.Sign(kScopeA, KeyPurpose::kDpop, dkt::ToBytes("x"), &signature).code ==
        ErrorCode::kKeyInvalidated);
  CHECK(fake.creates.size() == 1);  // never recreated silently
  REQUIRE(f.store.Delete(kScopeA).ok());
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());  // a new registration
  CHECK(fake.creates.size() == 2);
}

TEST(delete_removes_both_keys) {
  dkt::FakeKeyProvider fake(dkt::PlatformMode::kMissing);
  Fixture f(fake);
  KeyInfo info;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  std::wstring dpop = NameOf(f.store, kScopeA, KeyPurpose::kDpop);
  std::wstring step_up = NameOf(f.store, kScopeA, KeyPurpose::kStepUp);
  // A key under the step-up name (for example from a later version) is deleted as well.
  {
    dk::CngKeyProvider cng;
    dk::NcryptHandle provider;
    REQUIRE(cng.OpenProvider(ProviderKind::kSoftware, &provider) == ERROR_SUCCESS);
    dk::NcryptHandle key;
    REQUIRE(cng.CreatePersistedKey(provider.get(), BCRYPT_ECDSA_P256_ALGORITHM, step_up.c_str(),
                                   &key) == ERROR_SUCCESS);
    REQUIRE(cng.FinalizeKey(key.get()) == ERROR_SUCCESS);
  }
  REQUIRE(f.store.Delete(kScopeA).ok());
  for (const std::wstring& name : {dpop, step_up}) {
    dk::NcryptHandle key;
    CHECK(dkt::OpenRealKey(ProviderKind::kSoftware, name, &key) != ERROR_SUCCESS);
    CHECK(dkt::OpenRealKey(ProviderKind::kPlatform, name, &key) != ERROR_SUCCESS);
    std::optional<ProviderKind> marker = ProviderKind::kPlatform;
    REQUIRE(f.markers.Read(name, &marker) == ERROR_SUCCESS);
    CHECK(!marker.has_value());
  }
  std::optional<KeyInfo> found;
  REQUIRE(f.store.Info(kScopeA, KeyPurpose::kDpop, &found).ok());
  CHECK(!found.has_value());
  CHECK(f.store.Delete(kScopeA).ok());  // deleting missing keys is fine
}

TEST(errors_carry_no_key_material) {
  dkt::FakeKeyProvider fake(dkt::PlatformMode::kMissing);
  Fixture f(fake);
  KeyInfo info;
  REQUIRE(f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info).ok());
  dk::Bytes data = dkt::ToBytes("secret-signing-input");
  dk::Bytes signature;

  fake.sign_failure = NTE_FAIL;
  dk::Status failed = f.store.Sign(kScopeA, KeyPurpose::kDpop, data, &signature);
  CHECK(failed.code == ErrorCode::kInternal);
  CHECK(std::strcmp(failed.wire(), "internal") == 0);
  CHECK(failed.Message() == "the device key operation failed (0x80090020)");
  CHECK(signature.empty());

  fake.sign_failure = NTE_USER_CANCELLED;
  dk::Status cancelled = f.store.Sign(kScopeA, KeyPurpose::kDpop, data, &signature);
  CHECK(cancelled.code == ErrorCode::kUserCancelled);
  CHECK(std::strcmp(cancelled.wire(), "user_cancelled") == 0);

  fake.sign_failure = 0;
  fake.export_failure = NTE_BAD_DATA;
  dk::Status export_failed = f.store.Ensure(kScopeA, KeyPurpose::kDpop, &info);
  CHECK(export_failed.code == ErrorCode::kInternal);
  for (const dk::Status& status : {failed, cancelled, export_failed}) {
    std::string message = status.Message();
    CHECK(message.find("example") == std::string::npos);
    CHECK(message.find("secret") == std::string::npos);
    CHECK(message.find("device-key") == std::string::npos);
  }
  CHECK(std::strcmp(dk::Status::Fail(ErrorCode::kKeyInvalidated).wire(), "key_invalidated") == 0);
  CHECK(std::strcmp(dk::Status::Fail(ErrorCode::kNotAvailable).wire(), "not_available") == 0);
}

// SPDX-License-Identifier: Apache-2.0
//
// Helpers of the CTest suite that talk to CNG and the Credential Manager directly, so the tests
// check the stored state without going through the code under test.

#ifndef DEVICE_KEY_CORE_TESTS_TEST_SUPPORT_H_
#define DEVICE_KEY_CORE_TESTS_TEST_SUPPORT_H_

#include <windows.h>

#include <wincred.h>

#include <string>

#include "cng_key_provider.h"
#include "device_key_store.h"

namespace device_key::testing {

/// Every test key and marker starts with this prefix (brief M4-11 "Boundaries").
inline constexpr const wchar_t* kTestPrefixRoot = L"device-key-test-";

/// The prefix the tests give DeviceKeyStore; it starts with kTestPrefixRoot.
inline constexpr const wchar_t* kTestPrefix = L"device-key-test-core";

/// Whether [text] starts with kTestPrefixRoot.
inline bool IsTestName(const wchar_t* text) {
  return text != nullptr && std::wstring(text).rfind(kTestPrefixRoot, 0) == 0;
}

/// Deletes every key of the current user whose name starts with kTestPrefixRoot in both
/// providers, and every such credential marker. Runs before and after each test.
inline void DeleteLeftovers() {
  for (LPCWSTR provider_name : {MS_PLATFORM_CRYPTO_PROVIDER, MS_KEY_STORAGE_PROVIDER}) {
    NCRYPT_PROV_HANDLE provider = 0;
    if (NCryptOpenStorageProvider(&provider, provider_name, 0) != ERROR_SUCCESS) continue;
    std::vector<std::wstring> names;
    NCryptKeyName* key_name = nullptr;
    PVOID state = nullptr;
    while (NCryptEnumKeys(provider, nullptr, &key_name, &state, NCRYPT_SILENT_FLAG) ==
           ERROR_SUCCESS) {
      if (IsTestName(key_name->pszName)) names.emplace_back(key_name->pszName);
      NCryptFreeBuffer(key_name);
      key_name = nullptr;
    }
    if (state != nullptr) NCryptFreeBuffer(state);
    for (const std::wstring& name : names) {
      NCRYPT_KEY_HANDLE key = 0;
      if (NCryptOpenKey(provider, &key, name.c_str(), 0, NCRYPT_SILENT_FLAG) == ERROR_SUCCESS &&
          NCryptDeleteKey(key, NCRYPT_SILENT_FLAG) != ERROR_SUCCESS) {
        NCryptFreeObject(key);
      }
    }
    NCryptFreeObject(provider);
  }
  DWORD count = 0;
  PCREDENTIALW* credentials = nullptr;
  std::wstring filter = std::wstring(kTestPrefixRoot) + L"*";
  if (CredEnumerateW(filter.c_str(), 0, &count, &credentials)) {
    for (DWORD i = 0; i < count; ++i) {
      if (IsTestName(credentials[i]->TargetName)) {
        CredDeleteW(credentials[i]->TargetName, CRED_TYPE_GENERIC, 0);
      }
    }
    CredFree(credentials);
  }
}

/// Opens the key [name] in the real provider [kind]; returns the status of `NCryptOpenKey`, or of
/// `NCryptOpenStorageProvider` when the provider is missing.
inline SECURITY_STATUS OpenRealKey(ProviderKind kind, const std::wstring& name,
                                   NcryptHandle* key) {
  CngKeyProvider cng;
  NcryptHandle provider;
  SECURITY_STATUS status = cng.OpenProvider(kind, &provider);
  if (status != ERROR_SUCCESS) return status;
  return cng.OpenKey(provider.get(), name.c_str(), key);
}

/// Whether the real Platform Crypto Provider opens and supports ECDSA P-256 on this host.
inline bool PlatformAvailable() {
  CngKeyProvider cng;
  NcryptHandle provider;
  return cng.OpenProvider(ProviderKind::kPlatform, &provider) == ERROR_SUCCESS &&
         cng.IsAlgSupported(provider.get(), BCRYPT_ECDSA_P256_ALGORITHM) == ERROR_SUCCESS;
}

/// Verifies [signature] (`R || S`) over SHA-256 of [data] with the public key [info] through
/// `BCryptImportKeyPair` and `BCryptVerifySignature`.
inline bool Verifies(const KeyInfo& info, const Bytes& data, const Bytes& signature) {
  if (info.x.size() != 32 || info.y.size() != 32) return false;
  CngKeyProvider cng;
  Bytes digest;
  if (cng.Sha256(data, &digest) != 0) return false;
  Bytes blob(sizeof(BCRYPT_ECCKEY_BLOB));
  auto* header = reinterpret_cast<BCRYPT_ECCKEY_BLOB*>(blob.data());
  header->dwMagic = BCRYPT_ECDSA_PUBLIC_P256_MAGIC;
  header->cbKey = 32;
  blob.insert(blob.end(), info.x.begin(), info.x.end());
  blob.insert(blob.end(), info.y.begin(), info.y.end());
  BCRYPT_ALG_HANDLE algorithm = nullptr;
  if (BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_ECDSA_P256_ALGORITHM, nullptr, 0) != 0) {
    return false;
  }
  BCRYPT_KEY_HANDLE key = nullptr;
  bool ok = false;
  if (BCryptImportKeyPair(algorithm, nullptr, BCRYPT_ECCPUBLIC_BLOB, &key, blob.data(),
                          static_cast<ULONG>(blob.size()), 0) == 0) {
    ok = BCryptVerifySignature(key, nullptr, digest.data(), static_cast<ULONG>(digest.size()),
                               const_cast<PUCHAR>(signature.data()),
                               static_cast<ULONG>(signature.size()), 0) == 0;
    BCryptDestroyKey(key);
  }
  BCryptCloseAlgorithmProvider(algorithm, 0);
  return ok;
}

/// Bytes of an ASCII string.
inline Bytes ToBytes(const std::string& text) { return Bytes(text.begin(), text.end()); }

}  // namespace device_key::testing

#endif  // DEVICE_KEY_CORE_TESTS_TEST_SUPPORT_H_

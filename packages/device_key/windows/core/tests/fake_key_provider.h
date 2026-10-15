// SPDX-License-Identifier: Apache-2.0

#ifndef DEVICE_KEY_CORE_TESTS_FAKE_KEY_PROVIDER_H_
#define DEVICE_KEY_CORE_TESTS_FAKE_KEY_PROVIDER_H_

#include <map>
#include <vector>

#include "cng_key_provider.h"

namespace device_key::testing {

/// What the fake makes of the Platform Crypto Provider.
enum class PlatformMode {
  /// Passes every call to the real provider (a host with or without a TPM).
  kReal,
  /// The platform provider cannot be opened, as on a machine without TPM 2.0.
  kMissing,
  /// The platform provider opens but `NCryptIsAlgSupported(ECDSA_P256)` is `NTE_NOT_SUPPORTED`.
  /// The handle is a real software provider handle, so a create on it would land there; the store
  /// must not create a key on it.
  kWithoutP256,
};

/// A KeyProvider that wraps CngKeyProvider: it hides or degrades the TPM, records the calls and
/// can fail SignHash, while every key still comes from a real Windows provider (Design note 2).
class FakeKeyProvider : public KeyProvider {
 public:
  /// Creates a fake with [mode] for the platform provider.
  explicit FakeKeyProvider(PlatformMode mode = PlatformMode::kReal) : mode_(mode) {}

  /// The kinds CreatePersistedKey was called for, in order.
  std::vector<ProviderKind> creates;
  /// The flags of every SignHash call, in order.
  std::vector<DWORD> sign_flags;
  /// When not 0, SignHash returns this status instead of signing.
  SECURITY_STATUS sign_failure = 0;
  /// When not 0, ExportKey returns this status instead of exporting.
  SECURITY_STATUS export_failure = 0;

  SECURITY_STATUS OpenProvider(ProviderKind kind, NcryptHandle* provider) override {
    if (kind == ProviderKind::kPlatform && mode_ == PlatformMode::kMissing) {
      return NTE_PROV_DLL_NOT_FOUND;
    }
    ProviderKind real = kind;
    if (kind == ProviderKind::kPlatform && mode_ == PlatformMode::kWithoutP256) {
      real = ProviderKind::kSoftware;
    }
    SECURITY_STATUS status = real_.OpenProvider(real, provider);
    if (status == ERROR_SUCCESS) kinds_[provider->get()] = kind;
    return status;
  }

  SECURITY_STATUS IsAlgSupported(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm) override {
    if (mode_ == PlatformMode::kWithoutP256 && KindOf(provider) == ProviderKind::kPlatform) {
      return NTE_NOT_SUPPORTED;
    }
    return real_.IsAlgSupported(provider, algorithm);
  }

  SECURITY_STATUS CreatePersistedKey(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm,
                                     LPCWSTR name, NcryptHandle* key) override {
    creates.push_back(KindOf(provider));
    return real_.CreatePersistedKey(provider, algorithm, name, key);
  }

  SECURITY_STATUS SetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                   DWORD value) override {
    return real_.SetDwordProperty(object, property, value);
  }

  SECURITY_STATUS GetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                   DWORD* value) override {
    return real_.GetDwordProperty(object, property, value);
  }

  SECURITY_STATUS FinalizeKey(NCRYPT_KEY_HANDLE key) override { return real_.FinalizeKey(key); }

  SECURITY_STATUS OpenKey(NCRYPT_PROV_HANDLE provider, LPCWSTR name, NcryptHandle* key) override {
    if (mode_ == PlatformMode::kWithoutP256 && KindOf(provider) == ProviderKind::kPlatform) {
      // The simulated TPM holds no keys; its handle must not reach the software keys.
      return NTE_BAD_KEYSET;
    }
    return real_.OpenKey(provider, name, key);
  }

  SECURITY_STATUS ExportKey(NCRYPT_KEY_HANDLE key, LPCWSTR blob_type, Bytes* blob) override {
    if (export_failure != 0) return export_failure;
    return real_.ExportKey(key, blob_type, blob);
  }

  SECURITY_STATUS SignHash(NCRYPT_KEY_HANDLE key, const Bytes& hash, DWORD flags,
                           Bytes* signature) override {
    sign_flags.push_back(flags);
    if (sign_failure != 0) return sign_failure;
    return real_.SignHash(key, hash, flags, signature);
  }

  SECURITY_STATUS DeleteKey(NcryptHandle* key) override { return real_.DeleteKey(key); }

  SECURITY_STATUS Sha256(const Bytes& data, Bytes* digest) override {
    return real_.Sha256(data, digest);
  }

 private:
  ProviderKind KindOf(NCRYPT_PROV_HANDLE provider) const {
    auto it = kinds_.find(provider);
    return it == kinds_.end() ? ProviderKind::kSoftware : it->second;
  }

  PlatformMode mode_;
  CngKeyProvider real_;
  std::map<NCRYPT_HANDLE, ProviderKind> kinds_;
};

}  // namespace device_key::testing

#endif  // DEVICE_KEY_CORE_TESTS_FAKE_KEY_PROVIDER_H_

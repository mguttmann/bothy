// SPDX-License-Identifier: Apache-2.0
//
// The seam between the device key logic and Windows CNG (brief M4-11, Design note 2).
//
// DeviceKeyStore talks to the key storage providers only through KeyProvider, so the tests can
// inject a provider that hides the TPM, reports a TPM without P-256 or fails a call, while the
// keys themselves still come from the real Microsoft Software Key Storage Provider.

#ifndef DEVICE_KEY_CORE_KEY_PROVIDER_H_
#define DEVICE_KEY_CORE_KEY_PROVIDER_H_

#include <windows.h>

#include <bcrypt.h>
#include <ncrypt.h>

#include <cstdint>
#include <vector>

namespace device_key {

/// A byte string (public key blob, hash, signature).
using Bytes = std::vector<uint8_t>;

/// The two key storage providers the plugin uses (docs/spec/15-client-architecture.md 9).
enum class ProviderKind {
  /// `MS_PLATFORM_CRYPTO_PROVIDER`: keys in the TPM 2.0, hardware-bound.
  kPlatform,
  /// `MS_KEY_STORAGE_PROVIDER`: software keys, reported as not hardware-bound.
  kSoftware,
};

/// Owns an NCrypt provider or key handle and frees it with `NCryptFreeObject`.
///
/// Fakes of KeyProvider hand out real handles of the software provider, so freeing them with
/// NCrypt is right for every implementation.
class NcryptHandle {
 public:
  /// Creates an empty handle.
  NcryptHandle() = default;
  /// Takes ownership of [handle].
  explicit NcryptHandle(NCRYPT_HANDLE handle) : handle_(handle) {}
  NcryptHandle(const NcryptHandle&) = delete;
  NcryptHandle& operator=(const NcryptHandle&) = delete;
  /// Moves the ownership from [other].
  NcryptHandle(NcryptHandle&& other) noexcept : handle_(other.Release()) {}
  /// Frees the current handle and moves the ownership from [other].
  NcryptHandle& operator=(NcryptHandle&& other) noexcept {
    if (this != &other) Reset(other.Release());
    return *this;
  }
  /// Frees the handle.
  ~NcryptHandle() { Reset(); }

  /// Returns the handle without giving up ownership.
  NCRYPT_HANDLE get() const { return handle_; }
  /// Returns the address to receive a new handle; frees the current one first.
  NCRYPT_HANDLE* Out() {
    Reset();
    return &handle_;
  }
  /// Gives up ownership, for example after `NCryptDeleteKey` freed the key handle.
  NCRYPT_HANDLE Release() {
    NCRYPT_HANDLE handle = handle_;
    handle_ = 0;
    return handle;
  }
  /// Frees the current handle and takes [handle].
  void Reset(NCRYPT_HANDLE handle = 0) {
    if (handle_ != 0) NCryptFreeObject(handle_);
    handle_ = handle;
  }
  /// Whether a handle is held.
  explicit operator bool() const { return handle_ != 0; }

 private:
  NCRYPT_HANDLE handle_ = 0;
};

/// The CNG calls the device key needs; every method returns the `SECURITY_STATUS` of the call.
///
/// CngKeyProvider calls the documented NCrypt and BCrypt functions one to one; the comments name
/// the function each method stands for.
class KeyProvider {
 public:
  virtual ~KeyProvider() = default;

  /// `NCryptOpenStorageProvider` with the name of [kind].
  ///
  /// @param kind the provider to open.
  /// @param provider receives the provider handle.
  /// @return `ERROR_SUCCESS` or the failure, for example when the machine has no TPM.
  virtual SECURITY_STATUS OpenProvider(ProviderKind kind, NcryptHandle* provider) = 0;

  /// `NCryptIsAlgSupported`.
  ///
  /// @param provider an open provider.
  /// @param algorithm the CNG algorithm identifier, for example `BCRYPT_ECDSA_P256_ALGORITHM`.
  /// @return `ERROR_SUCCESS`, or `NTE_NOT_SUPPORTED` when the provider lacks the algorithm.
  virtual SECURITY_STATUS IsAlgSupported(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm) = 0;

  /// `NCryptCreatePersistedKey` for the current user (no `NCRYPT_MACHINE_KEY_FLAG`).
  ///
  /// @param provider an open provider.
  /// @param algorithm the CNG algorithm identifier.
  /// @param name the name of the persisted key.
  /// @param key receives the handle of the key, which is not usable before FinalizeKey.
  /// @return `ERROR_SUCCESS`, `NTE_EXISTS` when a key with the name exists, or the failure.
  virtual SECURITY_STATUS CreatePersistedKey(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm,
                                             LPCWSTR name, NcryptHandle* key) = 0;

  /// `NCryptSetProperty` with a `DWORD` value.
  ///
  /// @param object a key or provider handle.
  /// @param property the property identifier, for example `NCRYPT_EXPORT_POLICY_PROPERTY`.
  /// @param value the value.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual SECURITY_STATUS SetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                           DWORD value) = 0;

  /// `NCryptGetProperty` of a `DWORD` value.
  ///
  /// @param object a key or provider handle.
  /// @param property the property identifier.
  /// @param value receives the value.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual SECURITY_STATUS GetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                           DWORD* value) = 0;

  /// `NCryptFinalizeKey` with `NCRYPT_SILENT_FLAG`, so creating a key never shows a dialog.
  ///
  /// @param key a key from CreatePersistedKey.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual SECURITY_STATUS FinalizeKey(NCRYPT_KEY_HANDLE key) = 0;

  /// `NCryptOpenKey` of a key of the current user.
  ///
  /// @param provider an open provider.
  /// @param name the name of the persisted key.
  /// @param key receives the key handle.
  /// @return `ERROR_SUCCESS`, `NTE_BAD_KEYSET` when there is no such key, or the failure.
  virtual SECURITY_STATUS OpenKey(NCRYPT_PROV_HANDLE provider, LPCWSTR name,
                                  NcryptHandle* key) = 0;

  /// `NCryptExportKey` into a blob of [blob_type].
  ///
  /// @param key an open key.
  /// @param blob_type for example `BCRYPT_ECCPUBLIC_BLOB`.
  /// @param blob receives the blob.
  /// @return `ERROR_SUCCESS` or the failure (a private blob of these keys always fails).
  virtual SECURITY_STATUS ExportKey(NCRYPT_KEY_HANDLE key, LPCWSTR blob_type, Bytes* blob) = 0;

  /// `NCryptSignHash` without padding info (ECDSA).
  ///
  /// @param key an open key.
  /// @param hash the SHA-256 digest to sign.
  /// @param flags `NCRYPT_SILENT_FLAG` for a key that must never show a dialog, otherwise 0.
  /// @param signature receives `R || S` (CNG's form for ECDSA).
  /// @return `ERROR_SUCCESS`, `NTE_USER_CANCELLED`, or the failure.
  virtual SECURITY_STATUS SignHash(NCRYPT_KEY_HANDLE key, const Bytes& hash, DWORD flags,
                                   Bytes* signature) = 0;

  /// `NCryptDeleteKey`; on success the handle is freed by the call, so [key] is released.
  ///
  /// @param key an open key.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual SECURITY_STATUS DeleteKey(NcryptHandle* key) = 0;

  /// SHA-256 with `BCryptOpenAlgorithmProvider(BCRYPT_SHA256_ALGORITHM)` and `BCryptHash`.
  ///
  /// @param data the bytes to hash.
  /// @param digest receives the 32-byte digest.
  /// @return `STATUS_SUCCESS` (0) or the failing `NTSTATUS`.
  virtual SECURITY_STATUS Sha256(const Bytes& data, Bytes* digest) = 0;
};

/// Returns the CNG name of the provider [kind] (`MS_PLATFORM_CRYPTO_PROVIDER` or
/// `MS_KEY_STORAGE_PROVIDER`).
LPCWSTR ProviderName(ProviderKind kind);

}  // namespace device_key

#endif  // DEVICE_KEY_CORE_KEY_PROVIDER_H_

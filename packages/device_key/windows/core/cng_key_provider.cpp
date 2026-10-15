// SPDX-License-Identifier: Apache-2.0
//
// The CNG calls of the device key, one method per documented function
// (https://learn.microsoft.com/en-us/windows/win32/api/ncrypt/).

#include "cng_key_provider.h"

#include <utility>

namespace device_key {

namespace {

// BCrypt reports NTSTATUS values; 0 is STATUS_SUCCESS (ntstatus.h is not included with windows.h).
constexpr NTSTATUS kStatusSuccess = 0;

}  // namespace

LPCWSTR ProviderName(ProviderKind kind) {
  return kind == ProviderKind::kPlatform ? MS_PLATFORM_CRYPTO_PROVIDER : MS_KEY_STORAGE_PROVIDER;
}

SECURITY_STATUS CngKeyProvider::OpenProvider(ProviderKind kind, NcryptHandle* provider) {
  return NCryptOpenStorageProvider(reinterpret_cast<NCRYPT_PROV_HANDLE*>(provider->Out()),
                                   ProviderName(kind), 0);
}

SECURITY_STATUS CngKeyProvider::IsAlgSupported(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm) {
  return NCryptIsAlgSupported(provider, algorithm, 0);
}

SECURITY_STATUS CngKeyProvider::CreatePersistedKey(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm,
                                                   LPCWSTR name, NcryptHandle* key) {
  // dwLegacyKeySpec 0 (a CNG key) and no NCRYPT_MACHINE_KEY_FLAG: the key belongs to the user.
  return NCryptCreatePersistedKey(provider, reinterpret_cast<NCRYPT_KEY_HANDLE*>(key->Out()),
                                  algorithm, name, 0, 0);
}

SECURITY_STATUS CngKeyProvider::SetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                                 DWORD value) {
  return NCryptSetProperty(object, property, reinterpret_cast<PBYTE>(&value), sizeof(value),
                           NCRYPT_SILENT_FLAG);
}

SECURITY_STATUS CngKeyProvider::GetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                                 DWORD* value) {
  DWORD size = 0;
  SECURITY_STATUS status = NCryptGetProperty(object, property, reinterpret_cast<PBYTE>(value),
                                             sizeof(*value), &size, NCRYPT_SILENT_FLAG);
  if (status == ERROR_SUCCESS && size != sizeof(*value)) return NTE_BAD_DATA;
  return status;
}

SECURITY_STATUS CngKeyProvider::FinalizeKey(NCRYPT_KEY_HANDLE key) {
  return NCryptFinalizeKey(key, NCRYPT_SILENT_FLAG);
}

SECURITY_STATUS CngKeyProvider::OpenKey(NCRYPT_PROV_HANDLE provider, LPCWSTR name,
                                        NcryptHandle* key) {
  // dwLegacyKeySpec 0 and no NCRYPT_MACHINE_KEY_FLAG: only the user's own keys are found.
  return NCryptOpenKey(provider, reinterpret_cast<NCRYPT_KEY_HANDLE*>(key->Out()), name, 0,
                       NCRYPT_SILENT_FLAG);
}

SECURITY_STATUS CngKeyProvider::ExportKey(NCRYPT_KEY_HANDLE key, LPCWSTR blob_type, Bytes* blob) {
  DWORD size = 0;
  SECURITY_STATUS status =
      NCryptExportKey(key, 0, blob_type, nullptr, nullptr, 0, &size, NCRYPT_SILENT_FLAG);
  if (status != ERROR_SUCCESS) return status;
  blob->assign(size, 0);
  status = NCryptExportKey(key, 0, blob_type, nullptr, blob->data(), size, &size,
                           NCRYPT_SILENT_FLAG);
  if (status != ERROR_SUCCESS) {
    blob->clear();
    return status;
  }
  blob->resize(size);
  return ERROR_SUCCESS;
}

SECURITY_STATUS CngKeyProvider::SignHash(NCRYPT_KEY_HANDLE key, const Bytes& hash, DWORD flags,
                                         Bytes* signature) {
  // ECDSA takes no padding info; CNG returns the signature as R || S
  // (https://learn.microsoft.com/en-us/windows/win32/api/ncrypt/nf-ncrypt-ncryptsignhash).
  PBYTE input = const_cast<PBYTE>(hash.data());
  DWORD size = 0;
  SECURITY_STATUS status = NCryptSignHash(key, nullptr, input, static_cast<DWORD>(hash.size()),
                                          nullptr, 0, &size, flags);
  if (status != ERROR_SUCCESS) return status;
  signature->assign(size, 0);
  status = NCryptSignHash(key, nullptr, input, static_cast<DWORD>(hash.size()), signature->data(),
                          size, &size, flags);
  if (status != ERROR_SUCCESS) {
    signature->clear();
    return status;
  }
  signature->resize(size);
  return ERROR_SUCCESS;
}

SECURITY_STATUS CngKeyProvider::DeleteKey(NcryptHandle* key) {
  SECURITY_STATUS status = NCryptDeleteKey(static_cast<NCRYPT_KEY_HANDLE>(key->get()),
                                           NCRYPT_SILENT_FLAG);
  // On success NCryptDeleteKey frees the handle itself; freeing it again would be a double free.
  if (status == ERROR_SUCCESS) key->Release();
  return status;
}

SECURITY_STATUS CngKeyProvider::Sha256(const Bytes& data, Bytes* digest) {
  digest->clear();
  BCRYPT_ALG_HANDLE algorithm = nullptr;
  NTSTATUS status = BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0);
  if (status != kStatusSuccess) return status;
  // A hash object allocated by BCrypt itself (pbHashObject null, Windows 7 and later).
  BCRYPT_HASH_HANDLE hash = nullptr;
  status = BCryptCreateHash(algorithm, &hash, nullptr, 0, nullptr, 0, 0);
  if (status == kStatusSuccess) {
    status = BCryptHashData(hash, const_cast<PUCHAR>(data.data()),
                            static_cast<ULONG>(data.size()), 0);
    Bytes result(32, 0);
    if (status == kStatusSuccess) {
      status = BCryptFinishHash(hash, result.data(), static_cast<ULONG>(result.size()), 0);
    }
    if (status == kStatusSuccess) *digest = std::move(result);
    BCryptDestroyHash(hash);
  }
  BCryptCloseAlgorithmProvider(algorithm, 0);
  return status;
}

}  // namespace device_key

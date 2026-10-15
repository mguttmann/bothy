// SPDX-License-Identifier: Apache-2.0

#ifndef DEVICE_KEY_CORE_CNG_KEY_PROVIDER_H_
#define DEVICE_KEY_CORE_CNG_KEY_PROVIDER_H_

#include "key_provider.h"

namespace device_key {

/// The KeyProvider of the app: the documented NCrypt and BCrypt calls, without any logic.
///
/// Thread-safe: it keeps no state, and the NCrypt functions it calls may run on any thread.
class CngKeyProvider : public KeyProvider {
 public:
  SECURITY_STATUS OpenProvider(ProviderKind kind, NcryptHandle* provider) override;
  SECURITY_STATUS IsAlgSupported(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm) override;
  SECURITY_STATUS CreatePersistedKey(NCRYPT_PROV_HANDLE provider, LPCWSTR algorithm,
                                     LPCWSTR name, NcryptHandle* key) override;
  SECURITY_STATUS SetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                   DWORD value) override;
  SECURITY_STATUS GetDwordProperty(NCRYPT_HANDLE object, LPCWSTR property,
                                   DWORD* value) override;
  SECURITY_STATUS FinalizeKey(NCRYPT_KEY_HANDLE key) override;
  SECURITY_STATUS OpenKey(NCRYPT_PROV_HANDLE provider, LPCWSTR name, NcryptHandle* key) override;
  SECURITY_STATUS ExportKey(NCRYPT_KEY_HANDLE key, LPCWSTR blob_type, Bytes* blob) override;
  SECURITY_STATUS SignHash(NCRYPT_KEY_HANDLE key, const Bytes& hash, DWORD flags,
                           Bytes* signature) override;
  SECURITY_STATUS DeleteKey(NcryptHandle* key) override;
  SECURITY_STATUS Sha256(const Bytes& data, Bytes* digest) override;
};

}  // namespace device_key

#endif  // DEVICE_KEY_CORE_CNG_KEY_PROVIDER_H_

// SPDX-License-Identifier: Apache-2.0
//
// The markers of created keys in the Windows Credential Manager (brief M4-11, Design note 4;
// https://learn.microsoft.com/en-us/windows/win32/api/wincred/).

#include <windows.h>

#include <wincred.h>

#include <cstring>
#include <string>

#include "marker_store.h"

namespace device_key {

namespace {

constexpr char kPlatform[] = "platform";
constexpr char kSoftware[] = "software";

}  // namespace

DWORD CredentialMarkerStore::Read(const std::wstring& name, std::optional<ProviderKind>* kind) {
  kind->reset();
  PCREDENTIALW credential = nullptr;
  if (!CredReadW(name.c_str(), CRED_TYPE_GENERIC, 0, &credential)) {
    DWORD error = GetLastError();
    return error == ERROR_NOT_FOUND ? ERROR_SUCCESS : error;
  }
  std::string value(reinterpret_cast<const char*>(credential->CredentialBlob),
                    credential->CredentialBlobSize);
  CredFree(credential);
  if (value == kPlatform) {
    *kind = ProviderKind::kPlatform;
  } else if (value == kSoftware) {
    *kind = ProviderKind::kSoftware;
  } else {
    return ERROR_INVALID_DATA;
  }
  return ERROR_SUCCESS;
}

DWORD CredentialMarkerStore::Write(const std::wstring& name, ProviderKind kind) {
  const char* value = kind == ProviderKind::kPlatform ? kPlatform : kSoftware;
  std::wstring target = name;
  CREDENTIALW credential = {};
  credential.Type = CRED_TYPE_GENERIC;
  credential.TargetName = target.data();
  credential.CredentialBlobSize = static_cast<DWORD>(std::strlen(value));
  credential.CredentialBlob = reinterpret_cast<LPBYTE>(const_cast<char*>(value));
  // Local to this machine and user: an enterprise (roaming) marker could follow the profile to a
  // machine that never had the key and make it report key_invalidated there.
  credential.Persist = CRED_PERSIST_LOCAL_MACHINE;
  return CredWriteW(&credential, 0) ? ERROR_SUCCESS : GetLastError();
}

DWORD CredentialMarkerStore::Remove(const std::wstring& name) {
  if (CredDeleteW(name.c_str(), CRED_TYPE_GENERIC, 0)) return ERROR_SUCCESS;
  DWORD error = GetLastError();
  return error == ERROR_NOT_FOUND ? ERROR_SUCCESS : error;
}

}  // namespace device_key

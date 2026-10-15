// SPDX-License-Identifier: Apache-2.0
//
// The record of which keys this plugin created (brief M4-11, Design note 4).
//
// A key that is missing although its marker exists was lost (for example after a TPM clear): the
// store reports key_invalidated and never creates a new key silently, so the app registers as a
// new device (08 5.4). Without a marker a missing key is simply not created yet.

#ifndef DEVICE_KEY_CORE_MARKER_STORE_H_
#define DEVICE_KEY_CORE_MARKER_STORE_H_

#include <optional>
#include <string>

#include "key_provider.h"

namespace device_key {

/// Stores, per key name, the provider the key was created in.
class MarkerStore {
 public:
  virtual ~MarkerStore() = default;

  /// Reads the marker of the key [name].
  ///
  /// @param name the key name.
  /// @param kind receives the provider of the key, or nullopt when there is no marker.
  /// @return `ERROR_SUCCESS` or the failure (a missing marker is not a failure).
  virtual DWORD Read(const std::wstring& name, std::optional<ProviderKind>* kind) = 0;

  /// Writes the marker of the key [name], replacing an older one.
  ///
  /// @param name the key name.
  /// @param kind the provider the key was created in.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual DWORD Write(const std::wstring& name, ProviderKind kind) = 0;

  /// Removes the marker of the key [name]; a missing marker is not a failure.
  ///
  /// @param name the key name.
  /// @return `ERROR_SUCCESS` or the failure.
  virtual DWORD Remove(const std::wstring& name) = 0;
};

/// A MarkerStore in the Windows Credential Manager of the current user.
///
/// Each marker is a generic credential (`CRED_TYPE_GENERIC`) whose target name is the key name and
/// whose blob is the provider (`platform` or `software`); it holds no key material. The persistence
/// `CRED_PERSIST_LOCAL_MACHINE` keeps it on this machine for this user, so it never roams with a
/// profile (`CredWriteW`, https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw).
class CredentialMarkerStore : public MarkerStore {
 public:
  DWORD Read(const std::wstring& name, std::optional<ProviderKind>* kind) override;
  DWORD Write(const std::wstring& name, ProviderKind kind) override;
  DWORD Remove(const std::wstring& name) override;
};

}  // namespace device_key

#endif  // DEVICE_KEY_CORE_MARKER_STORE_H_

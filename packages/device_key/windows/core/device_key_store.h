// SPDX-License-Identifier: Apache-2.0

#ifndef DEVICE_KEY_CORE_DEVICE_KEY_STORE_H_
#define DEVICE_KEY_CORE_DEVICE_KEY_STORE_H_

#include <optional>
#include <string>

#include "key_provider.h"
#include "marker_store.h"

namespace device_key {

/// The purpose of a device key (docs/spec/08-identity.md 6.1, D-L10).
enum class KeyPurpose {
  /// Signs every DPoP proof; never asks the user.
  kDpop,
  /// Signs step-up confirmations; needs a confirmation per use that the system enforces.
  kStepUp,
};

/// The stable error codes of the plugin channel (brief M4-10, Design note 1; M4-11 note 4).
enum class ErrorCode {
  /// No error.
  kOk,
  /// The user cancelled the system prompt (`NTE_USER_CANCELLED`).
  kUserCancelled,
  /// A key this plugin created can no longer be opened; the app registers as a new device.
  kKeyInvalidated,
  /// No key, no usable provider, or no system-enforced confirmation for the step-up key.
  kNotAvailable,
  /// Any other failure.
  kInternal,
};

/// The result of a store operation: a code and, for failures, the Windows status behind it.
///
/// It never carries key material, a scope or signed data, so it is safe to log.
struct Status {
  /// The stable code.
  ErrorCode code = ErrorCode::kOk;
  /// The `SECURITY_STATUS`, `NTSTATUS` or Win32 error of the failing call, 0 if none.
  long hresult = 0;

  /// Whether the operation succeeded.
  bool ok() const { return code == ErrorCode::kOk; }

  /// The code sent over the channel: `user_cancelled`, `key_invalidated`, `not_available` or
  /// `internal` (empty for kOk).
  const char* wire() const;

  /// A fixed English text with the status in hex, for example
  /// `the device key operation failed (0x80090020)`; nothing else.
  std::string Message() const;

  /// A success.
  static Status Ok() { return Status{}; }
  /// A failure with [code] and the Windows status [hresult].
  static Status Fail(ErrorCode code, long hresult = 0) { return Status{code, hresult}; }
};

/// The public part of a device key.
struct KeyInfo {
  /// The x coordinate of the P-256 public key, 32 bytes big-endian.
  Bytes x;
  /// The y coordinate, 32 bytes big-endian.
  Bytes y;
  /// True for a key in the TPM (Platform Crypto Provider), false for a software key.
  bool hardware_bound = false;
  /// Whether every signature asks the user (the step-up key).
  bool requires_user_presence = false;
};

/// Creates, uses and deletes the two device keys of one Windows user for each server
/// (brief M4-11, Design note 2; docs/spec/15-client-architecture.md 9).
///
/// A new `dpop` key is created in the Platform Crypto Provider (TPM) when the provider opens and
/// `NCryptIsAlgSupported` confirms `BCRYPT_ECDSA_P256_ALGORITHM`, otherwise in the Software Key
/// Storage Provider and reported as not hardware-bound. A key stays in the provider it was
/// created in (D-M4-11-1). Keys are per user, ECDSA P-256, with export policy 0.
///
/// The `step_up` key is not offered on Windows: no Windows mechanism enforces a confirmation per
/// use of a P-256 key that code in the process cannot answer itself, so Ensure fails with
/// kNotAvailable (D-M4-11-2).
///
/// Example:
///
///     CngKeyProvider provider;
///     CredentialMarkerStore markers;
///     DeviceKeyStore store(provider, markers);
///     KeyInfo info;
///     Status status = store.Ensure("https://chat.example.com", KeyPurpose::kDpop, &info);
///
/// There is no operation that replaces a key (08 5.4): Delete removes both keys of a scope. The
/// store holds no state besides its references, so it may be used from several threads.
class DeviceKeyStore {
 public:
  /// The name prefix of the app's keys.
  static constexpr const wchar_t* kDefaultPrefix = L"device-key";

  /// Creates a store.
  ///
  /// @param provider the CNG calls; CngKeyProvider in the app, a fake in tests.
  /// @param markers the record of created keys; CredentialMarkerStore in the app.
  /// @param prefix the key name prefix; tests use one starting with `device-key-test-`.
  DeviceKeyStore(KeyProvider& provider, MarkerStore& markers,
                 std::wstring prefix = kDefaultPrefix);

  /// The key name `<prefix>.<sha256(scope) in hex>.<purpose>`, purpose `dpop` or `step_up`.
  ///
  /// Hashing keeps the server's address out of the key storage and gives every scope a fixed
  /// length; the name carries no product name (02 "Places that carry the product name").
  ///
  /// @param scope separates the keys per server, for example the server's origin.
  /// @param purpose the key's purpose.
  /// @param name receives the name.
  /// @return kOk, or kInternal when hashing fails.
  Status KeyName(const std::string& scope, KeyPurpose purpose, std::wstring* name) const;

  /// Returns the key of [purpose] for [scope], creating it when it is missing.
  ///
  /// @param scope separates the keys per server.
  /// @param purpose the key's purpose.
  /// @param info receives the public key and how it is stored.
  /// @return kOk; kNotAvailable for kStepUp or without a usable provider; kKeyInvalidated when a
  ///     key this plugin created is gone; kInternal otherwise.
  Status Ensure(const std::string& scope, KeyPurpose purpose, KeyInfo* info);

  /// Returns the key of [purpose] for [scope] without creating one.
  ///
  /// @param scope separates the keys per server.
  /// @param purpose the key's purpose.
  /// @param info receives the key, or nullopt when there is none.
  /// @return kOk; kKeyInvalidated when a key this plugin created is gone; kInternal otherwise.
  Status Info(const std::string& scope, KeyPurpose purpose, std::optional<KeyInfo>* info);

  /// Signs [data] with ES256: SHA-256 with BCrypt, then `NCryptSignHash`.
  ///
  /// The `dpop` key signs with `NCRYPT_SILENT_FLAG`, so it can never show a dialog.
  ///
  /// @param scope separates the keys per server.
  /// @param purpose the key's purpose.
  /// @param data the bytes to sign, for example the JWS signing input.
  /// @param signature receives `R || S`, 64 bytes (RFC 7518 3.4).
  /// @return kOk; kNotAvailable without a key; kKeyInvalidated; kUserCancelled; kInternal.
  Status Sign(const std::string& scope, KeyPurpose purpose, const Bytes& data, Bytes* signature);

  /// Returns the attestation of the key; Windows returns none in this version (08 5.1).
  ///
  /// @param scope separates the keys per server.
  /// @param purpose the key's purpose.
  /// @param challenge the server's challenge (unused).
  /// @param attestation receives nullopt.
  /// @return kOk; kNotAvailable without a key; kKeyInvalidated; kInternal.
  Status Attestation(const std::string& scope, KeyPurpose purpose, const Bytes& challenge,
                     std::optional<Bytes>* attestation);

  /// Deletes both keys of [scope] in both providers and their markers; missing keys are fine.
  ///
  /// @param scope separates the keys per server.
  /// @return kOk or kInternal.
  Status Delete(const std::string& scope);

 private:
  struct OpenedKey;
  Status Locate(const std::wstring& name, std::optional<OpenedKey>* key);
  Status Create(const std::wstring& name, KeyInfo* info);
  Status DescribeKey(NCRYPT_KEY_HANDLE key, bool hardware_bound, KeyPurpose purpose,
                     KeyInfo* info);

  KeyProvider& provider_;
  MarkerStore& markers_;
  std::wstring prefix_;
};

/// Whether [status] means "no such key" (`NTE_BAD_KEYSET`, `NTE_NO_KEY`, `NTE_NOT_FOUND`).
bool IsMissingKey(SECURITY_STATUS status);

}  // namespace device_key

#endif  // DEVICE_KEY_CORE_DEVICE_KEY_STORE_H_

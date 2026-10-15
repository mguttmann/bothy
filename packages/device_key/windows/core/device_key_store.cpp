// SPDX-License-Identifier: Apache-2.0
//
// The device key logic on Windows (brief M4-11, Design notes 2 to 4).

#include "device_key_store.h"

#include <cstdio>
#include <cstring>
#include <utility>

namespace device_key {

namespace {

constexpr const wchar_t* kPurposeNames[] = {L"dpop", L"step_up"};

const wchar_t* PurposeName(KeyPurpose purpose) {
  return kPurposeNames[purpose == KeyPurpose::kDpop ? 0 : 1];
}

// The size of an exported P-256 public key: BCRYPT_ECCKEY_BLOB, then X and Y of 32 bytes each.
constexpr size_t kCoordinate = 32;
constexpr size_t kPublicBlobSize = sizeof(BCRYPT_ECCKEY_BLOB) + 2 * kCoordinate;

// The JOSE form of an ES256 signature (RFC 7518 3.4), which NCryptSignHash returns for ECDSA.
constexpr size_t kSignatureSize = 64;

}  // namespace

bool IsMissingKey(SECURITY_STATUS status) {
  return status == NTE_BAD_KEYSET || status == NTE_NO_KEY || status == NTE_NOT_FOUND;
}

const char* Status::wire() const {
  switch (code) {
    case ErrorCode::kOk:
      return "";
    case ErrorCode::kUserCancelled:
      return "user_cancelled";
    case ErrorCode::kKeyInvalidated:
      return "key_invalidated";
    case ErrorCode::kNotAvailable:
      return "not_available";
    case ErrorCode::kInternal:
      break;
  }
  return "internal";
}

std::string Status::Message() const {
  const char* text = "the device key operation failed";
  switch (code) {
    case ErrorCode::kOk:
      return "ok";
    case ErrorCode::kUserCancelled:
      text = "the user cancelled the confirmation";
      break;
    case ErrorCode::kKeyInvalidated:
      text = "the device key was invalidated";
      break;
    case ErrorCode::kNotAvailable:
      text = "the device key is not available";
      break;
    case ErrorCode::kInternal:
      break;
  }
  if (hresult == 0) return text;
  char buffer[96];
  std::snprintf(buffer, sizeof(buffer), "%s (0x%08lx)", text,
                static_cast<unsigned long>(hresult) & 0xFFFFFFFFUL);
  return buffer;
}

/// A key found in one of the providers, with the provider handle it was opened through.
struct DeviceKeyStore::OpenedKey {
  NcryptHandle provider;
  NcryptHandle key;
  ProviderKind kind = ProviderKind::kSoftware;
};

DeviceKeyStore::DeviceKeyStore(KeyProvider& provider, MarkerStore& markers, std::wstring prefix)
    : provider_(provider), markers_(markers), prefix_(std::move(prefix)) {}

Status DeviceKeyStore::KeyName(const std::string& scope, KeyPurpose purpose,
                               std::wstring* name) const {
  Bytes digest;
  SECURITY_STATUS status = provider_.Sha256(Bytes(scope.begin(), scope.end()), &digest);
  if (status != 0) return Status::Fail(ErrorCode::kInternal, status);
  static constexpr wchar_t kHex[] = L"0123456789abcdef";
  std::wstring hex;
  for (uint8_t byte : digest) {
    hex.push_back(kHex[byte >> 4]);
    hex.push_back(kHex[byte & 0x0F]);
  }
  *name = prefix_ + L"." + hex + L"." + PurposeName(purpose);
  return Status::Ok();
}

Status DeviceKeyStore::Locate(const std::wstring& name, std::optional<OpenedKey>* found) {
  found->reset();
  std::optional<ProviderKind> marker;
  DWORD marker_status = markers_.Read(name, &marker);
  if (marker_status != ERROR_SUCCESS) {
    return Status::Fail(ErrorCode::kInternal, HRESULT_FROM_WIN32(marker_status));
  }
  if (marker.has_value()) {
    // The plugin created this key: it must be in the marker's provider and nowhere else.
    OpenedKey opened;
    opened.kind = *marker;
    SECURITY_STATUS status = provider_.OpenProvider(*marker, &opened.provider);
    if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kNotAvailable, status);
    status = provider_.OpenKey(opened.provider.get(), name.c_str(), &opened.key);
    if (IsMissingKey(status)) return Status::Fail(ErrorCode::kKeyInvalidated, status);
    if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, status);
    *found = std::move(opened);
    return Status::Ok();
  }
  // No marker: either no key, or the plugin stopped between NCryptFinalizeKey and writing the
  // marker. The key is used where it is (platform provider first) and never created elsewhere.
  for (ProviderKind kind : {ProviderKind::kPlatform, ProviderKind::kSoftware}) {
    OpenedKey opened;
    opened.kind = kind;
    if (provider_.OpenProvider(kind, &opened.provider) != ERROR_SUCCESS) continue;
    SECURITY_STATUS status = provider_.OpenKey(opened.provider.get(), name.c_str(), &opened.key);
    if (IsMissingKey(status)) continue;
    if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, status);
    DWORD written = markers_.Write(name, kind);
    if (written != ERROR_SUCCESS) {
      return Status::Fail(ErrorCode::kInternal, HRESULT_FROM_WIN32(written));
    }
    *found = std::move(opened);
    return Status::Ok();
  }
  return Status::Ok();
}

Status DeviceKeyStore::DescribeKey(NCRYPT_KEY_HANDLE key, bool hardware_bound, KeyPurpose purpose,
                                   KeyInfo* info) {
  Bytes blob;
  SECURITY_STATUS status = provider_.ExportKey(key, BCRYPT_ECCPUBLIC_BLOB, &blob);
  if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, status);
  BCRYPT_ECCKEY_BLOB header;
  if (blob.size() != kPublicBlobSize) return Status::Fail(ErrorCode::kInternal, NTE_BAD_DATA);
  std::memcpy(&header, blob.data(), sizeof(header));
  if (header.dwMagic != BCRYPT_ECDSA_PUBLIC_P256_MAGIC || header.cbKey != kCoordinate) {
    return Status::Fail(ErrorCode::kInternal, NTE_BAD_DATA);
  }
  // CNG writes X and Y big-endian after the header, the same form macOS returns.
  auto x = blob.begin() + sizeof(BCRYPT_ECCKEY_BLOB);
  info->x.assign(x, x + kCoordinate);
  info->y.assign(x + kCoordinate, x + 2 * kCoordinate);
  info->hardware_bound = hardware_bound;
  info->requires_user_presence = purpose == KeyPurpose::kStepUp;
  return Status::Ok();
}

Status DeviceKeyStore::Create(const std::wstring& name, KeyInfo* info) {
  // Provider choice per key at creation (D-M4-11-1): the TPM when it opens and supports P-256.
  ProviderKind kind = ProviderKind::kPlatform;
  NcryptHandle provider;
  if (provider_.OpenProvider(ProviderKind::kPlatform, &provider) != ERROR_SUCCESS ||
      provider_.IsAlgSupported(provider.get(), BCRYPT_ECDSA_P256_ALGORITHM) != ERROR_SUCCESS) {
    kind = ProviderKind::kSoftware;
    SECURITY_STATUS status = provider_.OpenProvider(ProviderKind::kSoftware, &provider);
    if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kNotAvailable, status);
  }
  NcryptHandle key;
  SECURITY_STATUS status = provider_.CreatePersistedKey(
      provider.get(), BCRYPT_ECDSA_P256_ALGORITHM, name.c_str(), &key);
  if (status != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, status);
  // Export policy 0: neither NCRYPT_ALLOW_EXPORT_FLAG nor NCRYPT_ALLOW_PLAINTEXT_EXPORT_FLAG; it can
  // only be set before NCryptFinalizeKey (Key Storage Property Identifiers).
  status = provider_.SetDwordProperty(key.get(), NCRYPT_EXPORT_POLICY_PROPERTY, 0);
  if (status == ERROR_SUCCESS) status = provider_.FinalizeKey(key.get());
  if (status != ERROR_SUCCESS) {
    provider_.DeleteKey(&key);  // a key that is not finalized is not usable; leave nothing behind
    return Status::Fail(ErrorCode::kInternal, status);
  }
  Status described = DescribeKey(key.get(), kind == ProviderKind::kPlatform, KeyPurpose::kDpop,
                                 info);
  DWORD written = described.ok() ? markers_.Write(name, kind) : ERROR_SUCCESS;
  if (!described.ok() || written != ERROR_SUCCESS) {
    provider_.DeleteKey(&key);
    return described.ok() ? Status::Fail(ErrorCode::kInternal, HRESULT_FROM_WIN32(written))
                          : described;
  }
  return Status::Ok();
}

Status DeviceKeyStore::Ensure(const std::string& scope, KeyPurpose purpose, KeyInfo* info) {
  // D-M4-11-2: no Windows mechanism enforces a confirmation per use of a P-256 key that code in
  // the process cannot give itself, and a confirmation only the plugin enforces is not enough
  // (08 6.1), so there is no step-up key on Windows.
  if (purpose == KeyPurpose::kStepUp) return Status::Fail(ErrorCode::kNotAvailable);
  std::wstring name;
  Status status = KeyName(scope, purpose, &name);
  if (!status.ok()) return status;
  std::optional<OpenedKey> found;
  status = Locate(name, &found);
  if (!status.ok()) return status;
  if (found.has_value()) {
    return DescribeKey(found->key.get(), found->kind == ProviderKind::kPlatform, purpose, info);
  }
  status = Create(name, info);
  if (status.hresult == NTE_EXISTS) {
    // Another caller created the key between Locate and Create: use that key.
    status = Locate(name, &found);
    if (!status.ok()) return status;
    if (!found.has_value()) return Status::Fail(ErrorCode::kInternal, NTE_EXISTS);
    return DescribeKey(found->key.get(), found->kind == ProviderKind::kPlatform, purpose, info);
  }
  return status;
}

Status DeviceKeyStore::Info(const std::string& scope, KeyPurpose purpose,
                            std::optional<KeyInfo>* info) {
  info->reset();
  std::wstring name;
  Status status = KeyName(scope, purpose, &name);
  if (!status.ok()) return status;
  std::optional<OpenedKey> found;
  status = Locate(name, &found);
  if (!status.ok() || !found.has_value()) return status;
  KeyInfo described;
  status = DescribeKey(found->key.get(), found->kind == ProviderKind::kPlatform, purpose,
                       &described);
  if (status.ok()) *info = std::move(described);
  return status;
}

Status DeviceKeyStore::Sign(const std::string& scope, KeyPurpose purpose, const Bytes& data,
                            Bytes* signature) {
  signature->clear();
  std::wstring name;
  Status status = KeyName(scope, purpose, &name);
  if (!status.ok()) return status;
  std::optional<OpenedKey> found;
  status = Locate(name, &found);
  if (!status.ok()) return status;
  if (!found.has_value()) return Status::Fail(ErrorCode::kNotAvailable);
  Bytes digest;
  SECURITY_STATUS result = provider_.Sha256(data, &digest);
  if (result != 0) return Status::Fail(ErrorCode::kInternal, result);
  // The DPoP key must never wait for a person: with NCRYPT_SILENT_FLAG a provider that would show
  // a dialog fails the call instead.
  DWORD flags = purpose == KeyPurpose::kDpop ? NCRYPT_SILENT_FLAG : 0;
  Bytes raw;
  result = provider_.SignHash(found->key.get(), digest, flags, &raw);
  if (result == NTE_USER_CANCELLED) return Status::Fail(ErrorCode::kUserCancelled, result);
  if (result != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, result);
  if (raw.size() != kSignatureSize) return Status::Fail(ErrorCode::kInternal, NTE_BAD_SIGNATURE);
  *signature = std::move(raw);
  return Status::Ok();
}

Status DeviceKeyStore::Attestation(const std::string& scope, KeyPurpose purpose,
                                   const Bytes& /*challenge*/, std::optional<Bytes>* attestation) {
  attestation->reset();
  std::wstring name;
  Status status = KeyName(scope, purpose, &name);
  if (!status.ok()) return status;
  std::optional<OpenedKey> found;
  status = Locate(name, &found);
  if (!status.ok()) return status;
  if (!found.has_value()) return Status::Fail(ErrorCode::kNotAvailable);
  return Status::Ok();  // TPM key attestation is out of scope of M4-11; the field stays empty
}

Status DeviceKeyStore::Delete(const std::string& scope) {
  for (KeyPurpose purpose : {KeyPurpose::kDpop, KeyPurpose::kStepUp}) {
    std::wstring name;
    Status status = KeyName(scope, purpose, &name);
    if (!status.ok()) return status;
    // Both providers, whatever the marker says, so no key of the scope survives.
    for (ProviderKind kind : {ProviderKind::kPlatform, ProviderKind::kSoftware}) {
      NcryptHandle provider;
      if (provider_.OpenProvider(kind, &provider) != ERROR_SUCCESS) continue;
      NcryptHandle key;
      SECURITY_STATUS result = provider_.OpenKey(provider.get(), name.c_str(), &key);
      if (IsMissingKey(result)) continue;
      if (result == ERROR_SUCCESS) result = provider_.DeleteKey(&key);
      if (result != ERROR_SUCCESS) return Status::Fail(ErrorCode::kInternal, result);
    }
    DWORD removed = markers_.Remove(name);
    if (removed != ERROR_SUCCESS) {
      return Status::Fail(ErrorCode::kInternal, HRESULT_FROM_WIN32(removed));
    }
  }
  return Status::Ok();
}

}  // namespace device_key

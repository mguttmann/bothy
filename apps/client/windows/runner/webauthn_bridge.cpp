// SPDX-License-Identifier: Apache-2.0
#include "webauthn_bridge.h"

#include <objbase.h>
#include <webauthn.h>

#include <cstdint>
#include <cstdio>
#include <functional>
#include <string>
#include <thread>
#include <utility>
#include <variant>
#include <vector>

#include "webauthn_mapping.h"

namespace webauthn_bridge {

namespace {

// The pure mappings (webauthn_mapping.h) must name the constants of the SDK header.
static_assert(webauthn_mapping::kUserVerificationRequired ==
              static_cast<uint32_t>(WEBAUTHN_USER_VERIFICATION_REQUIREMENT_REQUIRED));
static_assert(webauthn_mapping::kUserVerificationPreferred ==
              static_cast<uint32_t>(WEBAUTHN_USER_VERIFICATION_REQUIREMENT_PREFERRED));
static_assert(webauthn_mapping::kUserVerificationDiscouraged ==
              static_cast<uint32_t>(WEBAUTHN_USER_VERIFICATION_REQUIREMENT_DISCOURAGED));
static_assert(webauthn_mapping::kTransportUsb ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_USB));
static_assert(webauthn_mapping::kTransportNfc ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_NFC));
static_assert(webauthn_mapping::kTransportBle ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_BLE));
static_assert(webauthn_mapping::kTransportInternal ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_INTERNAL));
// Hybrid and smart card came with later SDKs than the API baseline; checked where named.
#ifdef WEBAUTHN_CTAP_TRANSPORT_HYBRID
static_assert(webauthn_mapping::kTransportHybrid ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_HYBRID));
#endif
#ifdef WEBAUTHN_CTAP_TRANSPORT_SMART_CARD
static_assert(webauthn_mapping::kTransportSmartCard ==
              static_cast<uint32_t>(WEBAUTHN_CTAP_TRANSPORT_SMART_CARD));
#endif
static_assert(webauthn_mapping::kUserCancelled ==
              static_cast<uint32_t>(NTE_USER_CANCELLED));
static_assert(webauthn_mapping::kWin32Cancelled ==
              (0x80070000u | static_cast<uint32_t>(ERROR_CANCELLED)));

using flutter::EncodableList;
using flutter::EncodableMap;
using flutter::EncodableValue;
using Result = flutter::MethodResult<EncodableValue>;
using Bytes = std::vector<uint8_t>;

// The answer of one ceremony, made on the worker thread and given to the platform thread.
struct Outcome {
  std::unique_ptr<Result> result;
  bool ok = false;
  EncodableValue value;
  std::string code;
  std::string message;
};

// The arguments of `makeCredential`, copied out of the call for the worker thread.
struct MakeArgs {
  std::string rp_id;
  std::string rp_name;
  Bytes user_id;
  std::string user_name;
  std::string user_display_name;
  Bytes client_data;
  std::vector<int64_t> algs;
  std::vector<Bytes> exclude_ids;
  DWORD timeout_ms = 0;
  bool resident_key_required = false;
  std::string user_verification;
  bool platform_only = true;
};

// The arguments of `getAssertion`, copied out of the call for the worker thread.
struct GetArgs {
  std::string rp_id;
  Bytes client_data;
  std::vector<Bytes> allow_ids;
  DWORD timeout_ms = 0;
  std::string user_verification;
};

std::wstring Wide(const std::string& text) {
  if (text.empty()) {
    return std::wstring();
  }
  const int length = MultiByteToWideChar(CP_UTF8, 0, text.data(),
                                         static_cast<int>(text.size()), nullptr, 0);
  if (length <= 0) {
    return std::wstring();
  }
  std::wstring wide(static_cast<size_t>(length), L'\0');
  MultiByteToWideChar(CP_UTF8, 0, text.data(), static_cast<int>(text.size()),
                      wide.data(), length);
  return wide;
}

std::string Narrow(PCWSTR text) {
  if (text == nullptr) {
    return std::string();
  }
  const int length =
      WideCharToMultiByte(CP_UTF8, 0, text, -1, nullptr, 0, nullptr, nullptr);
  if (length <= 1) {
    return std::string();
  }
  std::string narrow(static_cast<size_t>(length), '\0');
  WideCharToMultiByte(CP_UTF8, 0, text, -1, narrow.data(), length, nullptr,
                      nullptr);
  narrow.resize(static_cast<size_t>(length) - 1);
  return narrow;
}

const EncodableValue* Find(const EncodableMap& map, const char* key) {
  const auto it = map.find(EncodableValue(key));
  return it == map.end() ? nullptr : &it->second;
}

std::string String(const EncodableMap& map, const char* key) {
  const EncodableValue* value = Find(map, key);
  const auto* text = value != nullptr ? std::get_if<std::string>(value) : nullptr;
  return text != nullptr ? *text : std::string();
}

Bytes ByteArray(const EncodableValue* value) {
  const auto* bytes = value != nullptr ? std::get_if<Bytes>(value) : nullptr;
  return bytes != nullptr ? *bytes : Bytes();
}

bool Bool(const EncodableMap& map, const char* key, bool fallback) {
  const EncodableValue* value = Find(map, key);
  const auto* flag = value != nullptr ? std::get_if<bool>(value) : nullptr;
  return flag != nullptr ? *flag : fallback;
}

bool Int(const EncodableValue* value, int64_t* out) {
  if (value == nullptr) {
    return false;
  }
  if (const auto* narrow = std::get_if<int32_t>(value)) {
    *out = *narrow;
    return true;
  }
  if (const auto* large = std::get_if<int64_t>(value)) {
    *out = *large;
    return true;
  }
  return false;
}

std::vector<Bytes> ByteArrays(const EncodableMap& map, const char* key) {
  std::vector<Bytes> arrays;
  const EncodableValue* value = Find(map, key);
  const auto* list = value != nullptr ? std::get_if<EncodableList>(value) : nullptr;
  if (list != nullptr) {
    for (const EncodableValue& entry : *list) {
      arrays.push_back(ByteArray(&entry));
    }
  }
  return arrays;
}

std::vector<int64_t> Ints(const EncodableMap& map, const char* key) {
  std::vector<int64_t> ints;
  const EncodableValue* value = Find(map, key);
  const auto* list = value != nullptr ? std::get_if<EncodableList>(value) : nullptr;
  if (list != nullptr) {
    for (const EncodableValue& entry : *list) {
      int64_t number = 0;
      if (Int(&entry, &number)) {
        ints.push_back(number);
      }
    }
  }
  return ints;
}

DWORD Timeout(const EncodableMap& map) {
  int64_t timeout = 0;
  if (!Int(Find(map, "timeoutMs"), &timeout) || timeout <= 0) {
    return 0;  // The platform's default.
  }
  return timeout > 0xFFFFFFFFll ? 0xFFFFFFFFul : static_cast<DWORD>(timeout);
}

EncodableValue Value(const BYTE* bytes, DWORD size) {
  return EncodableValue(Bytes(bytes, bytes + size));
}

// Records the failure |hr| of |operation| in |outcome|: "cancelled" for a cancelled or
// declined prompt, otherwise "failed" with the WebAuthn error name and the HRESULT.
void Fail(Outcome& outcome, HRESULT hr, const char* operation) {
  if (webauthn_mapping::IsCancelled(static_cast<uint32_t>(hr))) {
    outcome.code = "cancelled";
    outcome.message = "Windows Hello was cancelled";
    return;
  }
  char code[16];
  std::snprintf(code, sizeof(code), "0x%08lX", static_cast<unsigned long>(hr));
  outcome.code = "failed";
  outcome.message = std::string(operation) + ": " +
                    Narrow(WebAuthNGetErrorName(hr)) + " (" + code + ")";
}

// A WEBAUTHN_CREDENTIAL_LIST over |ids|, which must outlive it; neither copied nor moved,
// because the list points into its own vectors.
class CredentialList {
 public:
  explicit CredentialList(std::vector<Bytes>& ids) : entries_(ids.size()) {
    for (size_t i = 0; i < ids.size(); ++i) {
      WEBAUTHN_CREDENTIAL_EX& entry = entries_[i];
      entry.dwVersion = WEBAUTHN_CREDENTIAL_EX_CURRENT_VERSION;
      entry.cbId = static_cast<DWORD>(ids[i].size());
      entry.pbId = ids[i].data();
      entry.pwszCredentialType = WEBAUTHN_CREDENTIAL_TYPE_PUBLIC_KEY;
      entry.dwTransports = 0;
      pointers_.push_back(&entry);
    }
    list_.cCredentials = static_cast<DWORD>(pointers_.size());
    list_.ppCredentials = pointers_.data();
  }
  CredentialList(const CredentialList&) = delete;
  CredentialList& operator=(const CredentialList&) = delete;

  // The list, or null when it is empty (no restriction).
  PWEBAUTHN_CREDENTIAL_LIST Get() { return pointers_.empty() ? nullptr : &list_; }

 private:
  std::vector<WEBAUTHN_CREDENTIAL_EX> entries_;
  std::vector<PWEBAUTHN_CREDENTIAL_EX> pointers_;
  WEBAUTHN_CREDENTIAL_LIST list_ = {};
};

WEBAUTHN_CLIENT_DATA ClientData(Bytes& client_data) {
  WEBAUTHN_CLIENT_DATA data = {};
  data.dwVersion = WEBAUTHN_CLIENT_DATA_CURRENT_VERSION;
  data.cbClientDataJSON = static_cast<DWORD>(client_data.size());
  data.pbClientDataJSON = client_data.data();
  data.pwszHashAlgId = WEBAUTHN_HASH_ALGORITHM_SHA_256;
  return data;
}

bool Available() {
  if (WebAuthNGetApiVersionNumber() < static_cast<DWORD>(WEBAUTHN_API_VERSION_1)) {
    return false;
  }
  BOOL available = FALSE;
  return SUCCEEDED(WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable(&available)) &&
         available == TRUE;
}

void MakeCredential(HWND window, MakeArgs& args, Outcome& outcome) {
  const std::wstring rp_id = Wide(args.rp_id);
  const std::wstring rp_name = Wide(args.rp_name);
  const std::wstring user_name = Wide(args.user_name);
  const std::wstring user_display_name = Wide(args.user_display_name);

  WEBAUTHN_RP_ENTITY_INFORMATION rp = {};
  rp.dwVersion = WEBAUTHN_RP_ENTITY_INFORMATION_CURRENT_VERSION;
  rp.pwszId = rp_id.c_str();
  rp.pwszName = rp_name.c_str();

  WEBAUTHN_USER_ENTITY_INFORMATION user = {};
  user.dwVersion = WEBAUTHN_USER_ENTITY_INFORMATION_CURRENT_VERSION;
  user.cbId = static_cast<DWORD>(args.user_id.size());
  user.pbId = args.user_id.data();
  user.pwszName = user_name.c_str();
  user.pwszDisplayName = user_display_name.c_str();

  std::vector<WEBAUTHN_COSE_CREDENTIAL_PARAMETER> params;
  for (const int64_t alg : args.algs) {
    WEBAUTHN_COSE_CREDENTIAL_PARAMETER param = {};
    param.dwVersion = WEBAUTHN_COSE_CREDENTIAL_PARAMETER_CURRENT_VERSION;
    param.pwszCredentialType = WEBAUTHN_CREDENTIAL_TYPE_PUBLIC_KEY;
    param.lAlg = static_cast<LONG>(alg);
    params.push_back(param);
  }
  WEBAUTHN_COSE_CREDENTIAL_PARAMETERS cose = {};
  cose.cCredentialParameters = static_cast<DWORD>(params.size());
  cose.pCredentialParameters = params.data();

  const WEBAUTHN_CLIENT_DATA client_data = ClientData(args.client_data);
  CredentialList exclude(args.exclude_ids);

  // Version 3 has every field used here and exists since WEBAUTHN_API_VERSION_1.
  WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS options = {};
  options.dwVersion = WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_3;
  options.dwTimeoutMilliseconds = args.timeout_ms;
  options.dwAuthenticatorAttachment = static_cast<DWORD>(
      args.platform_only ? WEBAUTHN_AUTHENTICATOR_ATTACHMENT_PLATFORM
                         : WEBAUTHN_AUTHENTICATOR_ATTACHMENT_ANY);
  options.bRequireResidentKey = args.resident_key_required ? TRUE : FALSE;
  options.dwUserVerificationRequirement =
      webauthn_mapping::UserVerification(args.user_verification);
  options.dwAttestationConveyancePreference =
      WEBAUTHN_ATTESTATION_CONVEYANCE_PREFERENCE_NONE;
  options.pExcludeCredentialList = exclude.Get();

  PWEBAUTHN_CREDENTIAL_ATTESTATION attestation = nullptr;
  const HRESULT hr = WebAuthNAuthenticatorMakeCredential(
      window, &rp, &user, &cose, &client_data, &options, &attestation);
  if (FAILED(hr) || attestation == nullptr) {
    if (attestation != nullptr) {
      WebAuthNFreeCredentialAttestation(attestation);
    }
    Fail(outcome, FAILED(hr) ? hr : E_UNEXPECTED, "makeCredential");
    return;
  }
  EncodableMap credential;
  credential[EncodableValue("credentialId")] =
      Value(attestation->pbCredentialId, attestation->cbCredentialId);
  credential[EncodableValue("attestationObject")] =
      Value(attestation->pbAttestationObject, attestation->cbAttestationObject);
  if (attestation->dwVersion >=
      static_cast<DWORD>(WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_3)) {
    EncodableList transports;
    for (const std::string& name :
         webauthn_mapping::TransportNames(attestation->dwUsedTransport)) {
      transports.push_back(EncodableValue(name));
    }
    if (!transports.empty()) {
      credential[EncodableValue("transports")] = EncodableValue(transports);
    }
  }
  WebAuthNFreeCredentialAttestation(attestation);
  outcome.ok = true;
  outcome.value = EncodableValue(credential);
}

void GetAssertion(HWND window, GetArgs& args, Outcome& outcome) {
  const std::wstring rp_id = Wide(args.rp_id);
  const WEBAUTHN_CLIENT_DATA client_data = ClientData(args.client_data);
  CredentialList allow(args.allow_ids);

  // Version 4 brings pAllowCredentialList and exists since WEBAUTHN_API_VERSION_1.
  WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS options = {};
  options.dwVersion = WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_4;
  options.dwTimeoutMilliseconds = args.timeout_ms;
  options.dwAuthenticatorAttachment = WEBAUTHN_AUTHENTICATOR_ATTACHMENT_ANY;
  options.dwUserVerificationRequirement =
      webauthn_mapping::UserVerification(args.user_verification);
  options.pAllowCredentialList = allow.Get();

  PWEBAUTHN_ASSERTION assertion = nullptr;
  const HRESULT hr = WebAuthNAuthenticatorGetAssertion(
      window, rp_id.c_str(), &client_data, &options, &assertion);
  if (FAILED(hr) || assertion == nullptr) {
    if (assertion != nullptr) {
      WebAuthNFreeAssertion(assertion);
    }
    Fail(outcome, FAILED(hr) ? hr : E_UNEXPECTED, "getAssertion");
    return;
  }
  EncodableMap credential;
  credential[EncodableValue("credentialId")] =
      Value(assertion->Credential.pbId, assertion->Credential.cbId);
  credential[EncodableValue("authenticatorData")] =
      Value(assertion->pbAuthenticatorData, assertion->cbAuthenticatorData);
  credential[EncodableValue("signature")] =
      Value(assertion->pbSignature, assertion->cbSignature);
  credential[EncodableValue("userHandle")] =
      assertion->cbUserId > 0 ? Value(assertion->pbUserId, assertion->cbUserId)
                              : EncodableValue();
  WebAuthNFreeAssertion(assertion);
  outcome.ok = true;
  outcome.value = EncodableValue(credential);
}

// Runs |ceremony| on a worker thread (it blocks until the person answers Windows Hello) and
// posts its outcome to |window|.
void RunAsync(HWND window, std::unique_ptr<Result> result,
              std::function<void(Outcome&)> ceremony) {
  auto* outcome = new Outcome();
  outcome->result = std::move(result);
  std::thread([window, outcome, work = std::move(ceremony)]() {
    const HRESULT com = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    work(*outcome);
    if (SUCCEEDED(com)) {
      CoUninitialize();
    }
    if (!PostMessageW(window, kDoneMessage, 0, reinterpret_cast<LPARAM>(outcome))) {
      // The window, and with it the engine, is gone. The outcome is left on purpose: its
      // result would answer through the destroyed messenger, from the wrong thread.
    }
  }).detach();
}

}  // namespace

void Handle(HWND window,
            const flutter::MethodCall<EncodableValue>& call,
            std::unique_ptr<Result> result) {
  const std::string& method = call.method_name();
  if (method == "available") {
    result->Success(EncodableValue(Available()));
    return;
  }
  if (method != "makeCredential" && method != "getAssertion") {
    result->NotImplemented();
    return;
  }
  const auto* map = std::get_if<EncodableMap>(call.arguments());
  if (map == nullptr) {
    result->Error("failed", method + ": arguments expected");
    return;
  }
  if (method == "makeCredential") {
    MakeArgs args;
    args.rp_id = String(*map, "rpId");
    args.rp_name = String(*map, "rpName");
    args.user_id = ByteArray(Find(*map, "userId"));
    args.user_name = String(*map, "userName");
    args.user_display_name = String(*map, "userDisplayName");
    args.client_data = ByteArray(Find(*map, "clientData"));
    args.algs = Ints(*map, "algs");
    args.exclude_ids = ByteArrays(*map, "excludeIds");
    args.timeout_ms = Timeout(*map);
    args.resident_key_required = Bool(*map, "residentKeyRequired", false);
    args.user_verification = String(*map, "userVerification");
    args.platform_only = Bool(*map, "platformOnly", true);
    RunAsync(window, std::move(result), [window, args](Outcome& outcome) mutable {
      MakeCredential(window, args, outcome);
    });
    return;
  }
  GetArgs args;
  args.rp_id = String(*map, "rpId");
  args.client_data = ByteArray(Find(*map, "clientData"));
  args.allow_ids = ByteArrays(*map, "allowIds");
  args.timeout_ms = Timeout(*map);
  args.user_verification = String(*map, "userVerification");
  RunAsync(window, std::move(result), [window, args](Outcome& outcome) mutable {
    GetAssertion(window, args, outcome);
  });
}

void Complete(LPARAM lparam) {
  std::unique_ptr<Outcome> outcome(reinterpret_cast<Outcome*>(lparam));
  if (outcome->ok) {
    outcome->result->Success(outcome->value);
  } else {
    outcome->result->Error(outcome->code, outcome->message);
  }
}

}  // namespace webauthn_bridge

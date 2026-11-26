// SPDX-License-Identifier: Apache-2.0

// Authenticode checks: the helper's own signature (`signature` of hello) and the signer of the
// frontmost app (`app.signer` of done). Both embedded signatures and system catalogs count, because
// most programs that ship with Windows are signed only through a catalog.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <softpub.h>
#include <wincrypt.h>
#include <wintrust.h>
#include <mscat.h>

#include <memory>
#include <string>


namespace bothy::helper::win32 {

namespace {

struct FileHandle {
  HANDLE h = INVALID_HANDLE_VALUE;
  ~FileHandle() {
    if (h != INVALID_HANDLE_VALUE) CloseHandle(h);
  }
};

// Runs WinVerifyTrust for an embedded signature or for a catalog member, without network access.
bool verify(WINTRUST_DATA& data) {
  GUID action = WINTRUST_ACTION_GENERIC_VERIFY_V2;
  data.cbStruct = sizeof data;
  data.dwUIChoice = WTD_UI_NONE;
  data.fdwRevocationChecks = WTD_REVOKE_NONE;
  data.dwStateAction = WTD_STATEACTION_VERIFY;
  data.dwProvFlags = WTD_CACHE_ONLY_URL_RETRIEVAL | WTD_REVOCATION_CHECK_NONE;
  LONG status = WinVerifyTrust(static_cast<HWND>(INVALID_HANDLE_VALUE), &action, &data);
  data.dwStateAction = WTD_STATEACTION_CLOSE;
  WinVerifyTrust(static_cast<HWND>(INVALID_HANDLE_VALUE), &action, &data);
  return status == ERROR_SUCCESS;
}

bool verify_embedded(const std::wstring& path) {
  WINTRUST_FILE_INFO file{};
  file.cbStruct = sizeof file;
  file.pcwszFilePath = path.c_str();
  WINTRUST_DATA data{};
  data.dwUnionChoice = WTD_CHOICE_FILE;
  data.pFile = &file;
  return verify(data);
}

// Finds the system catalog that lists the file; returns its path, or an empty string.
std::wstring catalog_for(const std::wstring& path, std::wstring& member_tag) {
  FileHandle f;
  f.h = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_DELETE, nullptr,
                    OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
  if (f.h == INVALID_HANDLE_VALUE) return {};
  HCATADMIN admin = nullptr;
  if (!CryptCATAdminAcquireContext2(&admin, nullptr, BCRYPT_SHA256_ALGORITHM, nullptr, 0)) return {};
  std::wstring result;
  DWORD hash_size = 0;
  CryptCATAdminCalcHashFromFileHandle2(admin, f.h, &hash_size, nullptr, 0);
  std::unique_ptr<BYTE[]> hash(new BYTE[hash_size ? hash_size : 1]);
  if (hash_size != 0 && CryptCATAdminCalcHashFromFileHandle2(admin, f.h, &hash_size, hash.get(), 0)) {
    HCATINFO info = CryptCATAdminEnumCatalogFromHash(admin, hash.get(), hash_size, 0, nullptr);
    if (info != nullptr) {
      CATALOG_INFO ci{};
      ci.cbStruct = sizeof ci;
      if (CryptCATCatalogInfoFromContext(info, &ci, 0)) {
        result = ci.wszCatalogFile;
        static const wchar_t* digits = L"0123456789ABCDEF";
        member_tag.clear();
        for (DWORD i = 0; i < hash_size; ++i) {
          member_tag.push_back(digits[hash[i] >> 4]);
          member_tag.push_back(digits[hash[i] & 0xF]);
        }
      }
      CryptCATAdminReleaseCatalogContext(admin, info, 0);
    }
  }
  CryptCATAdminReleaseContext(admin, 0);
  return result;
}

bool verify_catalog(const std::wstring& path, const std::wstring& catalog, const std::wstring& tag) {
  WINTRUST_CATALOG_INFO cat{};
  cat.cbStruct = sizeof cat;
  cat.pcwszCatalogFilePath = catalog.c_str();
  cat.pcwszMemberFilePath = path.c_str();
  cat.pcwszMemberTag = tag.c_str();
  WINTRUST_DATA data{};
  data.dwUnionChoice = WTD_CHOICE_CATALOG;
  data.pCatalog = &cat;
  return verify(data);
}

// The subject of the first signer of a signed file or catalog.
std::string subject_of(const std::wstring& signed_file) {
  HCERTSTORE store = nullptr;
  HCRYPTMSG msg = nullptr;
  DWORD encoding = 0, content = 0, format = 0;
  if (!CryptQueryObject(CERT_QUERY_OBJECT_FILE, signed_file.c_str(),
                        CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED | CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED |
                            CERT_QUERY_CONTENT_FLAG_CTL,
                        CERT_QUERY_FORMAT_FLAG_BINARY, 0, &encoding, &content, &format, &store, &msg,
                        nullptr))
    return {};
  std::string subject;
  DWORD size = 0;
  if (CryptMsgGetParam(msg, CMSG_SIGNER_INFO_PARAM, 0, nullptr, &size) && size != 0) {
    std::unique_ptr<BYTE[]> buf(new BYTE[size]);
    if (CryptMsgGetParam(msg, CMSG_SIGNER_INFO_PARAM, 0, buf.get(), &size)) {
      auto* signer = reinterpret_cast<CMSG_SIGNER_INFO*>(buf.get());
      CERT_INFO wanted{};
      wanted.Issuer = signer->Issuer;
      wanted.SerialNumber = signer->SerialNumber;
      PCCERT_CONTEXT cert = CertFindCertificateInStore(
          store, X509_ASN_ENCODING | PKCS_7_ASN_ENCODING, 0, CERT_FIND_SUBJECT_CERT, &wanted, nullptr);
      if (cert != nullptr) {
        wchar_t name[512];
        DWORD n = CertGetNameStringW(cert, CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nullptr, name,
                                     static_cast<DWORD>(sizeof name / sizeof name[0]));
        if (n > 1) subject = to_utf8(std::wstring(name, n - 1));
        CertFreeCertificateContext(cert);
      }
    }
  }
  CryptMsgClose(msg);
  CertCloseStore(store, 0);
  return subject;
}

}  // namespace

bool signature_verifies(const std::wstring& path) {
  if (verify_embedded(path)) return true;
  std::wstring tag;
  std::wstring catalog = catalog_for(path, tag);
  return !catalog.empty() && verify_catalog(path, catalog, tag);
}

std::string signer_subject(const std::wstring& path) {
  if (verify_embedded(path)) return subject_of(path);
  std::wstring tag;
  std::wstring catalog = catalog_for(path, tag);
  if (!catalog.empty() && verify_catalog(path, catalog, tag)) return subject_of(catalog);
  return {};
}

}  // namespace bothy::helper::win32

// SPDX-License-Identifier: Apache-2.0

// The helper's end of the local channel (27 9.4): a client of the host service's named pipe, with
// the server check of D-CU-02-5 and a reconnect after every break.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <aclapi.h>
#include <sddl.h>
#include <shlobj.h>

#include <memory>
#include <string>

#include "bothy/helper/trust.hpp"

namespace bothy::helper::win32 {

namespace {

// Reconnect waits: short at first, because the host service usually comes up within moments of the
// helper, then doubled up to a cap, so a stopped service costs almost nothing.
constexpr DWORD kRetryFirstMs = 250;
constexpr DWORD kRetryCapMs = 5000;
constexpr std::size_t kReadChunk = 64 * 1024;

// The user SID of a token as a string, or an empty string.
std::string token_user_sid(HANDLE token) {
  DWORD size = 0;
  GetTokenInformation(token, TokenUser, nullptr, 0, &size);
  if (size == 0) return {};
  std::unique_ptr<BYTE[]> buf(new BYTE[size]);
  if (!GetTokenInformation(token, TokenUser, buf.get(), size, &size)) return {};
  LPWSTR text = nullptr;
  if (!ConvertSidToStringSidW(reinterpret_cast<TOKEN_USER*>(buf.get())->User.Sid, &text)) return {};
  std::string out = to_utf8(text);
  LocalFree(text);
  return out;
}

std::string process_user_sid(HANDLE process) {
  HANDLE token = nullptr;
  if (!OpenProcessToken(process, TOKEN_QUERY, &token)) return {};
  std::string sid = token_user_sid(token);
  CloseHandle(token);
  return sid;
}

std::string account_sid(const wchar_t* name) {
  DWORD size = 0, domain_size = 0;
  SID_NAME_USE use{};
  LookupAccountNameW(nullptr, name, nullptr, &size, nullptr, &domain_size, &use);
  if (size == 0) return {};
  std::unique_ptr<BYTE[]> sid(new BYTE[size]);
  std::wstring domain(domain_size + 1, L'\0');
  if (!LookupAccountNameW(nullptr, name, sid.get(), &size, domain.data(), &domain_size, &use)) return {};
  LPWSTR text = nullptr;
  if (!ConvertSidToStringSidW(sid.get(), &text)) return {};
  std::string out = to_utf8(text);
  LocalFree(text);
  return out;
}

std::string pipe_owner_sid(HANDLE pipe) {
  PSID owner = nullptr;
  PSECURITY_DESCRIPTOR sd = nullptr;
  if (GetSecurityInfo(pipe, SE_KERNEL_OBJECT, OWNER_SECURITY_INFORMATION, &owner, nullptr, nullptr,
                      nullptr, &sd) != ERROR_SUCCESS)
    return {};
  std::string out;
  LPWSTR text = nullptr;
  if (owner != nullptr && ConvertSidToStringSidW(owner, &text)) {
    out = to_utf8(text);
    LocalFree(text);
  }
  LocalFree(sd);
  return out;
}

// The channel log `%LOCALAPPDATA%\bothy-computer-helper\channel.log`: one line per change of the
// channel (connected, closed, an open that fails with a new error, a refused server), so a helper
// that never reaches the host service can be diagnosed on the machine (on a test machine: the
// pipe refused GENERIC_WRITE with error 5 and nothing said so). It holds no message content and
// starts over above kChannelLogMax bytes.
constexpr LONGLONG kChannelLogMax = 256 * 1024;

void note(const std::wstring& text) {
  OutputDebugStringW((L"bothy-computer-helper: " + text + L"\n").c_str());
  PWSTR base = nullptr;
  if (FAILED(SHGetKnownFolderPath(FOLDERID_LocalAppData, KF_FLAG_CREATE, nullptr, &base))) return;
  std::wstring dir = std::wstring(base) + L"\\bothy-computer-helper";
  CoTaskMemFree(base);
  if (!CreateDirectoryW(dir.c_str(), nullptr) && GetLastError() != ERROR_ALREADY_EXISTS) return;
  std::wstring path = dir + L"\\channel.log";
  WIN32_FILE_ATTRIBUTE_DATA info{};
  bool restart = GetFileAttributesExW(path.c_str(), GetFileExInfoStandard, &info) &&
                 ((static_cast<LONGLONG>(info.nFileSizeHigh) << 32) | info.nFileSizeLow) > kChannelLogMax;
  HANDLE f = CreateFileW(path.c_str(), restart ? GENERIC_WRITE : FILE_APPEND_DATA, FILE_SHARE_READ, nullptr,
                         restart ? CREATE_ALWAYS : OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr);
  if (f == INVALID_HANDLE_VALUE) return;
  SYSTEMTIME t{};
  GetLocalTime(&t);
  wchar_t stamp[32];
  swprintf_s(stamp, L"%04u-%02u-%02uT%02u:%02u:%02u ", t.wYear, t.wMonth, t.wDay, t.wHour, t.wMinute, t.wSecond);
  std::string line = to_utf8(std::wstring(stamp) + text) + "\r\n";
  DWORD written = 0;
  WriteFile(f, line.data(), static_cast<DWORD>(line.size()), &written, nullptr);
  CloseHandle(f);
}

// kNoError marks "nothing logged yet" and kServerRefused a refused server in the change tracking.
constexpr DWORD kNoError = 0;
constexpr DWORD kServerRefused = 0xFFFFFFFF;
// A connection that ends within kShortMs was most likely refused by the host service (its log says
// why: computer_peer_refused with a reason); repeats of it are not logged.
constexpr ULONGLONG kShortMs = 5000;

}  // namespace

bool pipe_server_trusted(HANDLE pipe) {
  // Services run in session 0, a person's processes never do.
  ULONG session = 0;
  if (!GetNamedPipeServerSessionId(pipe, &session) || session != 0) return false;
  ULONG pid = 0;
  if (!GetNamedPipeServerProcessId(pipe, &pid) || pid == 0) return false;
  std::string server;
  HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
  if (process != nullptr) {
    server = process_user_sid(process);
    CloseHandle(process);
  }
  // A standard user may not open the token of a service process. The owner of the pipe object
  // comes from the creating token, and no user process can make it LocalSystem or the service
  // account; the host service sets it explicitly when it creates the pipe.
  if (server.empty()) server = pipe_owner_sid(pipe);
  // Empty strings (nothing could be opened or looked up) refuse in the rule itself.
  return pipe_server_allowed(server, process_user_sid(GetCurrentProcess()),
                             account_sid(L"NT SERVICE\\bothy-host"));
}

PipeChannel::PipeChannel(HWND window, std::wstring pipe_name, bool check_server, std::uint64_t frame_max,
                         Interrupt* interrupt)
    : window_(window),
      pipe_name_(std::move(pipe_name)),
      check_server_(check_server),
      frame_max_(frame_max),
      interrupt_(interrupt) {
  quit_event_ = CreateEventW(nullptr, TRUE, FALSE, nullptr);
}

PipeChannel::~PipeChannel() {
  quit_ = true;
  if (quit_event_ != nullptr) SetEvent(quit_event_);
  close();
  if (thread_.joinable()) thread_.join();
  if (quit_event_ != nullptr) CloseHandle(quit_event_);
}

void PipeChannel::start() {
  thread_ = std::thread([this] { run(); });
}

void PipeChannel::push(Event event) {
  {
    std::lock_guard<std::mutex> lock(mutex_);
    queue_.push_back(std::move(event));
  }
  PostMessageW(window_, kMsgChannel, 0, 0);
}

std::vector<PipeChannel::Event> PipeChannel::drain() {
  std::lock_guard<std::mutex> lock(mutex_);
  std::vector<Event> out(std::make_move_iterator(queue_.begin()), std::make_move_iterator(queue_.end()));
  queue_.clear();
  return out;
}

void PipeChannel::close() {
  std::lock_guard<std::mutex> lock(mutex_);
  // Cancelling the pending read makes the reader thread see the end, close and reconnect.
  if (pipe_ != INVALID_HANDLE_VALUE) CancelIoEx(pipe_, nullptr);
}

bool PipeChannel::write_all(const std::vector<std::uint8_t>& bytes) {
  std::lock_guard<std::mutex> lock(mutex_);
  if (pipe_ == INVALID_HANDLE_VALUE) return false;
  OVERLAPPED ov{};
  ov.hEvent = CreateEventW(nullptr, TRUE, FALSE, nullptr);
  if (ov.hEvent == nullptr) return false;
  std::size_t done = 0;
  bool ok = true;
  while (ok && done < bytes.size()) {
    DWORD chunk = static_cast<DWORD>(std::min<std::size_t>(bytes.size() - done, 0x7FFFFFFF));
    DWORD written = 0;
    ResetEvent(ov.hEvent);
    if (!WriteFile(pipe_, bytes.data() + done, chunk, nullptr, &ov) && GetLastError() != ERROR_IO_PENDING) {
      ok = false;
      break;
    }
    ok = GetOverlappedResult(pipe_, &ov, &written, TRUE) != FALSE && written > 0;
    done += written;
  }
  CloseHandle(ov.hEvent);
  if (!ok) CancelIoEx(pipe_, nullptr);  // the reader reports the break
  return ok;
}

bool PipeChannel::send(const json::Value& object) {
  std::optional<std::vector<std::uint8_t>> bytes = encode_object(object, frame_max_);
  return bytes && write_all(*bytes);
}

bool PipeChannel::send_image(const std::vector<std::uint8_t>& image) {
  std::optional<std::vector<std::uint8_t>> bytes = encode_image(image, frame_max_);
  return bytes && write_all(*bytes);
}

void PipeChannel::run() {
  DWORD wait = kRetryFirstMs;
  DWORD logged = kNoError;  // the last failure written to the channel log
  bool quiet = false;       // short connections are repeating; not logged until a long one
  std::vector<std::uint8_t> buf(kReadChunk);
  while (!quit_) {
    // SECURITY_IDENTIFICATION: the server may learn who connected but cannot act as the helper.
    // kPipeClientAccess: only the rights the pipe's DACL grants interactive users (trust.hpp).
    HANDLE h = CreateFileW(pipe_name_.c_str(), kPipeClientAccess, 0, nullptr, OPEN_EXISTING,
                           FILE_FLAG_OVERLAPPED | SECURITY_SQOS_PRESENT | SECURITY_IDENTIFICATION, nullptr);
    if (h == INVALID_HANDLE_VALUE) {
      DWORD error = GetLastError();
      if (error != logged && error != ERROR_PIPE_BUSY) {
        note(L"channel open failed: error " + std::to_wstring(error));
        logged = error;
      }
      if (error == ERROR_PIPE_BUSY) WaitNamedPipeW(pipe_name_.c_str(), wait);
      else WaitForSingleObject(quit_event_, wait);
      wait = std::min(wait * 2, kRetryCapMs);
      continue;
    }
    if (check_server_ && !pipe_server_trusted(h)) {
      // Someone else holds the endpoint name (D-CU-02-5): say nothing to it and try again later.
      if (logged != kServerRefused) note(L"pipe server refused (not the host service)");
      logged = kServerRefused;
      CloseHandle(h);
      WaitForSingleObject(quit_event_, wait);
      wait = std::min(wait * 2, kRetryCapMs);
      continue;
    }
    std::uint64_t id = 0;
    {
      std::lock_guard<std::mutex> lock(mutex_);
      pipe_ = h;
      id = ++connection_;
    }
    wait = kRetryFirstMs;
    logged = kNoError;
    if (!quiet) note(L"channel connected");
    ULONGLONG opened = GetTickCount64();
    push(Event{Event::Kind::Connected, Frame{}, id});

    FrameReader reader(frame_max_);
    OVERLAPPED ov{};
    ov.hEvent = CreateEventW(nullptr, TRUE, FALSE, nullptr);
    while (!quit_ && ov.hEvent != nullptr) {
      ResetEvent(ov.hEvent);
      DWORD got = 0;
      if (!ReadFile(h, buf.data(), static_cast<DWORD>(buf.size()), nullptr, &ov) &&
          GetLastError() != ERROR_IO_PENDING)
        break;
      HANDLE waits[2] = {ov.hEvent, quit_event_};
      if (WaitForMultipleObjects(2, waits, FALSE, INFINITE) != WAIT_OBJECT_0) {
        CancelIoEx(h, &ov);
        GetOverlappedResult(h, &ov, &got, TRUE);
        break;
      }
      if (!GetOverlappedResult(h, &ov, &got, FALSE) || got == 0) break;
      std::vector<Frame> frames;
      CodecError e = reader.feed(buf.data(), got, frames);
      for (Frame& f : frames) {
        // hide and disable stop a running action at once (27 9.6 item 1); the frame itself still
        // goes to the UI thread, which does the rest.
        if (interrupt_ != nullptr && f.type == kFrameObject) {
          const json::Value* op = f.object.find("op");
          const json::Value* sid = f.object.find("session_id");
          if (op != nullptr && op->is_string() && op->as_string() == "hide" && sid != nullptr &&
              sid->is_string())
            interrupt_->raise_hide(sid->as_string());
          if (op != nullptr && op->is_string() && op->as_string() == "disable")
            interrupt_->raise(Interrupt::Kind::Disable);
        }
        push(Event{Event::Kind::Frame, std::move(f), id});
      }
      if (e != CodecError::None) break;  // a longer or malformed frame ends the connection (D-CU-02-8)
    }
    if (ov.hEvent != nullptr) CloseHandle(ov.hEvent);
    {
      std::lock_guard<std::mutex> lock(mutex_);
      pipe_ = INVALID_HANDLE_VALUE;
    }
    CloseHandle(h);
    if (GetTickCount64() - opened >= kShortMs) {
      note(L"channel closed");
      quiet = false;
    } else if (!quiet) {
      note(L"channel closed at once (refused by the host service? see its log); repeats are not logged");
      quiet = true;
    }
    if (interrupt_ != nullptr) interrupt_->raise(Interrupt::Kind::ChannelBroken);
    push(Event{Event::Kind::Broken, Frame{}, id});
  }
}

}  // namespace bothy::helper::win32

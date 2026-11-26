// SPDX-License-Identifier: Apache-2.0

// The helper's end of the local channel (27 9.4): a client of the host service's named pipe, with
// the server check of D-CU-02-5 and a reconnect after every break.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <sddl.h>

#include <memory>

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

}  // namespace

bool pipe_server_trusted(HANDLE pipe) {
  ULONG pid = 0;
  if (!GetNamedPipeServerProcessId(pipe, &pid) || pid == 0) return false;
  std::string server;
  HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
  if (process != nullptr) {
    server = process_user_sid(process);
    CloseHandle(process);
  }
  // Empty strings (nothing could be opened or looked up) refuse in the rule itself.
  return pipe_server_allowed(server, process_user_sid(GetCurrentProcess()),
                             account_sid(L"NT SERVICE\\bothy-host"));
}

PipeChannel::PipeChannel(HWND window, std::wstring pipe_name, bool check_server, std::uint64_t frame_max)
    : window_(window), pipe_name_(std::move(pipe_name)), check_server_(check_server), frame_max_(frame_max) {
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
  std::vector<std::uint8_t> buf(kReadChunk);
  while (!quit_) {
    // SECURITY_IDENTIFICATION: the server may learn who connected but cannot act as the helper.
    HANDLE h = CreateFileW(pipe_name_.c_str(), GENERIC_READ | GENERIC_WRITE, 0, nullptr, OPEN_EXISTING,
                           FILE_FLAG_OVERLAPPED | SECURITY_SQOS_PRESENT | SECURITY_IDENTIFICATION, nullptr);
    if (h == INVALID_HANDLE_VALUE) {
      if (GetLastError() == ERROR_PIPE_BUSY) WaitNamedPipeW(pipe_name_.c_str(), wait);
      else WaitForSingleObject(quit_event_, wait);
      wait = std::min(wait * 2, kRetryCapMs);
      continue;
    }
    if (check_server_ && !pipe_server_trusted(h)) {
      // Someone else holds the endpoint name (D-CU-02-5): say nothing to it and try again later.
      OutputDebugStringW(L"bothy-computer-helper: pipe server refused (not the host service)\n");
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
      for (Frame& f : frames) push(Event{Event::Kind::Frame, std::move(f), id});
      if (e != CodecError::None) break;  // a longer or malformed frame ends the connection (D-CU-02-8)
    }
    if (ov.hEvent != nullptr) CloseHandle(ov.hEvent);
    {
      std::lock_guard<std::mutex> lock(mutex_);
      pipe_ = INVALID_HANDLE_VALUE;
    }
    CloseHandle(h);
    push(Event{Event::Kind::Broken, Frame{}, id});
  }
}

}  // namespace bothy::helper::win32

// SPDX-License-Identifier: Apache-2.0
//
// The Flutter plugin device_key on Windows (brief M4-11, Design note 1): the Pigeon host API over
// DeviceKeyStore.

#include "device_key_plugin.h"

#include <utility>

#include "include/device_key/device_key_plugin_c_api.h"

namespace device_key {

namespace {

// The private window message that carries a task (a heap-allocated std::function) in LPARAM.
constexpr UINT kRunTaskMessage = WM_APP + 1;
constexpr wchar_t kWindowClass[] = L"DeviceKeyPlatformThreadRunner";

KeyPurpose ToPurpose(KeyPurposeMessage purpose) {
  return purpose == KeyPurposeMessage::kDpop ? KeyPurpose::kDpop : KeyPurpose::kStepUp;
}

FlutterError ToError(const Status& status) {
  return FlutterError(status.wire(), status.Message());
}

DeviceKeyInfoMessage ToMessage(const device_key::KeyInfo& info) {
  return DeviceKeyInfoMessage(info.x, info.y, info.hardware_bound, info.requires_user_presence);
}

}  // namespace

PlatformThreadRunner::PlatformThreadRunner() {
  HINSTANCE instance = GetModuleHandle(nullptr);
  WNDCLASSW window_class = {};
  window_class.lpfnWndProc = &PlatformThreadRunner::WindowProc;
  window_class.hInstance = instance;
  window_class.lpszClassName = kWindowClass;
  RegisterClassW(&window_class);  // fails harmlessly when a second engine registered it already
  window_ = CreateWindowExW(0, kWindowClass, L"", 0, 0, 0, 0, 0, HWND_MESSAGE, nullptr, instance,
                            nullptr);
}

PlatformThreadRunner::~PlatformThreadRunner() {
  if (window_ == nullptr) return;
  // Tasks still queued belong to an engine that is shutting down: free them without running.
  MSG message;
  while (PeekMessageW(&message, window_, kRunTaskMessage, kRunTaskMessage, PM_REMOVE)) {
    delete reinterpret_cast<std::function<void()>*>(message.lParam);
  }
  DestroyWindow(window_);
}

void PlatformThreadRunner::Post(std::function<void()> task) {
  auto* boxed = new std::function<void()>(std::move(task));
  if (window_ == nullptr ||
      !PostMessageW(window_, kRunTaskMessage, 0, reinterpret_cast<LPARAM>(boxed))) {
    delete boxed;
  }
}

LRESULT CALLBACK PlatformThreadRunner::WindowProc(HWND window, UINT message, WPARAM wparam,
                                                  LPARAM lparam) {
  if (message == kRunTaskMessage) {
    std::unique_ptr<std::function<void()>> task(reinterpret_cast<std::function<void()>*>(lparam));
    (*task)();
    return 0;
  }
  return DefWindowProcW(window, message, wparam, lparam);
}

void DeviceKeyPlugin::RegisterWithRegistrar(flutter::PluginRegistrarWindows* registrar) {
  auto plugin = std::make_unique<DeviceKeyPlugin>();
  DeviceKeyHostApi::SetUp(registrar->messenger(), plugin.get());
  registrar->AddPlugin(std::move(plugin));
}

DeviceKeyPlugin::DeviceKeyPlugin()
    : store_(provider_, markers_), worker_([this] { RunWorker(); }) {}

DeviceKeyPlugin::~DeviceKeyPlugin() {
  {
    std::lock_guard<std::mutex> lock(mutex_);
    stopping_ = true;
    jobs_.clear();
  }
  wake_.notify_all();
  if (worker_.joinable()) worker_.join();
}

void DeviceKeyPlugin::RunWorker() {
  for (;;) {
    std::function<void()> job;
    {
      std::unique_lock<std::mutex> lock(mutex_);
      wake_.wait(lock, [this] { return stopping_ || !jobs_.empty(); });
      if (stopping_) return;
      job = std::move(jobs_.front());
      jobs_.pop_front();
    }
    job();
  }
}

ErrorOr<DeviceKeyInfoMessage> DeviceKeyPlugin::EnsureKey(const std::string& scope,
                                                         const KeyPurposeMessage& purpose) {
  device_key::KeyInfo info;
  Status status = store_.Ensure(scope, ToPurpose(purpose), &info);
  if (!status.ok()) return ToError(status);
  return ToMessage(info);
}

ErrorOr<std::optional<DeviceKeyInfoMessage>> DeviceKeyPlugin::KeyInfo(
    const std::string& scope, const KeyPurposeMessage& purpose) {
  std::optional<device_key::KeyInfo> info;
  Status status = store_.Info(scope, ToPurpose(purpose), &info);
  if (!status.ok()) return ToError(status);
  if (!info.has_value()) return std::optional<DeviceKeyInfoMessage>();
  return std::optional<DeviceKeyInfoMessage>(ToMessage(*info));
}

void DeviceKeyPlugin::Sign(const std::string& scope, const KeyPurposeMessage& purpose,
                           const std::vector<uint8_t>& data,
                           const PromptTextsMessage* /*prompt*/,
                           std::function<void(ErrorOr<std::vector<uint8_t>> reply)> result) {
  // TPM operations block for up to seconds, so they run on the worker; the reply goes back to
  // the platform thread, where Flutter expects channel replies.
  auto job = [this, scope, key_purpose = ToPurpose(purpose), data,
              result = std::move(result)]() mutable {
    Bytes signature;
    Status status = store_.Sign(scope, key_purpose, data, &signature);
    runner_.Post([status, signature = std::move(signature), result = std::move(result)]() {
      if (status.ok()) {
        result(signature);
      } else {
        result(ToError(status));
      }
    });
  };
  {
    std::lock_guard<std::mutex> lock(mutex_);
    jobs_.push_back(std::move(job));
  }
  wake_.notify_one();
}

ErrorOr<std::optional<std::vector<uint8_t>>> DeviceKeyPlugin::Attestation(
    const std::string& scope, const KeyPurposeMessage& purpose,
    const std::vector<uint8_t>& challenge) {
  std::optional<Bytes> attestation;
  Status status = store_.Attestation(scope, ToPurpose(purpose), challenge, &attestation);
  if (!status.ok()) return ToError(status);
  return attestation;
}

std::optional<FlutterError> DeviceKeyPlugin::DeleteKeys(const std::string& scope) {
  Status status = store_.Delete(scope);
  if (!status.ok()) return ToError(status);
  return std::nullopt;
}

}  // namespace device_key

void DeviceKeyPluginCApiRegisterWithRegistrar(FlutterDesktopPluginRegistrarRef registrar) {
  device_key::DeviceKeyPlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}

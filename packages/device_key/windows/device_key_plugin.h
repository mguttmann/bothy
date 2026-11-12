// SPDX-License-Identifier: Apache-2.0

#ifndef FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_H_
#define FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_H_

#include <windows.h>

#include <flutter/plugin_registrar_windows.h>

#include <condition_variable>
#include <deque>
#include <functional>
#include <memory>
#include <mutex>
#include <thread>

#include "cng_key_provider.h"
#include "device_key_api.g.h"
#include "device_key_store.h"
#include "marker_store.h"

namespace device_key {

/// Runs tasks on the platform thread: a message-only window created there receives them through
/// `PostMessage`, so a reply computed on a worker thread reaches Flutter on the thread it expects.
class PlatformThreadRunner {
 public:
  /// Creates the window; must run on the platform thread.
  PlatformThreadRunner();
  /// Destroys the window and drops the tasks not run yet.
  ~PlatformThreadRunner();
  PlatformThreadRunner(const PlatformThreadRunner&) = delete;
  PlatformThreadRunner& operator=(const PlatformThreadRunner&) = delete;

  /// Queues [task] for the platform thread; callable from any thread.
  ///
  /// @param task the work to run, for example a Pigeon reply.
  void Post(std::function<void()> task);

 private:
  static LRESULT CALLBACK WindowProc(HWND window, UINT message, WPARAM wparam, LPARAM lparam);
  HWND window_ = nullptr;
};

/// The device_key plugin on Windows: connects the Pigeon interface to DeviceKeyStore
/// (brief M4-11, Design note 1).
///
/// No call works on the platform thread: creating a key in the TPM and signing with it can block
/// for seconds, and every other call opens providers and keys as well. Each call becomes a job of
/// the plugin's one worker thread, in the order of the calls, and answers through
/// PlatformThreadRunner on the platform thread, where Flutter expects channel replies
/// (docs/decisions.d/AUD-pkg-device-key-01.md). Every error leaves as a FlutterError with a stable
/// code and a fixed text.
class DeviceKeyPlugin : public flutter::Plugin, public DeviceKeyHostApi {
 public:
  /// Creates the plugin and registers it on [registrar]'s messenger.
  ///
  /// @param registrar the registrar Flutter passes when the app starts.
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows* registrar);

  /// Creates a plugin over the CNG providers and the Credential Manager markers.
  DeviceKeyPlugin();
  /// Waits for the running job, then stops the worker thread; jobs not started are dropped.
  ~DeviceKeyPlugin() override;
  DeviceKeyPlugin(const DeviceKeyPlugin&) = delete;
  DeviceKeyPlugin& operator=(const DeviceKeyPlugin&) = delete;

  /// See DeviceKeyHostApi::EnsureKey; the reply arrives on the platform thread.
  void EnsureKey(const std::string& scope, const KeyPurposeMessage& purpose,
                 std::function<void(ErrorOr<DeviceKeyInfoMessage> reply)> result) override;
  /// See DeviceKeyHostApi::KeyInfo; the reply arrives on the platform thread.
  void KeyInfo(
      const std::string& scope, const KeyPurposeMessage& purpose,
      std::function<void(ErrorOr<std::optional<DeviceKeyInfoMessage>> reply)> result) override;
  /// See DeviceKeyHostApi::Sign; the reply arrives on the platform thread. Windows ignores
  /// prompt, the texts of the Android step-up prompt: it has no step-up key (D-M4-11-2).
  void Sign(const std::string& scope, const KeyPurposeMessage& purpose,
            const std::vector<uint8_t>& data, const PromptTextsMessage* prompt,
            std::function<void(ErrorOr<std::vector<uint8_t>> reply)> result) override;
  /// See DeviceKeyHostApi::Attestation; Windows returns none in this version. The reply arrives
  /// on the platform thread.
  void Attestation(
      const std::string& scope, const KeyPurposeMessage& purpose,
      const std::vector<uint8_t>& challenge,
      std::function<void(ErrorOr<std::optional<std::vector<uint8_t>>> reply)> result) override;
  /// See DeviceKeyHostApi::DeleteKeys; the reply arrives on the platform thread.
  void DeleteKeys(const std::string& scope,
                  std::function<void(std::optional<FlutterError> reply)> result) override;

 private:
  void RunWorker();
  /// Queues [job] for the worker thread.
  void Enqueue(std::function<void()> job);

  CngKeyProvider provider_;
  CredentialMarkerStore markers_;
  DeviceKeyStore store_;
  PlatformThreadRunner runner_;
  std::mutex mutex_;
  std::condition_variable wake_;
  std::deque<std::function<void()>> jobs_;
  bool stopping_ = false;
  std::thread worker_;
};

}  // namespace device_key

#endif  // FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_H_

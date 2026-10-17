// SPDX-License-Identifier: Apache-2.0
#ifndef RUNNER_FLUTTER_WINDOW_H_
#define RUNNER_FLUTTER_WINDOW_H_

#include <flutter/dart_project.h>
#include <flutter/encodable_value.h>
#include <flutter/flutter_view_controller.h>
#include <flutter/method_channel.h>

#include <memory>
#include <optional>
#include <string>
#include <vector>

#include "win32_window.h"

// A window that hosts a Flutter view and forwards deep links to Dart over the channel
// `bothy/deep_links` (deep_link.h).
class FlutterWindow : public Win32Window {
 public:
  // Creates a new FlutterWindow hosting a Flutter view running |project|; |link| is the deep
  // link the app was started with, kept until Dart asks for it.
  FlutterWindow(const flutter::DartProject& project,
                std::optional<std::string> link);
  virtual ~FlutterWindow();

 protected:
  // Win32Window:
  bool OnCreate() override;
  void OnDestroy() override;
  LRESULT MessageHandler(HWND window, UINT const message, WPARAM const wparam,
                         LPARAM const lparam) noexcept override;

 private:
  // The project to run.
  flutter::DartProject project_;

  // The Flutter instance hosted by this window.
  std::unique_ptr<flutter::FlutterViewController> flutter_controller_;

  // The channel of the deep links and the links Dart has not taken yet.
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> deep_links_;
  std::vector<std::string> pending_links_;
};

#endif  // RUNNER_FLUTTER_WINDOW_H_

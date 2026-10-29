// SPDX-License-Identifier: Apache-2.0
#include "flutter_window.h"

#include <flutter/standard_method_codec.h>

#include <optional>
#include <string>
#include <variant>

#include "deep_link.h"
#include "flutter/generated_plugin_registrant.h"
#include "rich_clipboard.h"

FlutterWindow::FlutterWindow(const flutter::DartProject& project,
                             std::optional<std::string> link)
    : project_(project) {
  if (link) {
    pending_links_.push_back(*link);
  }
}

FlutterWindow::~FlutterWindow() {}

bool FlutterWindow::OnCreate() {
  if (!Win32Window::OnCreate()) {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(
      frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view()) {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());
  deep_links_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(), "bothy/deep_links",
      &flutter::StandardMethodCodec::GetInstance());
  deep_links_->SetMethodCallHandler(
      [this](const flutter::MethodCall<flutter::EncodableValue>& call,
             std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
        if (call.method_name() != "takePending") {
          result->NotImplemented();
          return;
        }
        flutter::EncodableList links;
        for (const std::string& link : pending_links_) {
          links.push_back(flutter::EncodableValue(link));
        }
        pending_links_.clear();
        result->Success(flutter::EncodableValue(links));
      });
  clipboard_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(), "bothy/clipboard",
      &flutter::StandardMethodCodec::GetInstance());
  clipboard_->SetMethodCallHandler(
      [this](const flutter::MethodCall<flutter::EncodableValue>& call,
             std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
        if (call.method_name() == "readHtml") {
          const std::optional<std::string> html = rich_clipboard::ReadHtml(GetHandle());
          result->Success(html ? flutter::EncodableValue(*html) : flutter::EncodableValue());
          return;
        }
        if (call.method_name() != "write") {
          result->NotImplemented();
          return;
        }
        const auto* args = std::get_if<flutter::EncodableMap>(call.arguments());
        const std::string* text = nullptr;
        const std::string* cf_html = nullptr;
        if (args != nullptr) {
          auto t = args->find(flutter::EncodableValue("text"));
          auto h = args->find(flutter::EncodableValue("cf_html"));
          if (t != args->end()) text = std::get_if<std::string>(&t->second);
          if (h != args->end()) cf_html = std::get_if<std::string>(&h->second);
        }
        if (text == nullptr || cf_html == nullptr) {
          result->Error("clipboard", "text and cf_html expected");
          return;
        }
        if (!rich_clipboard::Write(GetHandle(), *text, *cf_html)) {
          result->Error("clipboard", "the clipboard did not take the copy");
          return;
        }
        result->Success();
      });
  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  flutter_controller_->engine()->SetNextFrameCallback([&]() {
    this->Show();
  });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy() {
  deep_links_ = nullptr;
  clipboard_ = nullptr;
  if (flutter_controller_) {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message,
                              WPARAM const wparam,
                              LPARAM const lparam) noexcept {
  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_) {
    std::optional<LRESULT> result =
        flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam,
                                                      lparam);
    if (result) {
      return *result;
    }
  }

  switch (message) {
    case WM_FONTCHANGE:
      flutter_controller_->engine()->ReloadSystemFonts();
      break;
    case WM_COPYDATA: {
      // A link from a second start of the app (deep_link.h).
      const auto* data = reinterpret_cast<const COPYDATASTRUCT*>(lparam);
      if (data != nullptr && data->dwData == deep_link::kCopyDataId &&
          deep_links_ != nullptr) {
        std::string link(static_cast<const char*>(data->lpData), data->cbData);
        deep_links_->InvokeMethod(
            "open", std::make_unique<flutter::EncodableValue>(link));
        return TRUE;
      }
      break;
    }
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}

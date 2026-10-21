// SPDX-License-Identifier: Apache-2.0
#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>
#include <windows.h>

#include "branding.h"
#include "deep_link.h"
#include "flutter_window.h"
#include "utils.h"

int APIENTRY wWinMain(_In_ HINSTANCE instance, _In_opt_ HINSTANCE prev,
                      _In_ wchar_t *command_line, _In_ int show_command) {
  // A link while the app runs goes to the running window (docs/DECISIONS.md D-M4-18-4).
  std::vector<std::string> command_line_arguments =
      GetCommandLineArguments();
  std::optional<std::string> link = deep_link::FindLink(command_line_arguments);
  if (link && deep_link::ForwardToRunningInstance(*link)) {
    return EXIT_SUCCESS;
  }
  // A second start without a link shows the running app, which may be hidden in the
  // notification area (15 10.3), instead of a second instance with the same device.
  if (!link && deep_link::ActivateRunningInstance()) {
    return EXIT_SUCCESS;
  }
  deep_link::RegisterUrlScheme();

  // Attach to console when present (e.g., 'flutter run') or create a
  // new console when running with a debugger.
  if (!::AttachConsole(ATTACH_PARENT_PROCESS) && ::IsDebuggerPresent()) {
    CreateAndAttachConsole();
  }

  // Initialize COM, so that it is available for use in the library and/or
  // plugins.
  ::CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);

  flutter::DartProject project(L"data");

  project.set_dart_entrypoint_arguments(std::move(command_line_arguments));

  FlutterWindow window(project, link);
  Win32Window::Point origin(10, 10);
  Win32Window::Size size(1280, 720);
  if (!window.Create(BRANDING_APP_NAME_W, origin, size)) {
    return EXIT_FAILURE;
  }
  window.SetQuitOnClose(true);

  ::MSG msg;
  while (::GetMessage(&msg, nullptr, 0, 0)) {
    ::TranslateMessage(&msg);
    ::DispatchMessage(&msg);
  }

  ::CoUninitialize();
  return EXIT_SUCCESS;
}

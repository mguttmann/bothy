// SPDX-License-Identifier: Apache-2.0
//
// The C entry point Flutter's generated plugin registrant calls on Windows (pubspec.yaml
// `pluginClass: DeviceKeyPluginCApi`).

#ifndef FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_C_API_H_
#define FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_C_API_H_

#include <flutter_plugin_registrar.h>

#ifdef FLUTTER_PLUGIN_IMPL
#define FLUTTER_PLUGIN_EXPORT __declspec(dllexport)
#else
#define FLUTTER_PLUGIN_EXPORT __declspec(dllimport)
#endif

#if defined(__cplusplus)
extern "C" {
#endif

/// Registers the device_key plugin with the app's plugin registrar.
///
/// @param registrar the registrar Flutter passes when the app starts.
FLUTTER_PLUGIN_EXPORT void DeviceKeyPluginCApiRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar);

#if defined(__cplusplus)
}  // extern "C"
#endif

#endif  // FLUTTER_PLUGIN_DEVICE_KEY_PLUGIN_C_API_H_

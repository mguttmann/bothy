# SPDX-License-Identifier: Apache-2.0
#
# The Windows part of the plugin device_key (brief M4-11), in the layout of the Flutter 3.47
# plugin template. The key logic is the static library device_key_core (core/), which has no
# Flutter dependency and its own CTest suite (platform-tests.json).
cmake_minimum_required(VERSION 3.14)

set(PROJECT_NAME "device_key")
project(${PROJECT_NAME} LANGUAGES CXX)

# This value is used when generating builds using this plugin, so it must not be changed.
set(PLUGIN_NAME "device_key_plugin")

add_subdirectory(core)

list(APPEND PLUGIN_SOURCES
  "device_key_plugin.cpp"
  "device_key_plugin.h"
  "device_key_api.g.cpp"
  "device_key_api.g.h"
)

add_library(${PLUGIN_NAME} SHARED
  "include/device_key/device_key_plugin_c_api.h"
  ${PLUGIN_SOURCES}
)

# Flutter's default build settings of the app (warnings as errors, C++17, no STL exceptions).
apply_standard_settings(${PLUGIN_NAME})

set_target_properties(${PLUGIN_NAME} PROPERTIES CXX_VISIBILITY_PRESET hidden)
target_compile_definitions(${PLUGIN_NAME} PRIVATE FLUTTER_PLUGIN_IMPL)

target_include_directories(${PLUGIN_NAME} INTERFACE "${CMAKE_CURRENT_SOURCE_DIR}/include")
target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin device_key_core)

# No libraries besides the plugin DLL need bundling.
set(device_key_bundled_libraries
  ""
  PARENT_SCOPE
)

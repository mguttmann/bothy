# SPDX-License-Identifier: Apache-2.0
#
# device_key_core: the Windows device key logic over CNG, without a Flutter dependency (brief
# M4-11, Design note 2). The plugin (../CMakeLists.txt) links it; built on its own
# (`cmake -S windows/core -B build/core-tests`, platform-tests.json) it also builds the CTest suite.
cmake_minimum_required(VERSION 3.14)
project(device_key_core LANGUAGES CXX)

add_library(device_key_core STATIC
  cng_key_provider.cpp
  credential_marker_store.cpp
  device_key_store.cpp
)
target_compile_features(device_key_core PUBLIC cxx_std_17)
target_include_directories(device_key_core PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}")
target_compile_definitions(device_key_core PUBLIC UNICODE _UNICODE NOMINMAX WIN32_LEAN_AND_MEAN)
# ncrypt: key storage providers; bcrypt: SHA-256; advapi32: the Credential Manager (markers).
target_link_libraries(device_key_core PUBLIC ncrypt bcrypt advapi32)
if(MSVC)
  # The same exception setting as Flutter's apply_standard_settings for the plugin.
  target_compile_definitions(device_key_core PRIVATE _HAS_EXCEPTIONS=0)
  target_compile_options(device_key_core PRIVATE /W4 /WX /EHsc)
endif()

if(CMAKE_SOURCE_DIR STREQUAL CMAKE_CURRENT_SOURCE_DIR)
  enable_testing()
  add_subdirectory(tests)
endif()

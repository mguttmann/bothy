# SPDX-License-Identifier: Apache-2.0
#
# bothy-computer-helper for Windows (docs/spec/27-computer-use.md section 9.2, D-CU-02-1).
#
# helper_core   portable static library (standard C++ only): JSON, channel codec, key names,
#               geometry, settings files and the Controller behind the platform interfaces.
# helper_core_tests  the core's tests with the project's own runner (test/check.hpp, D-CU-02-2),
#               one CTest test per CHECK_TEST.
# bothy-computer-helper.exe  the Win32 executable, built only on Windows (or a Windows cross
#               toolchain) and only without HELPER_CORE_ONLY.

cmake_minimum_required(VERSION 3.24)
project(bothy_computer_helper LANGUAGES CXX)

set(HELPER_VERSION "0.0.0-dev" CACHE STRING "Version the helper reports in hello and prints with --version")
option(HELPER_CORE_ONLY "Build only the portable core and its tests (no Win32 sources)" OFF)
option(HELPER_UIACCESS "Write uiAccess=\"true\" into the manifest; OFF for unsigned development and CI builds, which Windows refuses to start with uiAccess outside Program Files" ON)
option(HELPER_BUILD_TESTS "Build the core tests and register them with CTest" ON)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

if(MSVC)
  set(HELPER_WARNINGS /W4 /WX /permissive- /utf-8)
else()
  set(HELPER_WARNINGS -Wall -Wextra -Wpedantic -Werror -Wconversion -Wno-sign-conversion)
endif()

add_library(helper_core STATIC
  src/core/json.cpp
  src/core/utf.cpp
  src/core/codec.cpp
  src/core/keys.cpp
  src/core/geometry.cpp
  src/core/escape.cpp
  src/core/settings.cpp
  src/core/trust.cpp
  src/core/controller.cpp
)
target_include_directories(helper_core PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include)
target_compile_options(helper_core PRIVATE ${HELPER_WARNINGS})

if(HELPER_BUILD_TESTS)
  enable_testing()
  set(HELPER_TEST_SOURCES
    test/test_json.cpp
    test/test_codec.cpp
    test/test_small.cpp
    test/test_controller.cpp
  )
  add_executable(helper_core_tests test/main.cpp ${HELPER_TEST_SOURCES})
  target_link_libraries(helper_core_tests PRIVATE helper_core)
  target_compile_options(helper_core_tests PRIVATE ${HELPER_WARNINGS})
  # The shared vectors of the channel (CU-01); every codec of the channel must read them.
  get_filename_component(HELPER_VECTORS
    "${CMAKE_CURRENT_SOURCE_DIR}/../../testdata/computer/channel-vectors.json" ABSOLUTE)
  target_compile_definitions(helper_core_tests PRIVATE CHANNEL_VECTORS_PATH="${HELPER_VECTORS}")

  # One CTest test per CHECK_TEST, read from the sources, so a new test needs no CMake edit.
  foreach(source IN LISTS HELPER_TEST_SOURCES)
    file(STRINGS "${CMAKE_CURRENT_SOURCE_DIR}/${source}" lines REGEX "^CHECK_TEST\\(")
    foreach(line IN LISTS lines)
      string(REGEX REPLACE "^CHECK_TEST\\(([A-Za-z0-9_]+)\\).*$" "\\1" name "${line}")
      add_test(NAME core.${name} COMMAND helper_core_tests ${name})
    endforeach()
  endforeach()
endif()

if(WIN32 AND NOT HELPER_CORE_ONLY)
  if(HELPER_UIACCESS)
    set(HELPER_UIACCESS_VALUE "true")
  else()
    set(HELPER_UIACCESS_VALUE "false")
  endif()
  configure_file(src/win32/bothy-computer-helper.manifest.in
    ${CMAKE_CURRENT_BINARY_DIR}/bothy-computer-helper.manifest @ONLY)
  configure_file(src/win32/resources.rc.in ${CMAKE_CURRENT_BINARY_DIR}/resources.rc @ONLY)

  add_executable(bothy-computer-helper WIN32
    src/win32/main.cpp
    src/win32/wide.cpp
    src/win32/capturer.cpp
    src/win32/image_encoder.cpp
    src/win32/injector.cpp
    src/win32/app_inspector.cpp
    src/win32/signature.cpp
    src/win32/lock_monitor.cpp
    src/win32/waker.cpp
    src/win32/esc_tap.cpp
    src/win32/indicator.cpp
    src/win32/enablement_store.cpp
    src/win32/pipe_channel.cpp
    src/win32/environment.cpp
    ${CMAKE_CURRENT_BINARY_DIR}/resources.rc
  )
  target_include_directories(bothy-computer-helper PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src/win32
    ${CMAKE_CURRENT_BINARY_DIR})
  target_compile_definitions(bothy-computer-helper PRIVATE
    UNICODE _UNICODE WIN32_LEAN_AND_MEAN NOMINMAX _WIN32_WINNT=0x0A00 NTDDI_VERSION=0x0A000008
    HELPER_VERSION="${HELPER_VERSION}")
  target_compile_options(bothy-computer-helper PRIVATE ${HELPER_WARNINGS})
  target_link_libraries(bothy-computer-helper PRIVATE helper_core
    user32 gdi32 d3d11 dxgi windowscodecs ole32 oleaut32 shell32 shlwapi wintrust crypt32
    wtsapi32 version advapi32)
  if(MSVC)
    # The manifest comes from resources.rc; the linker must not add a second one.
    target_link_options(bothy-computer-helper PRIVATE /MANIFEST:NO)
  else()
    target_link_options(bothy-computer-helper PRIVATE -municode -static)
  endif()
endif()

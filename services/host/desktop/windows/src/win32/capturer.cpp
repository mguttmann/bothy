// SPDX-License-Identifier: Apache-2.0

// Screen capture: DXGI Desktop Duplication first (27 9.2), GDI as the fallback. Both run with
// per-monitor DPI awareness v2, so every rectangle is in physical pixels and the screen's scale is 1.
// The indicator windows carry WDA_EXCLUDEFROMCAPTURE, which both paths honour.

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <d3d11.h>
#include <dxgi1_2.h>

#include "bothy/helper/duplication.hpp"


namespace bothy::helper::win32 {

namespace {

template <typename T>
struct Com {
  T* p = nullptr;
  ~Com() {
    if (p != nullptr) p->Release();
  }
  T** put() { return &p; }
  T* operator->() const { return p; }
};

// Desktop Duplication delivers the first frame of a new duplication at once; the wait only bounds a
// driver that never answers, after which the GDI path takes over. That first frame can be
// pointer-only and black (duplication.hpp); acquire_desktop_image() skips it.
constexpr UINT kAcquireWaitMs = 500;

HMONITOR monitor_of(const ScreenInfo& s) {
  RECT r{static_cast<LONG>(s.x), static_cast<LONG>(s.y), static_cast<LONG>(s.x + s.width),
         static_cast<LONG>(s.y + s.height)};
  return MonitorFromRect(&r, MONITOR_DEFAULTTONEAREST);
}

// Copies the monitor's image through Desktop Duplication into `out` (BGRA, top-down).
bool capture_dxgi(HMONITOR monitor, int width, int height, std::vector<std::uint8_t>& out) {
  Com<IDXGIFactory1> factory;
  if (FAILED(CreateDXGIFactory1(IID_PPV_ARGS(factory.put())))) return false;
  for (UINT a = 0;; ++a) {
    Com<IDXGIAdapter1> adapter;
    if (factory->EnumAdapters1(a, adapter.put()) == DXGI_ERROR_NOT_FOUND) return false;
    for (UINT o = 0;; ++o) {
      Com<IDXGIOutput> output;
      if (adapter->EnumOutputs(o, output.put()) == DXGI_ERROR_NOT_FOUND) break;
      DXGI_OUTPUT_DESC desc{};
      if (FAILED(output->GetDesc(&desc)) || desc.Monitor != monitor) continue;
      if (desc.Rotation != DXGI_MODE_ROTATION_IDENTITY && desc.Rotation != DXGI_MODE_ROTATION_UNSPECIFIED)
        return false;  // a rotated output comes unrotated; GDI returns it as seen
      Com<IDXGIOutput1> output1;
      if (FAILED(output->QueryInterface(IID_PPV_ARGS(output1.put())))) return false;
      Com<ID3D11Device> device;
      Com<ID3D11DeviceContext> context;
      if (FAILED(D3D11CreateDevice(adapter.p, D3D_DRIVER_TYPE_UNKNOWN, nullptr, 0, nullptr, 0,
                                   D3D11_SDK_VERSION, device.put(), nullptr, context.put())))
        return false;
      Com<IDXGIOutputDuplication> dup;
      if (FAILED(output1->DuplicateOutput(device.p, dup.put()))) return false;
      Com<IDXGIResource> resource;
      bool image = acquire_desktop_image([&] {
        if (resource.p != nullptr) {
          resource.p->Release();
          resource.p = nullptr;
        }
        DXGI_OUTDUPL_FRAME_INFO info{};
        if (FAILED(dup->AcquireNextFrame(kAcquireWaitMs, &info, resource.put()))) return DuplicationFrame::Failed;
        if (info.LastPresentTime.QuadPart != 0) return DuplicationFrame::Image;
        dup->ReleaseFrame();  // pointer-only: the texture holds no desktop image
        return DuplicationFrame::NoImage;
      });
      if (!image) return false;
      bool ok = false;
      Com<ID3D11Texture2D> texture;
      if (SUCCEEDED(resource->QueryInterface(IID_PPV_ARGS(texture.put())))) {
        D3D11_TEXTURE2D_DESC td{};
        texture->GetDesc(&td);
        if (td.Format == DXGI_FORMAT_B8G8R8A8_UNORM && static_cast<int>(td.Width) == width &&
            static_cast<int>(td.Height) == height) {
          td.Usage = D3D11_USAGE_STAGING;
          td.BindFlags = 0;
          td.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
          td.MiscFlags = 0;
          td.MipLevels = 1;
          td.ArraySize = 1;
          td.SampleDesc.Count = 1;
          td.SampleDesc.Quality = 0;
          Com<ID3D11Texture2D> staging;
          if (SUCCEEDED(device->CreateTexture2D(&td, nullptr, staging.put()))) {
            context->CopyResource(staging.p, texture.p);
            D3D11_MAPPED_SUBRESOURCE map{};
            if (SUCCEEDED(context->Map(staging.p, 0, D3D11_MAP_READ, 0, &map))) {
              out.resize(static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4);
              const auto* src = static_cast<const std::uint8_t*>(map.pData);
              for (int row = 0; row < height; ++row) {
                memcpy(out.data() + static_cast<std::size_t>(row) * static_cast<std::size_t>(width) * 4,
                       src + static_cast<std::size_t>(row) * map.RowPitch,
                       static_cast<std::size_t>(width) * 4);
              }
              context->Unmap(staging.p, 0);
              ok = true;
            }
          }
        }
      }
      dup->ReleaseFrame();
      return ok;
    }
  }
}

// Copies the monitor's image through GDI into `out` (BGRA, top-down).
bool capture_gdi(const RECT& r, std::vector<std::uint8_t>& out) {
  int width = r.right - r.left;
  int height = r.bottom - r.top;
  HDC screen = GetDC(nullptr);
  if (screen == nullptr) return false;
  HDC mem = CreateCompatibleDC(screen);
  BITMAPINFO bi{};
  bi.bmiHeader.biSize = sizeof bi.bmiHeader;
  bi.bmiHeader.biWidth = width;
  bi.bmiHeader.biHeight = -height;  // top-down rows
  bi.bmiHeader.biPlanes = 1;
  bi.bmiHeader.biBitCount = 32;
  bi.bmiHeader.biCompression = BI_RGB;
  void* bits = nullptr;
  HBITMAP dib = mem ? CreateDIBSection(screen, &bi, DIB_RGB_COLORS, &bits, nullptr, 0) : nullptr;
  bool ok = false;
  if (dib != nullptr) {
    HGDIOBJ old = SelectObject(mem, dib);
    // CAPTUREBLT includes layered windows of other apps; the indicator stays out by its affinity.
    if (BitBlt(mem, 0, 0, width, height, screen, r.left, r.top, SRCCOPY | CAPTUREBLT)) {
      GdiFlush();
      const auto* src = static_cast<const std::uint8_t*>(bits);
      out.assign(src, src + static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4);
      ok = true;
    }
    SelectObject(mem, old);
    DeleteObject(dib);
  }
  if (mem != nullptr) DeleteDC(mem);
  ReleaseDC(nullptr, screen);
  return ok;
}

}  // namespace

std::optional<ScreenInfo> Win32Capturer::screen() {
  HWND foreground = GetForegroundWindow();
  HMONITOR monitor = foreground != nullptr ? MonitorFromWindow(foreground, MONITOR_DEFAULTTOPRIMARY)
                                           : MonitorFromPoint(POINT{0, 0}, MONITOR_DEFAULTTOPRIMARY);
  MONITORINFO mi{};
  mi.cbSize = sizeof mi;
  if (monitor == nullptr || !GetMonitorInfoW(monitor, &mi)) return std::nullopt;
  const RECT& r = mi.rcMonitor;
  return ScreenInfo{static_cast<double>(r.left), static_cast<double>(r.top),
                    static_cast<double>(r.right - r.left), static_cast<double>(r.bottom - r.top), 1.0};
}

std::optional<std::vector<std::uint8_t>> Win32Capturer::capture(const CaptureRequest& request) {
  HMONITOR monitor = monitor_of(request.screen);
  MONITORINFO mi{};
  mi.cbSize = sizeof mi;
  if (!GetMonitorInfoW(monitor, &mi)) return std::nullopt;
  const RECT& r = mi.rcMonitor;
  int width = r.right - r.left;
  int height = r.bottom - r.top;
  // The screen changed between screen() and now (a mode change); the host takes a new screenshot.
  if (width != static_cast<int>(request.screen.width) || height != static_cast<int>(request.screen.height))
    return std::nullopt;
  std::vector<std::uint8_t> pixels;
  if (!capture_dxgi(monitor, width, height, pixels) && !capture_gdi(r, pixels)) return std::nullopt;
  return encode_image_wic(pixels, width, height, request);
}

}  // namespace bothy::helper::win32

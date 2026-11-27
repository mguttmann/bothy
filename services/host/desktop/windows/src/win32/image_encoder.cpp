// SPDX-License-Identifier: Apache-2.0

// WIC encoding in memory: crop, scale, drop alpha, encode. The image never touches the disk
// (brief CU-02 "Definition of done").

#include "win32.hpp"  // first: windows.h must precede the other Windows headers

#include <objidl.h>
#include <shlwapi.h>
#include <wincodec.h>


namespace bothy::helper::win32 {

namespace {

// Releases a COM pointer at scope end; the helper has no ATL or WRL dependency.
template <typename T>
struct Com {
  T* p = nullptr;
  ~Com() {
    if (p != nullptr) p->Release();
  }
  T** put() { return &p; }
  T* operator->() const { return p; }
};

}  // namespace

std::optional<std::vector<std::uint8_t>> encode_image_wic(const std::vector<std::uint8_t>& pixels,
                                                          int width, int height,
                                                          const CaptureRequest& request) {
  if (width <= 0 || height <= 0 || request.out_width <= 0 || request.out_height <= 0) return std::nullopt;
  Com<IWICImagingFactory> factory;
  if (FAILED(CoCreateInstance(CLSID_WICImagingFactory, nullptr, CLSCTX_INPROC_SERVER,
                              IID_PPV_ARGS(factory.put()))))
    return std::nullopt;

  Com<IWICBitmap> bitmap;
  if (FAILED(factory->CreateBitmapFromMemory(static_cast<UINT>(width), static_cast<UINT>(height),
                                             GUID_WICPixelFormat32bppBGRA,
                                             static_cast<UINT>(width) * 4,
                                             static_cast<UINT>(pixels.size()),
                                             const_cast<BYTE*>(pixels.data()), bitmap.put())))
    return std::nullopt;

  WICRect rect{static_cast<INT>(request.source.x), static_cast<INT>(request.source.y),
               static_cast<INT>(request.source.width), static_cast<INT>(request.source.height)};
  Com<IWICBitmapClipper> clipper;
  if (FAILED(factory->CreateBitmapClipper(clipper.put())) ||
      FAILED(clipper->Initialize(bitmap.p, &rect)))
    return std::nullopt;

  IWICBitmapSource* source = clipper.p;
  Com<IWICBitmapScaler> scaler;
  if (request.out_width != rect.Width || request.out_height != rect.Height) {
    // Fant keeps small text legible when an image shrinks, which matters for a model reading it.
    if (FAILED(factory->CreateBitmapScaler(scaler.put())) ||
        FAILED(scaler->Initialize(clipper.p, static_cast<UINT>(request.out_width),
                                  static_cast<UINT>(request.out_height),
                                  WICBitmapInterpolationModeFant)))
      return std::nullopt;
    source = scaler.p;
  }

  // The desktop's alpha channel is meaningless (GDI leaves it 0), so both formats get 24-bit BGR.
  Com<IWICFormatConverter> converter;
  if (FAILED(factory->CreateFormatConverter(converter.put())) ||
      FAILED(converter->Initialize(source, GUID_WICPixelFormat24bppBGR, WICBitmapDitherTypeNone,
                                   nullptr, 0.0, WICBitmapPaletteTypeCustom)))
    return std::nullopt;

  Com<IStream> stream;
  stream.p = SHCreateMemStream(nullptr, 0);
  if (stream.p == nullptr) return std::nullopt;

  Com<IWICBitmapEncoder> encoder;
  const GUID& container =
      request.format == ImageFormat::Png ? GUID_ContainerFormatPng : GUID_ContainerFormatJpeg;
  if (FAILED(factory->CreateEncoder(container, nullptr, encoder.put())) ||
      FAILED(encoder->Initialize(stream.p, WICBitmapEncoderNoCache)))
    return std::nullopt;

  Com<IWICBitmapFrameEncode> frame;
  Com<IPropertyBag2> props;
  if (FAILED(encoder->CreateNewFrame(frame.put(), props.put()))) return std::nullopt;
  if (request.format == ImageFormat::Jpeg && props.p != nullptr) {
    PROPBAG2 option{};
    wchar_t name[] = L"ImageQuality";
    option.pstrName = name;
    VARIANT value;
    VariantInit(&value);
    value.vt = VT_R4;
    value.fltVal = static_cast<float>(request.quality) / 100.0f;
    props->Write(1, &option, &value);
  }
  WICPixelFormatGUID format = GUID_WICPixelFormat24bppBGR;
  if (FAILED(frame->Initialize(props.p)) ||
      FAILED(frame->SetSize(static_cast<UINT>(request.out_width), static_cast<UINT>(request.out_height))) ||
      FAILED(frame->SetPixelFormat(&format)) || FAILED(frame->WriteSource(converter.p, nullptr)) ||
      FAILED(frame->Commit()) || FAILED(encoder->Commit()))
    return std::nullopt;

  STATSTG stat{};
  if (FAILED(stream->Stat(&stat, STATFLAG_NONAME))) return std::nullopt;
  std::vector<std::uint8_t> out(static_cast<std::size_t>(stat.cbSize.QuadPart));
  LARGE_INTEGER zero{};
  ULONG read = 0;
  if (FAILED(stream->Seek(zero, STREAM_SEEK_SET, nullptr)) ||
      FAILED(stream->Read(out.data(), static_cast<ULONG>(out.size()), &read)) || read != out.size())
    return std::nullopt;
  return out;
}

}  // namespace bothy::helper::win32

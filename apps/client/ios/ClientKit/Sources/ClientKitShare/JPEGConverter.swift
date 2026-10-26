// SPDX-License-Identifier: Apache-2.0

import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

/// Turns an image file into a JPEG file; `JPEGConverter` in the extension, a failing one in the
/// tests of the loader's fallback.
public protocol ImageConverting {
    /// Writes `source` as a JPEG to `destination`.
    ///
    /// - Parameters:
    ///   - source: the image file.
    ///   - destination: the JPEG to write; absent after a failure.
    /// - Throws: `JPEGConverterError`.
    func convert(from source: URL, to destination: URL) throws
}

/// Why a conversion failed.
public enum JPEGConverterError: Error, Equatable {
    /// The source holds no image ImageIO can read.
    case unreadable
    /// The JPEG could not be written.
    case encodingFailed
}

/// The JPEG conversion of photos for the upload (15 14; 12 7.1; ADR 0047; brief M5-06 Design
/// note 2).
///
/// Only HEIC and HEIF photos (the library's format) are converted: the server does not clean
/// them losslessly, so they would reach it only as a file. JPEG, PNG, GIF and WebP stay
/// unchanged, because the server removes their metadata itself, and every other file is sent as
/// it is (docs/decisions.d/M5-06.md).
///
/// The extension's memory is small, so the photo is never decoded as a whole (the decoded size of
/// a phone photo is several times its file size): the encoder pulls the pixels through a
/// sequential data provider, which decodes one band of rows at a time from the source with
/// ImageIO (`CGImage.cropping(to:)` of a source image that is not cached, so a tiled HEIC
/// decodes only the tiles of the band). The pixels stay as stored and the EXIF orientation is
/// written into the JPEG, which the server keeps; the color space goes along when it is an RGB
/// space, else the pixels are drawn into sRGB. No other metadata is copied: the location never
/// leaves the device. `UIImage` is not used, because it decodes the whole image.
///
/// Example:
///
///     if JPEGConverter.needsConversion(type) {
///         try JPEGConverter().convert(from: heic, to: jpeg)
///     }
public struct JPEGConverter: ImageConverting {
    /// The rows decoded at once; a memory detail, not a limit.
    public static let defaultBandRows = 256
    /// The JPEG quality: high, because the server keeps the original as the download in full
    /// quality (ADR 0047); an encoding constant, not a limit.
    public static let quality = 0.9

    private let bandRows: Int

    /// Creates the converter.
    ///
    /// - Parameter bandRows: the rows decoded at once.
    public init(bandRows: Int = defaultBandRows) {
        self.bandRows = max(1, bandRows)
    }

    /// Whether a shared item of `type` is converted: HEIC and HEIF stills, not image sequences.
    ///
    /// - Parameter type: the item's type.
    /// - Returns: true for a photo the server cannot clean losslessly.
    public static func needsConversion(_ type: UTType) -> Bool {
        type.conforms(to: .heic) || type.conforms(to: .heif)
    }

    /// See `ImageConverting.convert(from:to:)`.
    public func convert(from source: URL, to destination: URL) throws {
        let uncached = [kCGImageSourceShouldCache: false, kCGImageSourceShouldCacheImmediately: false] as CFDictionary
        guard let imageSource = CGImageSourceCreateWithURL(source as CFURL, uncached),
              CGImageSourceGetCount(imageSource) > 0,
              case let primary = CGImageSourceGetPrimaryImageIndex(imageSource),
              let image = CGImageSourceCreateImageAtIndex(imageSource, primary, uncached)
        else {
            throw JPEGConverterError.unreadable
        }
        // The primary image: a HEIC can hold more images (depth, gain map) besides the photo.
        let properties = CGImageSourceCopyPropertiesAtIndex(imageSource, primary, nil) as? [CFString: Any]
        let orientation = (properties?[kCGImagePropertyOrientation] as? NSNumber)?.intValue ?? 1
        let space = Self.outputSpace(of: image)
        let bands = BandRenderer(image: image, space: space, bandRows: bandRows)
        guard let pixels = bands.pixelImage(),
              let output = CGImageDestinationCreateWithURL(destination as CFURL, UTType.jpeg.identifier as CFString, 1, nil)
        else {
            throw JPEGConverterError.encodingFailed
        }
        let options: [CFString: Any] = [
            kCGImageDestinationLossyCompressionQuality: Self.quality,
            kCGImagePropertyOrientation: (1...8).contains(orientation) ? orientation : 1,
        ]
        CGImageDestinationAddImage(output, pixels, options as CFDictionary)
        guard CGImageDestinationFinalize(output), !bands.failed else {
            try? FileManager.default.removeItem(at: destination)
            throw JPEGConverterError.encodingFailed
        }
    }

    /// The image's own color space when it is RGB and can be drawn into; sRGB otherwise (grey,
    /// indexed, CMYK), since a JPEG of the upload is RGB.
    private static func outputSpace(of image: CGImage) -> CGColorSpace {
        if let space = image.colorSpace, space.model == .rgb, space.supportsOutput {
            return space
        }
        return CGColorSpace(name: CGColorSpace.sRGB) ?? CGColorSpaceCreateDeviceRGB()
    }
}

/// Decodes the bands of `image` into one reused buffer and hands their bytes to the encoder in
/// order, through a sequential `CGDataProvider`.
private final class BandRenderer {
    private let image: CGImage
    private let space: CGColorSpace
    private let bandRows: Int
    private let width: Int
    private let height: Int
    private let bytesPerRow: Int
    private let buffer: UnsafeMutableRawPointer
    private var bandStart = -1
    private var position = 0
    private(set) var failed = false

    init(image: CGImage, space: CGColorSpace, bandRows: Int) {
        self.image = image
        self.space = space
        self.bandRows = min(bandRows, max(1, image.height))
        width = image.width
        height = image.height
        bytesPerRow = image.width * 4
        buffer = UnsafeMutableRawPointer.allocate(byteCount: max(1, bytesPerRow * self.bandRows), alignment: 64)
    }

    deinit {
        buffer.deallocate()
    }

    /// The image whose pixels this renderer delivers on demand; it keeps the renderer alive.
    func pixelImage() -> CGImage? {
        var callbacks = CGDataProviderSequentialCallbacks(
            version: 0,
            getBytes: { info, out, count in
                Unmanaged<BandRenderer>.fromOpaque(info!).takeUnretainedValue().copy(into: out, count: count)
            },
            skipForward: { info, count in
                let renderer = Unmanaged<BandRenderer>.fromOpaque(info!).takeUnretainedValue()
                renderer.position += Int(count)
                return count
            },
            rewind: { info in
                let renderer = Unmanaged<BandRenderer>.fromOpaque(info!).takeUnretainedValue()
                renderer.position = 0
                renderer.bandStart = -1
            },
            releaseInfo: { info in Unmanaged<BandRenderer>.fromOpaque(info!).release() })
        let info = Unmanaged.passRetained(self).toOpaque()
        guard let provider = CGDataProvider(sequentialInfo: info, callbacks: &callbacks) else {
            Unmanaged<BandRenderer>.fromOpaque(info).release()
            return nil
        }
        return CGImage(
            width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: bytesPerRow, space: space,
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.noneSkipLast.rawValue), provider: provider, decode: nil,
            shouldInterpolate: false, intent: .defaultIntent)
    }

    /// Copies the next `count` bytes, decoding bands as the position reaches them; fewer bytes
    /// after the end or after a failure, which fails the encoder.
    private func copy(into out: UnsafeMutableRawPointer, count: Int) -> Int {
        let total = bytesPerRow * height
        var done = 0
        while done < count, position < total, !failed {
            let start = (position / bytesPerRow) / bandRows * bandRows
            if start != bandStart {
                guard render(start) else {
                    failed = true
                    break
                }
                bandStart = start
            }
            let offset = position - start * bytesPerRow
            let bandBytes = min(bandRows, height - start) * bytesPerRow
            let chunk = min(count - done, bandBytes - offset)
            memcpy(out + done, buffer + offset, chunk)
            done += chunk
            position += chunk
        }
        return done
    }

    /// Decodes rows `start ..< start + bandRows` of the source into the buffer, top row first.
    private func render(_ start: Int) -> Bool {
        let rows = min(bandRows, height - start)
        guard let context = CGContext(
            data: buffer, width: width, height: rows, bitsPerComponent: 8, bytesPerRow: bytesPerRow, space: space,
            bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue),
            let band = image.cropping(to: CGRect(x: 0, y: start, width: width, height: rows))
        else {
            return false
        }
        // A JPEG has no transparency: transparent pixels become white, not leftovers of the
        // previous band.
        let area = CGRect(x: 0, y: 0, width: width, height: rows)
        context.setFillColor(CGColor(gray: 1, alpha: 1))
        context.fill(area)
        context.interpolationQuality = .none
        context.draw(band, in: area)
        return true
    }
}

/// The small preview of the first shared image (brief M5-06 Design note 1).
public enum ShareThumbnail {
    /// A thumbnail of the image file `url`, upright, its longer side at most `maxPixelSize`;
    /// ImageIO decodes it subsampled, never the whole image.
    ///
    /// - Parameters:
    ///   - url: the staged file.
    ///   - maxPixelSize: the longer side in pixels; a display size, not a limit.
    /// - Returns: the thumbnail, or nil when the file is no image ImageIO reads.
    public static func make(_ url: URL, maxPixelSize: Int) -> CGImage? {
        guard let source = CGImageSourceCreateWithURL(url as CFURL, [kCGImageSourceShouldCache: false] as CFDictionary) else {
            return nil
        }
        let options: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxPixelSize,
            kCGImageSourceShouldCacheImmediately: true,
        ]
        return CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary)
    }
}

// SPDX-License-Identifier: Apache-2.0

package com.example.client

/**
 * The MIME types other apps can share into the app (docs/spec/15-client-architecture.md 12;
 * brief M5-06 Design note 6 and fixed requirement 17).
 *
 * The server takes every type except the operator's `file.denied_mime_types` and detects the
 * type from the bytes (docs/spec/12-files.md 6.1), so the list is not a security boundary: it
 * names the concrete types a person shares in practice, because Android asks receivers for
 * concrete types instead of a wildcard. `application/octet-stream` is left out on purpose,
 * since accepting it would offer the app for every file like a wildcard.
 *
 * - Text and links: `text/plain`.
 * - Images: `image/jpeg`, `image/png`, `image/heic`, `image/heif`, `image/webp`, `image/gif`,
 *   `image/avif`, `image/bmp`, `image/tiff`, `image/svg+xml`.
 * - Documents: PDF, RTF, Word, Excel, PowerPoint (legacy and Office Open XML), OpenDocument
 *   text, spreadsheet and presentation, EPUB, CSV (both names Android uses), Markdown,
 *   calendar and vCard files (both names), JSON and XML.
 * - Archives: ZIP, GZIP, TAR, 7-Zip, RAR (both names).
 * - Audio: MP3, MP4 and M4A audio, AAC, Ogg, Opus, FLAC, WAV (both names), WebM, AMR, 3GPP.
 * - Video: MP4, QuickTime, WebM, 3GPP, Matroska, MPEG.
 *
 * The manifest's share filters list exactly these types; `ShareManifestTest` holds both
 * together. The value of each entry is the file extension a nameless item gets.
 */
object ShareTypes {
    /** Each supported type with the extension of a derived name (`file.<extension>`). */
    val EXTENSIONS: Map<String, String> =
        linkedMapOf(
            "text/plain" to "txt",
            "image/jpeg" to "jpg",
            "image/png" to "png",
            "image/heic" to "heic",
            "image/heif" to "heif",
            "image/webp" to "webp",
            "image/gif" to "gif",
            "image/avif" to "avif",
            "image/bmp" to "bmp",
            "image/tiff" to "tiff",
            "image/svg+xml" to "svg",
            "application/pdf" to "pdf",
            "application/rtf" to "rtf",
            "application/msword" to "doc",
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document" to "docx",
            "application/vnd.ms-excel" to "xls",
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" to "xlsx",
            "application/vnd.ms-powerpoint" to "ppt",
            "application/vnd.openxmlformats-officedocument.presentationml.presentation" to "pptx",
            "application/vnd.oasis.opendocument.text" to "odt",
            "application/vnd.oasis.opendocument.spreadsheet" to "ods",
            "application/vnd.oasis.opendocument.presentation" to "odp",
            "application/epub+zip" to "epub",
            "text/csv" to "csv",
            "text/comma-separated-values" to "csv",
            "text/markdown" to "md",
            "text/calendar" to "ics",
            "text/vcard" to "vcf",
            "text/x-vcard" to "vcf",
            "application/json" to "json",
            "application/xml" to "xml",
            "text/xml" to "xml",
            "application/zip" to "zip",
            "application/gzip" to "gz",
            "application/x-tar" to "tar",
            "application/x-7z-compressed" to "7z",
            "application/vnd.rar" to "rar",
            "application/x-rar-compressed" to "rar",
            "audio/mpeg" to "mp3",
            "audio/mp4" to "m4a",
            "audio/x-m4a" to "m4a",
            "audio/aac" to "aac",
            "audio/ogg" to "ogg",
            "audio/opus" to "opus",
            "audio/flac" to "flac",
            "audio/wav" to "wav",
            "audio/x-wav" to "wav",
            "audio/webm" to "weba",
            "audio/amr" to "amr",
            "audio/3gpp" to "3gp",
            "video/mp4" to "mp4",
            "video/quicktime" to "mov",
            "video/webm" to "webm",
            "video/3gpp" to "3gp",
            "video/x-matroska" to "mkv",
            "video/mpeg" to "mpeg",
        )

    /** The supported types in the order of the manifest's filters. */
    val MIME_TYPES: List<String> = EXTENSIONS.keys.toList()
}

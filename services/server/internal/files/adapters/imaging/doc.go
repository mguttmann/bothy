// SPDX-License-Identifier: Apache-2.0

// Package imaging is the pure Go image codec of the files module (ADR 0047, docs/spec/12-files.md
// 7): it removes the metadata of JPEG and PNG originals with lossless segment and chunk filters,
// re-encodes GIF, converts still WebP and renders the thumbnail variants, all without cgo. It
// implements app.ImageCodec in codec.go and is the only package of the server that imports
// golang.org/x/image, so a decoder or resampler cannot slip into another package unnoticed.
package imaging

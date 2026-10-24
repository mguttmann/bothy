// SPDX-License-Identifier: Apache-2.0

// Package transcription holds the adapter openai_compatible of the outgoing port app.Transcriber
// of the files module (docs/spec/12-files.md 8.2, M3-09 design note 11): it sends the audio of a
// voice message to the one configured OpenAI-compatible transcription service
// (voice.transcription_url, for example a self-hosted Whisper service) and returns its transcript.
//
// Request form (design note 11 b):
//
//   - POST to voice.transcription_url through the egress client of internal/wire, purpose
//     transcription, whose host policy (HostPolicy) allows exactly the scheme, host and port of
//     that URL, with no redirects and the time limit voice.transcription_timeout. A self-hosted
//     service on a private address also needs an operator entry host[:port];allow_private in
//     egress.allowed_hosts. The URL is instance configuration only; no file name, request field
//     or user value reaches the request line.
//   - Body multipart/form-data, streamed from the object through an io.Pipe without a spool
//     file: model only when voice.transcription_model is set, language only when the uploader's
//     locale gave a hint, response_format=json, and the part file with the file name audio.m4a
//     (audio.mp3 for audio/mpeg), never the original name, and the stored media type.
//   - Authorization: Bearer <key> only when voice.transcription_api_key_ref is set; the key is
//     resolved through the secret provider on every call (cached by secrets.cache_ttl, so a
//     rotation needs no restart) and held as secrets.Value until the header is written.
//
// Response: status 200 with a JSON object whose member text (string) is required and language
// (string) optional; other members are ignored. The text must be valid UTF-8 without U+0000 and
// at most message.max_content_bytes bytes.
//
// Error classes (design note 11 d): transient are timeouts, connection errors, 408, 429 and 5xx,
// and a key the secret provider cannot read at the moment; the job retries them up to
// voice.transcription_max_attempts. Permanent, wrapping app.ErrTranscriptionPermanent, are an
// egress block (egress.ErrBlocked), a body over egress.max_response_bytes
// (egress.ErrResponseTooLarge), every 3xx (never followed), every other status, and a malformed
// or invalid body. Every failed call writes one Warn line "transcription failed" with
// bothy.file.id, bothy.reason (transient or permanent) and http.response.status_code when a
// response arrived; never the URL path or query, the key, a response body or a transcript. Error
// texts follow the same rule.
package transcription

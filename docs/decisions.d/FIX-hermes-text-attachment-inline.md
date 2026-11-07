# FIX-hermes-text-attachment-inline A text attachment reaches the Hermes turn as text

Found live on 2026-11-07 (adapter on Hermes main `a8b0bc15053c`): a long paste the desktop app
turns into the attachment `paste.txt` (12,430 bytes, `text/plain`, 01 6.13) reached the turn
only as Hermes' note `[The user sent a text document: 'paste.txt'. It is saved at: <path>. Its
content is not inlined here. ...]`. The agent had to call `read_file`, and a profile whose read
confinement hook does not allow the profile's `cache/documents` could not read it. Cause, read in
the pinned source and reproduced with the pinned Hermes (`tests/fake_hermes/test_media.py`): the
adapter passed every file with `media_text_inlined` false (D-FIX-HIMG-1), so Hermes' gateway
(`_prepend_inbound_document_notes` in `gateway/run_inbound.py`, `_build_document_context_note` in
`gateway/run.py`, the same in `v0.22.5`) wrote the note without content. Hermes' gateway itself
never inlines a file: its platforms do (Discord `_collect_attachment_media`, Slack, WhatsApp,
Telegram, Feishu prepend `[Content of <name>]:` and the text to the event's text and set the
attachment's `media_text_inlined` true; then the gateway's note says the content is included).
The text did stand in the user side (the context's data block of the triggering message's file),
but that rides Hermes' `pre_llm_call` hook context, which the profile's
`hooks.output_spill.max_chars` (default 10,000) bounds, so the 12,430-byte text was left out
(D-FHUSC-3). `media_filenames` (new in `a8b0bc15053c`, absent in `v0.22.5`) only sets the display
name of the note; the adapter does not set it, the cached file name already carries the name.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FHTAI-1 | 2026-11-07 | The adapter uses Hermes' own inlining contract: for a text attachment it puts the text into the turn's text and passes the file with `media_text_inlined` true; the file stays in the media cache and in `media_urls`, so Hermes' note names it and its path and says the content is included below. A text file of a type outside `text/*` (`application/json`) is passed as `text/plain` in `media_types`, because Hermes' note calls only a `text/*` file a text document and every other file a binary to extract. This replaces the `media_text_inlined` false of D-FIX-HIMG-1 for text attachments. Place: `handler.py` (`_media`, `_with_media`), `media.py` (`is_text_attachment`, `inline_text`, `too_long_note`), `platform.py` (`_attach`), `runtime.py` (`InboundMedia.text_inlined`). | The same fields as on Hermes' other platforms give the same note and leave the turn independent of the agent's file tools; there is no inlining in Hermes' gateway the adapter could switch on. |
| D-FHTAI-2 | 2026-11-07 | A text attachment is one with a `text/*` type or a known text type (`application/json`, `application/x-ndjson`, `application/yaml`, `application/xml`, `application/toml`, `application/csv`, `application/sql`, `application/x-sh`, `application/x-diff`, `application/x-patch`, `application/javascript` and the like); for a type that tells nothing (none or `application/octet-stream`) the name's suffix decides (`.txt`, `.md`, `.json`, `.yaml`, `.csv`, `.log`, `.diff`, `.patch` and the other common suffixes of Hermes' own `_TEXT_INJECT_EXTENSIONS`). Images, audio, video and every other type are passed unchanged. | Hermes gates on type and suffix, never on a blind UTF-8 decode (a PDF or a ZIP starts with decodable bytes); a suffix alone would take `clip.ts` with `video/mp2t` for text. |
| D-FHTAI-3 | 2026-11-07 | The text is decoded as UTF-8 with replacement and stands as `[Content of <name>]:`, the SDK's data notice and a data block `<<<DATA <boundary> source=file id=<file>>>>` ... `<<<END DATA <boundary>>>>` with a random boundary the text does not contain; the blocks stand after the notes of undelivered files and before the person's text, in the attachments' order. An inlined text leaves the user side (the context lists the file by name, type and size), so the model gets it once. | User-supplied data in the same form as every file text of the context (09 4.6, T-AI-03: a document can carry instructions that are not the person's); the Discord heading matches Hermes' note ("included below"); a text the profile's hook limit would cut is not sent twice. |
| D-FHTAI-4 | 2026-11-07 | The bound is `hermes_adapter.max_message_bytes` (default `262144`, equal to `message.max_content_bytes`), counted on the whole file. A longer text attachment keeps Hermes' note with the cached path and gets the note `[The text of the file '<name>' (<size> bytes) is larger than the inline limit of <limit> bytes, so its content is not included in this message.]` before the person's text. | A paste becomes a file from `ui.composer_paste_file_chars` (4000) characters or `ui.composer_paste_file_lines` (100) lines on (01 6.13, R-PR-26); the same text sent as message text up to `message.max_content_bytes` would reach the agent whole, so the file should too. `hermes_adapter.attachment_text_max_bytes` (32768) stays the bound of file texts in the context, where many messages share one hook limit. |

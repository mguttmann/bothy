# F-AGENT-FORMAT Format rules for agent replies and their rendering

Owner request of 2026-11-04: code, tables and texts of agent replies should always look as they
should. Trigger: a text to forward, written into a code block with hard line breaks after about
60 characters, copied as a bundle of short lines. The copy itself is a separate ticket; this one
fixes the source and checks the rendering.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FAF-1 | 2026-11-04 | Found in the code: the Hermes adapter gives its runtime instructions in the system part of `context_format.py` (fixed note, channel instructions, then the context), handed to Hermes as the ephemeral channel prompt, also for a command run; the Claude-Code-Runner in the system prompt file of `prompt.py` (`--append-system-prompt-file`), while the run context goes in the first user message. Neither had format rules beyond "Reply in Markdown". | The rules belong in these instruction channels, not in the context. |
| D-FAF-2 | 2026-11-04 | The rules are `MESSAGE_FORMAT_RULES` and `MESSAGE_FORMAT_TITLE` of `bothy_sdk.message_format`; both adapters import them and place them after their fixed instructions and before the channel prompt (09 4.11, ADR 0074). | One source that both adapters already depend on; the server's run context has no command runs and is data, not instruction. |
| D-FAF-3 | 2026-11-04 | No change to a Hermes profile or any runtime configuration outside the repository. | Owner constraint; the adapter's system part reaches Hermes on every turn anyway. |
| D-FAF-4 | 2026-11-04 | Rendering checked on all clients through the shared component `DsMessage`/`DsMarkdown` (desktop, phone and the web console use it): widget tests per target platform and a golden. The only real defect found: a table wider than the view scrolled sideways without a visible scrollbar on the desktop, so a mouse user could not see or drag it (the same defect 01 6.14 fixed for code blocks). The table now brings its own scrollbar, always visible on the desktop, with room below the table. | Fix only what the test shows; no other change of the rendering. |
| D-FAF-5 | 2026-11-04 | The TypeScript SDK gets no copy of the rules now. | It has no agent adapter; the first one takes the text over with a parity test (ADR 0074). |

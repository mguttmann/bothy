// SPDX-License-Identifier: Apache-2.0

// An agent reply written by the format rules of docs/spec/09-agents.md 4.11 (F-AGENT-FORMAT):
// running text, a narrow and a wide GFM table, a code block with its language, a `text` block
// to copy and a list. Test data only (Alex, Sam, Robin, example.com).

import 'package:design_system/design_system.dart';

/// The start of the running text paragraph (one source line).
const agentReplyParagraphStart = 'The release candidate is ready';

/// A cell of the narrow table.
const agentReplyNarrowCell = 'Review';

/// The longest cell of the wide table.
const agentReplyWideCell = 'the migration of the event log runs before start';

/// The reply as Markdown.
const agentReply =
    '$agentReplyParagraphStart. This paragraph is one line in the source, as the '
    'format rules ask, and the app wraps it to the width of the view instead of the agent '
    'breaking it by hand after about sixty characters.\n'
    '\n'
    '| Step | Owner | State |\n'
    '|---|---|---|\n'
    '| Build | Alex | done |\n'
    '| $agentReplyNarrowCell | Sam | open |\n'
    '\n'
    '| Component | Platform | Version | Build | Tests | Owner | Changed | Note |\n'
    '|---|---|---|---|---|---|---|---|\n'
    '| server | linux | 1.4.2 | green | 1520 passed | Alex | 2026-11-02 | '
    '$agentReplyWideCell |\n'
    '| client | macOS | 1.4.2 | green | 980 passed | Robin | 2026-11-03 | none |\n'
    '\n'
    '```python\n'
    'def greet(name: str) -> str:\n'
    '    return f"Hello, {name}"\n'
    '```\n'
    '\n'
    '```text\n'
    'Hello Sam, the release is ready. Please read the notes on example.com and tell me '
    'by Monday whether anything is missing.\n'
    '\n'
    'Thanks, Alex\n'
    '```\n'
    '\n'
    '- Build the server\n'
    '- Run the tests\n';

/// [agentReply] as the message of the agent Robin.
DsMessageData agentReplyMessage() => const DsMessageData(
  id: 'agent-reply',
  authorName: 'Robin',
  origin: DsMessageOrigin.agent,
  kind: DsMessageKind.text,
  timeLabel: '10:42',
  content: agentReply,
);

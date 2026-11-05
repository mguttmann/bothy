// SPDX-License-Identifier: Apache-2.0

// The golden of an agent reply in the format rules of docs/spec/09-agents.md 4.11
// (F-AGENT-FORMAT): running text wrapped to the view, a narrow GFM table, a wide one that keeps
// its columns and scrolls sideways (the visible scrollbar on the desktop), a code block with its
// language, a `text` block and a list. Light and dark, on the desktop (comfortable) and on the
// phone (touch); the behaviour per platform is test/components/markdown_agent_reply_test.dart.

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';

import 'agent_reply.dart';
import 'sheets.dart';

void main() {
  goldenTest(
    'an agent reply with text, tables, code, a text block and a list',
    fileName: 'agent_reply',
    pumpBeforeTest: dsPumpIcons,
    builder: () => dsGoldenMatrix(
      name: 'agent reply',
      builder: () => Sheet([
        DsMessage(agentReplyMessage(), collapseLines: 400, onOpenLink: (_) {}),
      ]),
      densities: const [DsDensityMode.comfortable, DsDensityMode.touch],
    ),
  );
}

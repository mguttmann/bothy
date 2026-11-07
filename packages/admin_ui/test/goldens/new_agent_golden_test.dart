// SPDX-License-Identifier: Apache-2.0

// Goldens of "New agent" (brief A1-08 Tests; docs/spec/01-product.md 6.20): the three ways in,
// the preview of a template with a local MCP server, the steps after a refused definition, the
// interview with a fallback, and the phone layout; light and dark.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../agents/new_agent/fake_new_agent_port.dart';

const NewAgentOffer _offer = NewAgentOffer(
  templates: [writingTemplate, filesTemplate],
  runners: [devRunner],
);

Widget _frame(Widget child, {double width = 760}) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: width,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

/// The flow of a file helper on Dev runner.
NewAgentFlow _filesFlow(FakeNewAgentPort port) => NewAgentFlow(
  port: port,
  offer: _offer,
  newKey: () => 'key',
  context: port.contextValue,
)..useTemplate(filesTemplate);

/// The steps after the definition was refused for the class Chat (26 4.3 item 7).
class _Refused extends StatefulWidget {
  const _Refused();

  @override
  State<_Refused> createState() => _RefusedState();
}

class _RefusedState extends State<_Refused> {
  final FakeNewAgentPort _port = FakeNewAgentPort(offerValue: _offer);
  late final NewAgentFlow _flow = _filesFlow(_port)
    ..edit((d) => d.toolClass = AgentToolClass.chat);

  @override
  void initState() {
    super.initState();
    unawaited(_flow.run());
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: _flow,
    builder: (context, _) => NewAgentSteps(
      flow: _flow,
      announceMinInterval: const Duration(seconds: 10),
    ),
  );
}

/// The interview after the fallback `builder_offline`.
class _Fallback extends StatefulWidget {
  const _Fallback();

  @override
  State<_Fallback> createState() => _FallbackState();
}

class _FallbackState extends State<_Fallback> {
  late final AgentInterview _interview = AgentInterview(
    FakeNewAgentPort()
      ..interviewReplies.add(
        const InterviewReply(
          outcome: InterviewOutcome.fallback,
          fallbackReason: 'builder_offline',
        ),
      ),
  );

  @override
  void initState() {
    super.initState();
    unawaited(_interview.start());
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: _interview,
    builder: (context, _) => NewAgentInterviewView(
      interview: _interview,
      onTemplates: () {},
      onEmpty: () {},
    ),
  );
}

/// The scenes of the goldens, shared with the accessibility tests (test/a11y).
final Map<String, Widget Function()> views = {
  'new_agent_start': () => _frame(
    NewAgentDialog(
      port: FakeNewAgentPort(offerValue: _offer),
      offer: _offer,
      newKey: () => 'key',
      announceMinInterval: const Duration(seconds: 10),
    ),
  ),
  'new_agent_preview': () =>
      _frame(NewAgentPreview(flow: _filesFlow(FakeNewAgentPort()))),
  'new_agent_steps': () => _frame(const _Refused()),
  'new_agent_interview': () => _frame(const _Fallback()),
  'new_agent_phone': () => SizedBox(
    width: 390,
    height: 700,
    child: NewAgentDialog(
      port: FakeNewAgentPort(offerValue: _offer),
      offer: _offer,
      newKey: () => 'key',
      announceMinInterval: const Duration(seconds: 10),
      fullScreen: true,
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark',
      fileName: name,
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(name: name, builder: view),
    );
  }
}

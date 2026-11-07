// SPDX-License-Identifier: Apache-2.0

/// The built-in agents on the phone (docs/spec/01-product.md 6.20 item 6; brief A1-09
/// requirement 16; docs/decisions.d/A1-09.md): the phone has no agent list of the
/// administration, so this page lists every built-in agent with its [BuiltinAgentSection].
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../new_agent/new_agent_dialog.dart';
import '../new_agent/new_agent_port.dart';
import 'builtin_agent_section.dart';

/// Every built-in agent of [host]'s port (`listAgents`, `adapter_kind = 'claude_agent'`), each
/// with its name, handle and [BuiltinAgentSection]; [timeZone] and [clock] format the times of
/// the versions.
///
/// Example: `BuiltinAgentsView(host: newAgentHost)`.
class BuiltinAgentsView extends StatefulWidget {
  /// Creates the view.
  const BuiltinAgentsView({
    super.key,
    required this.host,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The port and the layout of the dialogs.
  final NewAgentHost host;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<BuiltinAgentsView> createState() => _BuiltinAgentsViewState();
}

class _BuiltinAgentsViewState extends State<BuiltinAgentsView> {
  List<NewAgentState>? _agents;
  NewAgentFailure? _failure;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    try {
      final agents = await widget.host.port.builtinAgents();
      if (mounted) {
        setState(() {
          _agents = agents;
          _failure = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(
          () => _failure = e is NewAgentFailure
              ? e
              : const NewAgentFailure.offline(),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final agents = _agents;
    final failure = _failure;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    if (failure != null) {
      return DsBanner(
        key: const ValueKey('builtinAgents.error'),
        tone: DsTone.danger,
        text: failure.kind == NewAgentFailureKind.offline
            ? l.newAgentErrOffline
            : problemText(l, failure.code ?? '', reason: failure.reason),
      );
    }
    if (agents == null) return const Center(child: CircularProgressIndicator());
    return Column(
      key: const ValueKey('builtinAgents'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.builtinAgentsEntry,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        if (agents.isEmpty) Text(l.builtinAgentsEmpty, style: secondary),
        for (final a in agents)
          Container(
            key: ValueKey('builtinAgents.${a.id}'),
            margin: const EdgeInsets.only(bottom: DsSpace.s12),
            padding: const EdgeInsets.all(DsSpace.s12),
            decoration: BoxDecoration(
              border: Border.all(color: colors.borderSubtle),
              borderRadius: BorderRadius.circular(DsSpace.s8),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                Semantics(
                  header: true,
                  child: Text(
                    a.name,
                    style: dsTextStyle(DsType.bodyStrong)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
                Text(a.handle, style: secondary),
                BuiltinAgentSection(
                  host: widget.host,
                  agentId: a.id,
                  showTitle: false,
                  timeZone: widget.timeZone,
                  clock: widget.clock,
                  onChanged: () => unawaited(_load()),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

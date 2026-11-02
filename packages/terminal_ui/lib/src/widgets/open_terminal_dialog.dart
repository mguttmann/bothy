// SPDX-License-Identifier: Apache-2.0

/// The dialog "Open terminal" (screen D11 of docs/spec/01-product.md; docs/design/components.md
/// 12 "Confirmation on opening"): host, profile, working folder for `claude`, login badge and
/// the note that every opening is logged; the step-up follows its confirmation.
library;

import 'package:api_client/api_client.dart'
    show Host, HostProfileKindEnum, LlmCredentials;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/controller/terminal_prompts.dart';

/// Returns the badge of the LLM login of [host] (24 12: "Personal", "Organization", "None").
Widget llmLoginBadge(BuildContext context, Host host) =>
    switch (host.llmCredentials) {
      LlmCredentials.personal => const DsBillingBadge(
        mode: DsBillingMode.personal,
      ),
      LlmCredentials.organization => const DsBillingBadge(
        mode: DsBillingMode.organization,
      ),
      _ => Text(
        dsL10n(context).terminalLlmLoginNone,
        style: dsTextStyle(DsType.caption)
            .copyWith(color: context.dsColors.textSecondary),
      ),
    };

/// The working folders the host reported for the session user assigned to [selfUserId]
/// (24 11.1: `workdir_index` indexes this list); empty without an assignment.
List<String> ownWorkdirs(Host host, String selfUserId) {
  final own = host.sessionUsers?.where((u) => u.userId == selfUserId);
  if (own == null || own.isEmpty) return const [];
  final osUser = own.first.osUser;
  final reported = host.reportedSessionUsers?.where((r) => r.osUser == osUser);
  if (reported == null || reported.isEmpty) return const [];
  return reported.first.workdirs.toList();
}

/// The session user assigned to [selfUserId] on [host] when it is the owner account of the host
/// (ADR 0073): the owner's own account with administrator rights; null otherwise.
String? ownOwnerAccount(Host host, String selfUserId) {
  final own = host.sessionUsers?.where(
    (u) => u.userId == selfUserId && (u.ownerAccount ?? false),
  );
  if (own == null || own.isEmpty) return null;
  return own.first.osUser;
}

/// Pops the chosen [OpenChoice], or null on "Cancel".
///
/// ```dart
/// final choice = await showDialog<OpenChoice>(context: context,
///     builder: (_) => OpenTerminalDialog(host: host, initial: const OpenChoice(profile: 'shell'),
///         selfUserId: me));
/// ```
class OpenTerminalDialog extends StatefulWidget {
  /// Creates the dialog for [host] with the preselected [initial] choice.
  const OpenTerminalDialog({
    super.key,
    required this.host,
    required this.initial,
    required this.selfUserId,
  });

  /// The host to open on.
  final Host host;

  /// The preselected profile and working folder.
  final OpenChoice initial;

  /// The person's own user ID, to find the own working folders.
  final String selfUserId;

  @override
  State<OpenTerminalDialog> createState() => _OpenTerminalDialogState();
}

class _OpenTerminalDialogState extends State<OpenTerminalDialog> {
  late String _profile = widget.initial.profile;
  late int? _workdir = widget.initial.workdirIndex;

  bool get _isClaude => widget.host.profiles.any(
    (p) => p.name == _profile && p.kind == HostProfileKindEnum.claude,
  );

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final label = dsTextStyle(DsType.label).copyWith(color: c.textSecondary);
    final workdirs = ownWorkdirs(widget.host, widget.selfUserId);
    final ownerAccount = ownOwnerAccount(widget.host, widget.selfUserId);
    final needsWorkdir = _isClaude;
    final canConfirm =
        widget.host.profiles.any((p) => p.name == _profile) &&
        (!needsWorkdir || (_workdir != null && _workdir! < workdirs.length));
    return AlertDialog(
      title: Text(l.terminalOpenTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.columnMin * 3),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (ownerAccount != null) ...[
                DsBanner(
                  key: const ValueKey('terminalOpen.ownerAccount'),
                  tone: DsTone.warning,
                  text: l.terminalOwnerAccountWarning(ownerAccount),
                ),
                const SizedBox(height: DsSpace.s12),
              ],
              Text(l.terminalOpenHost, style: label),
              Text(widget.host.name, style: dsTextStyle(DsType.bodyStrong)),
              const SizedBox(height: DsSpace.s12),
              Text(l.terminalOpenProfile, style: label),
              RadioGroup<String>(
                groupValue: _profile,
                onChanged: (v) => setState(() {
                  if (v != null) _profile = v;
                }),
                child: Column(
                  children: [
                    for (final p in widget.host.profiles)
                      RadioListTile<String>(
                        value: p.name,
                        title: Text(p.displayName),
                        dense: true,
                        contentPadding: EdgeInsets.zero,
                      ),
                  ],
                ),
              ),
              if (needsWorkdir) ...[
                Text(l.terminalOpenWorkdir, style: label),
                DropdownButton<int>(
                  value: _workdir != null && _workdir! < workdirs.length
                      ? _workdir
                      : null,
                  isExpanded: true,
                  items: [
                    for (var i = 0; i < workdirs.length; i++)
                      DropdownMenuItem(value: i, child: Text(workdirs[i])),
                  ],
                  onChanged: (v) => setState(() => _workdir = v),
                ),
              ],
              const SizedBox(height: DsSpace.s12),
              Text(l.terminalOpenLogin, style: label),
              const SizedBox(height: DsSpace.s4),
              llmLoginBadge(context, widget.host),
              const SizedBox(height: DsSpace.s12),
              Text(
                l.terminalOpenLogged,
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: c.textSecondary),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.terminalCancel),
        ),
        FilledButton(
          onPressed: canConfirm
              ? () => Navigator.of(context).pop(
                  OpenChoice(
                    profile: _profile,
                    workdirIndex: needsWorkdir ? _workdir : null,
                  ),
                )
              : null,
          child: Text(l.terminalOpenConfirm),
        ),
      ],
    );
  }
}

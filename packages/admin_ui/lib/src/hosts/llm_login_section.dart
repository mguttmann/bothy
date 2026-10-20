// SPDX-License-Identifier: Apache-2.0

/// The LLM login of a host with the explanation of I6 (docs/spec/01-product.md 5.2 A8; 24 7.2,
/// 7.3; brief M4-37 Design notes 1 and 4, fixed requirement 14).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import '../data/self_repository.dart';
import '../self/self_feedback.dart';
import 'host_labels.dart';

/// The text of `409 i6_violation` in plain words: the problem, what I6 means for the person
/// (not even other owners, 24 7.3), and the server's `hint` with the way out, so the violation
/// is explained and not just rejected (brief fixed requirement 14).
String i6ViolationText(AppLocalizations l, ApiProblem problem) =>
    adminFailure(l, SelfActionFailed(problem))!.text;

/// The banner of an I6 violation ([i6ViolationText]).
Widget i6ViolationBanner(AppLocalizations l, ApiProblem problem) => DsBanner(
  key: const ValueKey('i6.violation'),
  tone: DsTone.danger,
  liveRegion: true,
  text: i6ViolationText(l, problem),
);

/// The choice of the LLM login: none, personal with a holder, organization; no choice is
/// preselected in the confirmation (a mandatory question, 24 3.2). Under "Personal" it says
/// what I6 means before any answer of the server.
///
/// Example:
/// ```dart
/// LlmLoginChoice(login: login, holderId: holder, people: people,
///     onLogin: (v) => setState(() => login = v), onHolder: (v) => setState(() => holder = v));
/// ```
class LlmLoginChoice extends StatelessWidget {
  /// Creates the choice.
  const LlmLoginChoice({
    super.key,
    required this.login,
    required this.holderId,
    required this.people,
    required this.onLogin,
    required this.onHolder,
  });

  /// The chosen declaration, or null.
  final HostLlmLogin? login;

  /// The holder of a personal login, or null.
  final String? holderId;

  /// The people a holder is chosen from.
  final List<AdminPerson> people;

  /// Picks the declaration.
  final ValueChanged<HostLlmLogin> onLogin;

  /// Picks the holder.
  final ValueChanged<String?> onHolder;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.hostLlmQuestion,
          style: dsTextStyle(DsType.bodyStrong)
              .copyWith(color: colors.textPrimary),
        ),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final v in HostLlmLogin.values)
              ChoiceChip(
                key: ValueKey('llmChoice.${v.wire}'),
                label: Text(llmLoginLabel(l, v)),
                selected: login == v,
                onSelected: (_) => onLogin(v),
              ),
          ],
        ),
        if (login == HostLlmLogin.personal) ...[
          DropdownButtonFormField<String>(
            key: const ValueKey('llmChoice.holder'),
            initialValue: holderId,
            decoration: InputDecoration(labelText: l.hostLlmHolder),
            items: [
              for (final p in people)
                DropdownMenuItem(value: p.id, child: Text(p.displayName)),
            ],
            onChanged: onHolder,
          ),
          const SizedBox(height: DsSpace.s4),
          Text(
            l.hostLlmI6Explained,
            key: const ValueKey('llm.i6Explained'),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
        ],
      ],
    );
  }
}

/// The LLM login of a confirmed [host] with "Change": a dialog with [LlmLoginChoice] and a
/// reason, then `setHostLlmCredentials` (step-up from the interceptor). A `409 i6_violation`
/// shows [i6ViolationBanner]; [onChanged] receives the changed host.
class LlmLoginSection extends StatefulWidget {
  /// Creates the section.
  const LlmLoginSection({
    super.key,
    required this.repository,
    required this.host,
    required this.people,
    required this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host.
  final AdminHost host;

  /// The people (for the holder's name and the picker).
  final List<AdminPerson> people;

  /// Receives the changed host.
  final ValueChanged<AdminHost> onChanged;

  @override
  State<LlmLoginSection> createState() => _LlmLoginSectionState();
}

class _LlmLoginSectionState extends State<LlmLoginSection>
    with AdminActionState<LlmLoginSection> {
  ApiProblem? _i6;

  Future<void> _change() async {
    final l = AppLocalizations.of(context);
    final h = widget.host;
    final answer = await showDialog<(HostLlmLogin, String?, String)>(
      context: context,
      builder: (_) => _LlmDialog(host: h, people: widget.people),
    );
    if (answer == null || !mounted) return;
    final (login, holder, reason) = answer;
    setState(() => _i6 = null);
    AdminHost? changed;
    await runChange('llm', () async {
      try {
        changed = await widget.repository.setLlmLogin(
          h,
          login,
          holderId: holder,
          reason: reason,
          purpose: l.stepUpPurposeHostLlmLogin(h.name),
        );
      } on SelfActionFailed catch (e) {
        final p = e.problem;
        if (p?.code != 'i6_violation') rethrow;
        if (mounted) setState(() => _i6 = p);
        return null;
      }
      return SelfNotice(l.hostLlmChanged(h.name));
    });
    if (changed case final host?) widget.onChanged(host);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final h = widget.host;
    final holder = h.credentialHolderId;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                h.llmLogin == HostLlmLogin.personal && holder != null
                    ? l.hostLlmPersonalOf(personName(widget.people, holder))
                    : llmLoginLabel(l, h.llmLogin),
                key: const ValueKey('llm.current'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: context.dsColors.textPrimary),
              ),
            ),
            if (h.state != HostLifecycle.retired && !denied.contains('llm'))
              TextButton(
                key: const ValueKey('llm.change'),
                onPressed: busy ? null : _change,
                child: Text(l.hostChange),
              ),
          ],
        ),
        if (h.llmLogin == HostLlmLogin.personal)
          Text(
            l.hostLlmI6Explained,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textSecondary),
          ),
        if (_i6 case final p?) ...[
          const SizedBox(height: DsSpace.s8),
          i6ViolationBanner(l, p),
        ],
        noticeBanner(),
      ],
    );
  }
}

/// The dialog of "Change": the choice, its holder and a reason; pops (login, holder, reason).
class _LlmDialog extends StatefulWidget {
  const _LlmDialog({required this.host, required this.people});

  final AdminHost host;
  final List<AdminPerson> people;

  @override
  State<_LlmDialog> createState() => _LlmDialogState();
}

class _LlmDialogState extends State<_LlmDialog> {
  final TextEditingController _reason = TextEditingController();
  late HostLlmLogin _login = widget.host.llmLogin;
  late String? _holder = widget.host.credentialHolderId;

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  bool get _ready =>
      _reason.text.trim().isNotEmpty &&
      (_login != HostLlmLogin.personal || _holder != null);

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ConfirmChange(
      title: l.hostLlmTitle(widget.host.name),
      body: l.hostLlmBody,
      confirmLabel: l.hostChange,
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          LlmLoginChoice(
            login: _login,
            holderId: _holder,
            people: widget.people,
            onLogin: (v) => setState(() => _login = v),
            onHolder: (v) => setState(() => _holder = v),
          ),
          TextField(
            key: const ValueKey('llm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
      canConfirm: _ready,
      onConfirm: (_) => Navigator.of(context).pop((
        _login,
        _login == HostLlmLogin.personal ? _holder : null,
        _reason.text.trim(),
      )),
    );
  }
}

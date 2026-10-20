// SPDX-License-Identifier: Apache-2.0

/// "Check chain": `verifyAuditChain` and its result panel (docs/spec/01-product.md 5.2 A6;
/// ADR 0021; brief M4-29 Design note 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../self/self_feedback.dart';
import 'audit_repository.dart';

/// The text of the failure reason [code] of a chain check.
String auditChainReasonText(AppLocalizations l, String? code) => switch (code) {
  'hash_mismatch' => l.auditChainReasonHash,
  'link_mismatch' => l.auditChainReasonLink,
  'seq_gap' => l.auditChainReasonGap,
  'checkpoint_mismatch' => l.auditChainReasonCheckpoint,
  'checkpoint_invalid' => l.auditChainReasonCheckpointInvalid,
  _ => l.auditChainReasonOther(code ?? ''),
};

/// The text of the chain check warning [code].
String auditChainWarningText(AppLocalizations l, String code) => switch (code) {
  'checkpoint_sink_not_configured' => l.auditChainWarningNoSink,
  _ => l.auditChainWarning(code),
};

/// The result of a chain check: "Chain checked up to no. n: unchanged" when [result] is ok,
/// otherwise the first broken entry and its reason in the danger style; below it the checked
/// range, the checkpoints compared and each warning (ADR 0021).
class AuditChainPanel extends StatelessWidget {
  /// Creates the panel for [result].
  const AuditChainPanel({super.key, required this.result});

  /// The answer of `verifyAuditChain`.
  final AuditChainCheck result;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final r = result;
    final broken = r.firstBrokenSeq;
    final small = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: ValueKey(r.ok ? 'audit.chain.ok' : 'audit.chain.broken'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        DsBanner(
          tone: r.ok ? DsTone.info : DsTone.danger,
          text: r.ok
              ? l.auditChainOk(r.checkedToSeq)
              : l.auditChainBroken(
                  broken ?? r.checkedToSeq,
                  auditChainReasonText(l, r.failureReason),
                ),
          liveRegion: true,
        ),
        const SizedBox(height: DsSpace.s4),
        Text(
          l.auditChainRange(
            r.checkedFromSeq,
            r.checkedToSeq,
            r.checkpointsCompared,
          ),
          style: small,
        ),
        for (final w in r.warnings)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              auditChainWarningText(l, w),
              key: ValueKey('audit.chain.warning.$w'),
              style: small.copyWith(color: colors.textPrimary),
            ),
          ),
      ],
    );
  }
}

/// The "Check chain" button and, after a check, its [AuditChainPanel].
///
/// [range] gives the `from_seq` and `to_seq` of the check: null checks from the last
/// checkpoint (the server's default), a pair checks the range of the current filter (brief
/// M4-29 Design note 2). The step-up of `audit.verify` comes from the server.
class AuditChainCheckButton extends StatefulWidget {
  /// Creates the control.
  const AuditChainCheckButton({
    super.key,
    required this.repository,
    this.range,
  });

  /// The audit log's calls.
  final AuditRepository repository;

  /// The range to check, or null for the server's default.
  final ({int from, int to})? Function()? range;

  @override
  State<AuditChainCheckButton> createState() => _AuditChainCheckButtonState();
}

class _AuditChainCheckButtonState extends State<AuditChainCheckButton> {
  bool _busy = false;
  AuditChainCheck? _result;
  SelfNotice? _failure;

  Future<void> _check() async {
    final l = AppLocalizations.of(context);
    final range = widget.range?.call();
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final result = await widget.repository.verifyAuditChain(
        fromSeq: range?.from,
        toSeq: range?.to,
        purpose: l.stepUpPurposeAuditVerify,
      );
      if (mounted) setState(() => _result = result);
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted && failure != null) {
        setState(
          () => _failure = SelfNotice(failure.text, tone: DsTone.danger),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: OutlinedButton(
            key: const ValueKey('audit.check'),
            onPressed: _busy ? null : _check,
            child: Text(l.auditCheckChain),
          ),
        ),
        if (_failure case final f?)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: selfNoticeBanner(f),
          ),
        if (_result case final r?)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: AuditChainPanel(result: r),
          ),
      ],
    );
  }
}

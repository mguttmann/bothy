// SPDX-License-Identifier: Apache-2.0

/// The grant wizard of A2: who, where, level and capabilities, with the server's preview
/// before every confirmation (docs/spec/01-product.md 5.2 A2; 07 9.3, 9.5; brief M4-27 Design
/// note 1 and fixed requirements 9, 11 to 14).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../agents/guided_flow.dart';
import '../agents/i1_explanation.dart';
import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;
import '../groups/member_editor.dart' show parseDay;
import '../self/self_feedback.dart';
import 'grant_labels.dart';
import 'level_step.dart';
import 'preview_panel.dart';
import 'subject_step.dart';
import 'target_step.dart';

/// A subject chosen before the wizard opens, for example the person of a row of "Who has
/// access?".
final class GrantSubjectChoice {
  /// Creates the choice.
  const GrantSubjectChoice(this.kind, this.id, this.name);

  /// The kind.
  final AdminSubjectKind kind;

  /// The principal or group ID.
  final String id;

  /// The display name.
  final String name;
}

/// The three steps of A2 and the [PreviewPanel].
///
/// Every change of the request (subject, target, level, capabilities, last day, reason) calls
/// `previewGrant` with the full request after [debounce] (07 9.3: "the console shows the
/// preview before every confirmation"); an answer for an older request is dropped. "Confirm"
/// is enabled only when the shown preview belongs to the current request, the server answered
/// `allowed = true` and the reason is set; it calls `createGrant`, and the step-up follows from
/// the interceptor. The wizard derives no permission of its own (15 1 principle 1).
///
/// When the preview reports an I1 conflict (10 11), "Confirm" stays disabled and the preview
/// offers the guided option instead: it asks the reason of the switch and sends ONE
/// `createGrant` with `resolve_i1`, so the server switches the agents and creates the grant in
/// one transaction (BR-07). Success shows [GuidedResult] and then [onCreated]; a failure
/// changed nothing and shows the problem, with the conflicts that remain.
///
/// [target], [subject] and [level] preselect parts of the request and [step] opens a step
/// (0 to 2), for example from "Who has access?" or the grants of a target. [onCreated]
/// receives the new grant; [onCancel] adds a "Cancel" button.
///
/// Example:
/// ```dart
/// final grant = await GrantWizard.show(context, repository: admin, target: channel);
/// ```
class GrantWizard extends StatefulWidget {
  /// Creates the wizard.
  const GrantWizard({
    super.key,
    required this.repository,
    this.target,
    this.subject,
    this.level,
    this.step = 0,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.debounce = const Duration(milliseconds: 300),
    this.onCreated,
    this.onCancel,
  });

  /// The preselected subject, or null.
  final GrantSubjectChoice? subject;

  /// The preselected level, or null.
  final AccessLevel? level;

  /// The step that opens first (0: who, 1: where, 2: level).
  final int step;

  /// The administration's calls.
  final AdminRepository repository;

  /// The preselected target, or null.
  final AdminTarget? target;

  /// The time zone in which the typed last day is read.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// The pause after a change before the preview is asked.
  final Duration debounce;

  /// Receives the created grant.
  final ValueChanged<AdminGrant>? onCreated;

  /// Leaves the wizard, or null for no "Cancel" button.
  final VoidCallback? onCancel;

  /// Shows the wizard in a dialog over [context]; returns the created grant or null.
  static Future<AdminGrant?> show(
    BuildContext context, {
    required AdminRepository repository,
    AdminTarget? target,
    String timeZone = 'UTC',
    DateTime Function() clock = DateTime.now,
  }) => showDialog<AdminGrant>(
    context: context,
    builder: (dialog) => Dialog(
      key: const ValueKey('wizard.dialog'),
      backgroundColor: dialog.dsColors.surfaceRaised,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(DsSpace.s24),
          child: GrantWizard(
            repository: repository,
            target: target,
            timeZone: timeZone,
            clock: clock,
            onCreated: (g) => Navigator.of(dialog).pop(g),
            onCancel: () => Navigator.of(dialog).pop(),
          ),
        ),
      ),
    ),
  );

  @override
  State<GrantWizard> createState() => _GrantWizardState();
}

class _GrantWizardState extends State<GrantWizard>
    with AdminActionState<GrantWizard> {
  final TextEditingController _until = TextEditingController();
  final TextEditingController _reason = TextEditingController();
  AccessNames? _names;
  int _step = 0;
  AdminSubjectKind _kind = AdminSubjectKind.user;
  String? _subjectId;
  String _subjectName = '';
  AdminTarget? _target;
  AccessLevel? _level;
  final Set<AccessCapability> _capabilities = {};
  Timer? _timer;
  String? _asked;
  String? _shownFor;
  GrantPreviewResult? _preview;
  SelfFailure? _previewFailure;
  List<InvariantFinding> _remaining = const [];

  /// Sets [target] and, when its kind changes between a host and any other target, the parts
  /// that depend on it: a host has no level and exactly `terminal` (07 2.12), the others have a
  /// level and no `terminal`.
  void _setTarget(AdminTarget? target) {
    final wasHost = _target?.kind == AdminTargetKind.host;
    final isHost = target?.kind == AdminTargetKind.host;
    _target = target;
    if (isHost) {
      _level = AccessLevel.none;
      _capabilities
        ..clear()
        ..add(AccessCapability.terminal);
    } else if (wasHost) {
      _level = null;
      _capabilities.clear();
    }
  }

  @override
  void initState() {
    super.initState();
    _step = widget.step.clamp(0, 2);
    _level = widget.level;
    _setTarget(widget.target);
    if (widget.subject case final subject?) {
      _kind = subject.kind;
      _subjectId = subject.id;
      _subjectName = subject.name;
    }
    _until.addListener(_changed);
    _reason.addListener(_changed);
    _load();
  }

  @override
  void dispose() {
    _timer?.cancel();
    _until.dispose();
    _reason.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final names = await AccessNames.load(widget.repository);
    if (!mounted) return;
    setState(() => _names = names);
    // A preselected request is previewed at once.
    _changed();
  }

  /// The end of the validity: the start of the day after the typed day in the person's time
  /// zone, so the grant holds through the typed day (07 2.5). Null for no day; throws a
  /// [FormatException] for a text that is no day.
  DateTime? _expiresAt() {
    final text = _until.text.trim();
    if (text.isEmpty) return null;
    final day = parseDay(text);
    if (day == null) throw const FormatException('no day');
    final format = L10nFormat(Localizations.localeOf(context));
    try {
      return format.dayStart(
        day.year,
        day.month,
        day.day + 1,
        timeZone: widget.timeZone,
      );
    } on ArgumentError {
      return DateTime.utc(day.year, day.month, day.day + 1);
    }
  }

  bool get _untilValid {
    try {
      _expiresAt();
      return true;
    } on FormatException {
      return false;
    }
  }

  /// The full request, or null while a part is missing or the day is invalid.
  GrantRequest? _request() {
    final subject = _subjectId;
    final target = _target;
    final level = _level;
    if (subject == null || target == null || level == null || !_untilValid) {
      return null;
    }
    return GrantRequest(
      subjectKind: _kind,
      subjectId: subject,
      target: target,
      level: level,
      capabilities: Set.of(_capabilities),
      expiresAt: _expiresAt(),
      reason: _reason.text.trim(),
    );
  }

  static String _keyOf(GrantRequest r) => [
    r.subjectKind.name,
    r.subjectId,
    r.target.key,
    r.level.wire,
    [for (final c in r.capabilities) c.wire]..sort(),
    r.expiresAt?.toIso8601String(),
    r.reason,
  ].join('|');

  /// Runs after every change of the request: the preview of the new request follows.
  void _changed() {
    if (!mounted) return;
    setState(() {});
    final request = _request();
    if (request == null) {
      _timer?.cancel();
      _asked = null;
      return;
    }
    final key = _keyOf(request);
    // A notification without a change (cursor, selection, trailing space) keeps the preview
    // that is pending or shown for this request.
    if (key == _asked) return;
    _timer?.cancel();
    _asked = key;
    if (widget.debounce == Duration.zero) {
      unawaited(_ask(request, key));
    } else {
      _timer = Timer(widget.debounce, () => _ask(request, key));
    }
  }

  Future<void> _ask(GrantRequest request, String key) async {
    final l = AppLocalizations.of(context);
    try {
      final preview = await widget.repository.previewGrant(request);
      if (!mounted || key != _asked) return;
      setState(() {
        _preview = preview;
        _shownFor = key;
        _previewFailure = null;
      });
    } on Object catch (e) {
      if (!mounted || key != _asked) return;
      setState(() {
        _preview = null;
        _shownFor = null;
        // The next change asks again, even for the same request.
        _asked = null;
        _previewFailure = adminFailure(l, e);
      });
    }
  }

  /// Whether the shown preview belongs to the current request with its reason set: the
  /// condition of both "Confirm" and the guided option.
  bool get _previewCurrent {
    final request = _request();
    return !busy &&
        request != null &&
        request.reason.isNotEmpty &&
        _preview != null &&
        _shownFor == _keyOf(request);
  }

  bool get _canConfirm => _previewCurrent && _preview!.allowed;

  Future<void> _confirm() async {
    final l = AppLocalizations.of(context);
    final request = _request();
    if (request == null) return;
    AdminGrant? created;
    await runChange('create', () async {
      created = await widget.repository.createGrant(
        request,
        purpose: l.stepUpPurposeCreateGrant(
          _subjectName,
          targetLabel(l, request.target),
        ),
      );
      return SelfNotice(l.adminGrantCreated(_subjectName));
    });
    if (created case final grant?) widget.onCreated?.call(grant);
  }

  /// The guided option of 10 11 for the I1 [findings] of the preview: the reason, then one
  /// `createGrant` with `resolve_i1` (05 rule 1.6), never a switch on its own.
  Future<void> _guided(List<InvariantFinding> findings) async {
    final l = AppLocalizations.of(context);
    final request = _request();
    final names = _names;
    if (request == null || names == null || !_previewCurrent) return;
    final ids = guidedAgentIds(findings);
    final reason = await GuidedFlow.askReason(
      context,
      agents: guidedAgentNames(names, findings),
    );
    if (reason == null || !mounted) return;
    setState(() => _remaining = const []);
    AdminGrant? created;
    await runChange('guided', () async {
      try {
        created = await widget.repository.createGrant(
          request,
          purpose: l.stepUpPurposeCreateGrant(
            _subjectName,
            targetLabel(l, request.target),
          ),
          resolveI1: I1Resolution(ids, reason: reason),
        );
      } on SelfActionFailed catch (e) {
        // The server's transaction changed nothing; a conflict it still finds (an agent the
        // preview did not name) is explained with its principals (10 5.4).
        if (mounted && e.conflicts.isNotEmpty) {
          setState(() => _remaining = e.conflicts);
        }
        rethrow;
      }
      return SelfNotice(l.adminGrantCreated(_subjectName));
    });
    final grant = created;
    if (grant == null || !mounted) return;
    await GuidedResult.show(
      context,
      repository: widget.repository,
      agentIds: ids,
    );
    widget.onCreated?.call(grant);
  }

  bool get _stepReady => switch (_step) {
    0 => _subjectId != null,
    1 => _target != null,
    _ => true,
  };

  Widget _stepBody(AccessNames names) => switch (_step) {
    0 => SubjectStep(
      names: names,
      kind: _kind,
      selectedId: _subjectId,
      onKind: (kind) {
        _kind = kind;
        _subjectId = null;
        _changed();
      },
      onSelect: (id, name) {
        _subjectId = id;
        _subjectName = name;
        _changed();
      },
    ),
    1 => TargetStep(
      targets: names.targets,
      hosts: names.hosts,
      selected: _target,
      onSelect: (t) {
        _setTarget(t);
        _changed();
      },
    ),
    _ => LevelStep(
      host: _target?.kind == AdminTargetKind.host,
      level: _level,
      capabilities: _capabilities,
      until: _until,
      untilValid: _untilValid,
      reason: _reason,
      onLevel: (v) {
        _level = v;
        _changed();
      },
      onCapability: (c, on) {
        on ? _capabilities.add(c) : _capabilities.remove(c);
        _changed();
      },
    ),
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final names = _names;
    final titles = [l.adminWizardWho, l.adminWizardWhere, l.adminWizardLevel];
    return Column(
      key: const ValueKey('wizard'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminGrantsCreate,
                  style: dsTextStyle(DsType.title)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            Text(
              l.adminWizardStep(_step + 1, titles.length),
              key: const ValueKey('wizard.step'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        Text(
          titles[_step],
          style: dsTextStyle(DsType.bodyStrong)
              .copyWith(color: colors.textPrimary),
        ),
        const SizedBox(height: DsSpace.s8),
        if (names == null)
          const Center(child: CircularProgressIndicator())
        else
          _stepBody(names),
        if (_step == 2 && _level != null && names != null) ...[
          const SizedBox(height: DsSpace.s16),
          if (_previewFailure case final failure?)
            DsBanner(
              key: const ValueKey('preview.failed'),
              tone: DsTone.warning,
              text: failure.text,
            )
          else
            PreviewPanel(
              preview: _preview,
              names: names,
              person: _subjectName,
              // A refused guided request (403) hides the option (15 1 principle 1).
              onGuided: denied.contains('guided') ? null : _guided,
              guidedReady: _previewCurrent,
            ),
        ],
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        if (_remaining.isNotEmpty && names != null) ...[
          const SizedBox(height: DsSpace.s8),
          I1Explanation(conflicts: _remaining, names: names),
        ],
        const SizedBox(height: DsSpace.s12),
        Wrap(
          alignment: WrapAlignment.end,
          spacing: DsSpace.s8,
          children: [
            if (widget.onCancel case final cancel?)
              TextButton(
                key: const ValueKey('wizard.cancel'),
                onPressed: cancel,
                child: Text(l.selfCancel),
              ),
            if (_step > 0)
              TextButton(
                key: const ValueKey('wizard.back'),
                onPressed: () => setState(() => _step--),
                child: Text(l.adminWizardBack),
              ),
            if (_step < 2)
              FilledButton(
                key: const ValueKey('wizard.next'),
                onPressed: _stepReady ? () => setState(() => _step++) : null,
                child: Text(l.adminWizardNext),
              )
            else
              FilledButton(
                key: const ValueKey('wizard.confirm'),
                onPressed: _canConfirm ? _confirm : null,
                child: Text(l.adminWizardConfirm),
              ),
          ],
        ),
      ],
    );
  }
}

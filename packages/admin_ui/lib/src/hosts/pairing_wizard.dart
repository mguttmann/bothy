// SPDX-License-Identifier: Apache-2.0

/// The three steps of pairing a host (second wireframe of docs/spec/01-product.md 5.2 A8;
/// 24 3.2, 4.3, 7; brief M4-37 Design note 1 and fixed requirements 12 to 15).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import '../data/self_repository.dart';
import '../self/self_feedback.dart';
import 'create_host_dialog.dart';
import 'fingerprint_confirm.dart';
import 'host_labels.dart';
import 'llm_login_section.dart';
import 'server_host_section.dart';
import 'session_users_section.dart';

/// The binary of the host service, one of the places that carry the product name
/// (docs/spec/02-architecture.md "Places that carry the product name"; 24 3.2 item 2).
const String hostServiceCommand = 'bothy-host';

/// The command an operator types on the host: `bothy-host pair --server <origin> --code <code>`.
String pairCommand(String origin, String code) =>
    '$hostServiceCommand pair --server $origin --code $code';

/// The remaining time until [end] seen at [now] as `m:ss`, or null when it passed.
String? remainingTime(DateTime end, DateTime now) {
  final left = end.difference(now);
  if (left <= Duration.zero) return null;
  final seconds = left.inSeconds;
  return '${seconds ~/ 60}:${(seconds % 60).toString().padLeft(2, '0')}';
}

/// How often "Confirm" reads the waiting host again after `412 version_conflict` before the
/// conflict is shown: every report of the host raises its version (24 3.2).
const int confirmVersionAttempts = 3;

/// The pairing wizard.
///
/// Step 1 creates the host ([CreateHostDialog], `createHost`). Step 2 shows the one-time code
/// in the command line built from the instance origin, copyable, with its remaining time from
/// `expires_at`; "New code" issues another (`issueHostPairingCode`); "Next" reloads the host
/// and goes on once the host reported its keys (`confirming`). Step 3 asks the first
/// characters of the fingerprint ([FingerprintConfirm]), the LLM login ([LlmLoginChoice], with
/// I6 explained), the server host ([ServerHostChoice]), the session users per person
/// ([SessionUserRows], names only from `reported_session_users`) and a reason; "Confirm with
/// passkey / device" sends `confirmHost`, and the step-up comes from the interceptor.
///
/// `409 fingerprint_mismatch` keeps step 3 with the attempts the server names; at `0` the host
/// fell back to `pairing` and the wizard returns to step 2, which needs a new code (TM-T19).
/// `409 i6_violation` and refused session users are explained. [host] opens the wizard for an
/// existing host (step 2, or step 3 in `confirming`); [code] shows a code just issued (the
/// re-pairing of the host page). [onDone] receives the confirmed host; [onClose] adds "Close".
///
/// Example:
/// ```dart
/// final host = await PairingWizard.show(context, repository: admin);
/// ```
class PairingWizard extends StatefulWidget {
  /// Creates the wizard.
  const PairingWizard({
    super.key,
    required this.repository,
    this.host,
    this.code,
    this.reason,
    this.clock = DateTime.now,
    this.onDone,
    this.onClose,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host to continue, or null to create one.
  final AdminHost? host;

  /// A code issued before the wizard opened, or null.
  final HostPairingCodeInfo? code;

  /// The reason given before the wizard opened (re-pairing), or null.
  final String? reason;

  /// The current time.
  final DateTime Function() clock;

  /// Receives the confirmed host.
  final ValueChanged<AdminHost>? onDone;

  /// Closes the wizard, or null for no "Close" button.
  final VoidCallback? onClose;

  /// Shows the wizard in a dialog over [context]; returns the confirmed host or null.
  static Future<AdminHost?> show(
    BuildContext context, {
    required AdminRepository repository,
    AdminHost? host,
    HostPairingCodeInfo? code,
    String? reason,
    DateTime Function() clock = DateTime.now,
  }) => showDialog<AdminHost>(
    context: context,
    builder: (dialog) => Dialog(
      key: const ValueKey('pairing.dialog'),
      backgroundColor: dialog.dsColors.surfaceRaised,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(DsSpace.s24),
          child: PairingWizard(
            repository: repository,
            host: host,
            code: code,
            reason: reason,
            clock: clock,
            onDone: (h) => Navigator.of(dialog).pop(h),
            onClose: () => Navigator.of(dialog).pop(),
          ),
        ),
      ),
    ),
  );

  @override
  State<PairingWizard> createState() => _PairingWizardState();
}

class _PairingWizardState extends State<PairingWizard>
    with AdminActionState<PairingWizard> {
  final TextEditingController _prefix = TextEditingController();
  final TextEditingController _reason = TextEditingController();
  List<AdminPerson> _people = const [];
  int _step = 0;
  AdminHost? _host;
  HostPairingCodeInfo? _code;
  Timer? _ticker;
  String? _remaining;
  bool _notYet = false;
  bool _fellBack = false;
  int? _attemptsLeft;
  String? _mismatch;
  ApiProblem? _i6;
  String? _sessionProblem;
  HostLlmLogin? _login;
  String? _holder;
  bool? _serverHost;

  /// The session user rows of step 3; they stay when the owner goes back and are dropped
  /// when the host falls back to pairing (its next report may name other users).
  final List<SessionUserRow> _rows = [];

  @override
  void initState() {
    super.initState();
    _host = widget.host;
    _code = widget.code;
    _reason.text = widget.reason ?? '';
    _prefix.addListener(() => setState(() {}));
    _reason.addListener(() => setState(() {}));
    _step = switch (_host?.state) {
      null => 0,
      HostLifecycle.confirming when _code == null => 2,
      _ => 1,
    };
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) => _tick());
    _tick();
    _loadPeople();
  }

  @override
  void dispose() {
    _ticker?.cancel();
    _prefix.dispose();
    _reason.dispose();
    super.dispose();
  }

  Future<void> _loadPeople() async {
    try {
      final people = await allPeople(widget.repository);
      if (!mounted) return;
      setState(() {
        _people = [
          for (final p in people)
            if (p.status == AdminPersonStatus.active ||
                p.status == AdminPersonStatus.pending)
              p,
        ];
      });
    } on Object {
      // Without the list the pickers stay empty; the server's answers still apply.
    }
  }

  /// Updates the remaining time; a rebuild only when the shown text changes.
  void _tick() {
    final code = _code;
    final text = code == null
        ? null
        : remainingTime(code.expiresAt, widget.clock());
    if (text == _remaining || !mounted) return;
    setState(() => _remaining = text);
  }

  Future<void> _create(String name, String? assignee, String reason) async {
    final l = AppLocalizations.of(context);
    HostPairingCodeInfo? code;
    await runChange('create', () async {
      code = await widget.repository.createHost(
        name: name,
        assigneeId: assignee,
        reason: reason,
        purpose: l.stepUpPurposeCreateHost(name),
      );
      return null;
    });
    final issued = code;
    if (issued == null || !mounted) return;
    AdminHost? host;
    try {
      host = await widget.repository.host(issued.hostId);
    } on Object {
      host = null;
    }
    if (!mounted) return;
    setState(() {
      _reason.text = reason;
      _host = host;
      _code = issued;
      _step = 1;
    });
    _tick();
  }

  Future<void> _newCode() async {
    final l = AppLocalizations.of(context);
    final h = _host;
    if (h == null) return;
    var reason = _reason.text.trim();
    if (reason.isEmpty) {
      final typed = await askHostReason(
        context,
        title: l.hostPairingNewCode,
        body: l.hostRepairBody,
        confirmLabel: l.hostPairingNewCode,
      );
      if (typed == null || !mounted) return;
      reason = typed;
      _reason.text = typed;
    }
    HostPairingCodeInfo? code;
    await runChange('code', () async {
      code = await widget.repository.newPairingCode(
        h.id,
        reason: reason,
        purpose: l.stepUpPurposePairingCode(h.name),
      );
      return null;
    });
    if (code == null || !mounted) return;
    setState(() {
      _code = code;
      _fellBack = false;
      _notYet = false;
    });
    _tick();
  }

  Future<void> _next() async {
    final h = _host;
    if (h == null) return;
    try {
      final fresh = await widget.repository.host(h.id);
      if (!mounted) return;
      setState(() {
        _host = fresh;
        _notYet = fresh.state != HostLifecycle.confirming;
        if (!_notYet) _step = 2;
      });
    } on Object catch (e) {
      if (!mounted) return;
      final failure = adminFailure(AppLocalizations.of(context), e);
      if (failure != null) {
        setState(() => notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    }
  }

  bool get _ready =>
      !busy &&
      _prefix.text.trim().isNotEmpty &&
      _reason.text.trim().isNotEmpty &&
      _login != null &&
      (_login != HostLlmLogin.personal || _holder != null) &&
      _serverHost != null;

  Future<void> _confirm() async {
    final l = AppLocalizations.of(context);
    final h = _host;
    final login = _login;
    final serverHost = _serverHost;
    if (h == null || login == null || serverHost == null) return;
    setState(() {
      _i6 = null;
      _sessionProblem = null;
    });
    AdminHost? confirmed;
    // Exactly the complete rows step 3 shows (brief Design note 1).
    List<HostAssignment> shown(AdminHost host) => [
      for (final r in _rows)
        if (offeredNames(host).contains(r.osUser)) ?r.assignment,
    ];
    final answers = shown(h);
    await runChange('confirm', () async {
      var current = h;
      try {
        for (var attempt = 1; ; attempt++) {
          try {
            confirmed = await widget.repository.confirmHost(
              current,
              HostConfirmRequest(
                fingerprintPrefix: _prefix.text.trim(),
                llmLogin: login,
                credentialHolderId: login == HostLlmLogin.personal
                    ? _holder
                    : null,
                declaredServerHost: serverHost,
                sessionUsers: answers,
                reason: _reason.text.trim(),
              ),
              purpose: l.stepUpPurposeConfirmHost(h.name),
            );
            break;
          } on SelfActionFailed catch (e) {
            // The host service reports while the host waits for its confirmation, and each
            // report raises the host's version (24 3.2), so the version read on "Next" may be
            // old by the time the owner confirms (brief M4-38, found by the terminal E2E). Read
            // the host again and send the same answers, as long as it still offers every
            // session user they name; otherwise the conflict stays for the owner to see.
            if (e.problem?.code != 'version_conflict' ||
                attempt >= confirmVersionAttempts) {
              rethrow;
            }
            final fresh = await widget.repository.host(h.id);
            if (fresh.state != HostLifecycle.confirming ||
                !_sameAssignments(shown(fresh), answers)) {
              rethrow;
            }
            current = fresh;
            if (mounted) setState(() => _host = fresh);
          }
        }
      } on FingerprintMismatch catch (m) {
        await _mismatched(m);
        return null;
      } on SelfActionFailed catch (e) {
        final p = e.problem;
        if (p?.code == 'i6_violation') {
          if (mounted) setState(() => _i6 = p);
          return null;
        }
        final text = sessionUserProblemText(l, p);
        if (text == null) rethrow;
        if (mounted) setState(() => _sessionProblem = text);
        return null;
      }
      return SelfNotice(l.hostConfirmed(h.name));
    });
    if (confirmed case final host?) widget.onDone?.call(host);
  }

  static bool _sameAssignments(List<HostAssignment> a, List<HostAssignment> b) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i].userId != b[i].userId || a[i].osUser != b[i].osUser) {
        return false;
      }
    }
    return true;
  }

  /// 24 3.2: a mismatch keeps step 3 with the server's count; at `0` the host is back in
  /// `pairing` without keys, and step 2 asks for a new code.
  Future<void> _mismatched(FingerprintMismatch m) async {
    final l = AppLocalizations.of(context);
    if (m.attemptsRemaining > 0) {
      setState(() {
        _attemptsLeft = m.attemptsRemaining;
        _mismatch = problemText(l, m.problem.code);
      });
      return;
    }
    AdminHost? fresh;
    try {
      fresh = await widget.repository.host(_host!.id);
    } on Object {
      fresh = null;
    }
    if (!mounted) return;
    setState(() {
      _host = fresh ?? _host;
      _fellBack = true;
      _attemptsLeft = null;
      _code = null;
      _remaining = null;
      _prefix.clear();
      _rows.clear();
      _step = 1;
    });
  }

  Widget _stepOne() =>
      CreateHostDialog(people: _people, busy: busy, onCreate: _create);

  Widget _stepTwo(AppLocalizations l) {
    final colors = context.dsColors;
    final code = _code;
    final origin = widget.repository.serverOrigin;
    final command = code == null ? null : pairCommand(origin, code.code);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_fellBack)
          DsBanner(
            key: const ValueKey('fingerprint.fallback'),
            tone: DsTone.danger,
            liveRegion: true,
            text: l.hostFingerprintFallback,
          ),
        if (command == null) ...[
          if (!_fellBack)
            Text(
              l.hostPairingNeedsCode,
              key: const ValueKey('pairing.needsCode'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textPrimary),
            ),
        ] else ...[
          Text(
            l.hostPairingCommandIntro,
            style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
          ),
          const SizedBox(height: DsSpace.s8),
          Container(
            padding: const EdgeInsets.all(DsSpace.s8),
            decoration: BoxDecoration(
              color: colors.surfaceCanvas,
              border: Border.all(color: colors.borderDefault),
              borderRadius: BorderRadius.circular(DsRadius.md),
            ),
            child: SelectableText(
              command,
              key: const ValueKey('pairing.command'),
              style: dsTextStyle(DsType.code)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          Row(
            children: [
              TextButton(
                key: const ValueKey('pairing.copy'),
                onPressed: () =>
                    Clipboard.setData(ClipboardData(text: command)),
                child: Text(l.onboardingCopy),
              ),
              const Spacer(),
              Text(
                _remaining == null
                    ? l.hostPairingExpired
                    : l.hostPairingExpiresIn(_remaining!),
                key: const ValueKey('pairing.expires'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textSecondary),
              ),
            ],
          ),
        ],
        if (_notYet)
          DsBanner(
            key: const ValueKey('pairing.notYet'),
            tone: DsTone.info,
            liveRegion: true,
            text: l.hostPairingNotYet,
          ),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('pairing.newCode'),
              onPressed: busy ? null : _newCode,
              child: Text(l.hostPairingNewCode),
            ),
            if (command != null)
              FilledButton(
                key: const ValueKey('pairing.next'),
                onPressed: busy ? null : _next,
                child: Text(l.adminWizardNext),
              ),
          ],
        ),
      ],
    );
  }

  Widget _stepThree(AppLocalizations l) {
    final h = _host!;
    final i6 = _i6;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        FingerprintConfirm(
          controller: _prefix,
          attemptsRemaining: _attemptsLeft,
          mismatchText: _mismatch,
        ),
        const SizedBox(height: DsSpace.s16),
        LlmLoginChoice(
          login: _login,
          holderId: _holder,
          people: _people,
          onLogin: (v) => setState(() => _login = v),
          onHolder: (v) => setState(() => _holder = v),
        ),
        if (i6 != null) ...[
          const SizedBox(height: DsSpace.s8),
          i6ViolationBanner(l, i6),
        ],
        const SizedBox(height: DsSpace.s16),
        ServerHostChoice(
          value: _serverHost,
          onChanged: (v) => setState(() => _serverHost = v),
        ),
        const SizedBox(height: DsSpace.s16),
        SessionUserRows(
          host: h,
          people: _people,
          rows: _rows,
          onChanged: () => setState(() {}),
        ),
        if (_sessionProblem case final text?)
          DsBanner(
            key: const ValueKey('sessionUsers.problem'),
            tone: DsTone.danger,
            liveRegion: true,
            text: text,
          ),
        TextField(
          key: const ValueKey('pairing.reason'),
          controller: _reason,
          decoration: InputDecoration(labelText: l.adminReasonRequired),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      key: const ValueKey('pairing.wizard'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  _host == null
                      ? l.hostPairingTitle
                      : '${l.hostPairingTitle}: ${_host!.name}',
                  style: dsTextStyle(DsType.title)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            Text(
              l.adminWizardStep(_step + 1, 3),
              key: const ValueKey('pairing.step'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s16),
        switch (_step) {
          0 => _stepOne(),
          1 => _stepTwo(l),
          _ => _stepThree(l),
        },
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        Wrap(
          alignment: WrapAlignment.end,
          spacing: DsSpace.s8,
          children: [
            if (widget.onClose case final close?)
              TextButton(
                key: const ValueKey('pairing.close'),
                onPressed: close,
                child: Text(l.hostPairingClose),
              ),
            if (_step == 2) ...[
              if (_code != null)
                TextButton(
                  key: const ValueKey('pairing.back'),
                  onPressed: () => setState(() => _step = 1),
                  child: Text(l.adminWizardBack),
                ),
              FilledButton(
                key: const ValueKey('pairing.confirm'),
                onPressed: _ready ? _confirm : null,
                child: Text(l.hostConfirmSubmit),
              ),
            ],
          ],
        ),
      ],
    );
  }
}

// SPDX-License-Identifier: Apache-2.0

/// The installation of an app from a manifest file (11 4.1; brief M6-08 Design note 1, Ruling
/// 1): file, the server's validation, the consent, install, the preview of each chosen grant,
/// grants and token, the token shown once.
library;

import 'package:api_client/api_client.dart' show ProblemFieldError;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/principal_tokens.dart' show tokenExpiry;
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../grants/grant_labels.dart' show AccessNames, targetLabel;
import '../grants/preview_panel.dart';
import '../secrets/once_only_secret.dart';
import 'app_labels.dart';
import 'consent_view.dart';
import 'manifest_picker.dart';

/// Where the flow stands.
enum _Stage { pick, consent, grants }

/// The status of one step of the progress after "Install".
sealed class _Step {
  const _Step();
}

final class _Waiting extends _Step {
  const _Waiting();
}

final class _Finished extends _Step {
  const _Finished();
}

final class _Failed extends _Step {
  const _Failed(this.reason);

  final String reason;
}

/// The install dialog: the owner picks a manifest file (never an address, 11 4.1), the server
/// validates it (`validateManifest`, nothing is stored) and a refusal shows each failed rule
/// with its pointer and the server's detail; a file the client cannot read shows its problem
/// and never reaches the server.
///
/// The consent ([ConsentView]) lists what the app asks for; optional scopes and events start
/// checked (the manifest asks for them) and the owner unchecks; each requested grant gets a
/// target picker with the server's spaces or channels of its kind, or "Do not create" (an
/// instance grant has no picker: its target is the instance). The token fields (name, last day,
/// optional DPoP thumbprint) and the reason complete it.
///
/// "Install" runs `installApp` (step-up through the interceptor). The grants step then shows
/// the server's preview of every chosen grant (`previewGrant` with the app as subject) before
/// any grant exists; "Create access grants and token" runs `createGrant` per target, then
/// `createToken` and `getWebhookKeys`, and shows the token once in [OnceOnlySecret] (keys
/// `appToken.*`) with the key ID of the active signing key. A failed step stops the flow: the
/// progress shows what finished and the server's reason, "Try again" runs the failed step and
/// the rest, "Uninstall" runs `uninstallApp`. Nothing is retried silently, and a cancelled
/// step-up says nothing.
///
/// The token's secret is a local of the method that shows it: no field of this state holds it,
/// so after the secret dialog closes nothing keeps it (fixed requirement 17).
///
/// Pops true when the server changed (the app was installed, or uninstalled again), null
/// otherwise.
///
/// Example:
/// ```dart
/// final changed = await InstallAppDialog.show(context, repository: admin, pickManifest: pick);
/// ```
class InstallAppDialog extends StatefulWidget {
  /// Creates the dialog.
  const InstallAppDialog({
    super.key,
    required this.repository,
    required this.pickManifest,
    this.timeZone = 'UTC',
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host's file dialog for a manifest.
  final ManifestPicker pickManifest;

  /// The time zone of the signed-in person: the token holds through its last day there.
  final String timeZone;

  /// Shows the dialog over [context]; returns true when the server changed.
  static Future<bool?> show(
    BuildContext context, {
    required AdminRepository repository,
    required ManifestPicker pickManifest,
    String timeZone = 'UTC',
  }) => showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => InstallAppDialog(
      repository: repository,
      pickManifest: pickManifest,
      timeZone: timeZone,
    ),
  );

  @override
  State<InstallAppDialog> createState() => _InstallAppDialogState();
}

class _InstallAppDialogState extends State<InstallAppDialog> {
  _Stage _stage = _Stage.pick;
  bool _busy = false;

  String? _fileName;
  ManifestFileProblem? _fileProblem;
  List<ProblemFieldError>? _errors;
  String? _checkFailure;

  ManifestUpload? _upload;
  List<AdminTarget> _targets = const [];
  final Set<String> _scopes = {};
  final Set<String> _events = {};

  /// The picked target per index of `requested_grants`; absent is "Do not create".
  final Map<int, AdminTarget> _picked = {};

  final TextEditingController _tokenName = TextEditingController();
  final TextEditingController _lastDay = TextEditingController();
  final TextEditingController _dpop = TextEditingController();
  final TextEditingController _reason = TextEditingController();

  AppRecord? _app;
  _Step _install = const _Waiting();
  final Map<int, _Step> _grantSteps = {};
  _Step _token = const _Waiting();
  final Map<int, GrantPreviewResult> _previews = {};
  final Map<int, String> _previewFailures = {};
  String? _uninstallFailure;

  @override
  void initState() {
    super.initState();
    for (final c in [_tokenName, _lastDay, _dpop, _reason]) {
      c.addListener(() => setState(() {}));
    }
  }

  @override
  void dispose() {
    _tokenName.dispose();
    _lastDay.dispose();
    _dpop.dispose();
    _reason.dispose();
    super.dispose();
  }

  AppManifestInfo get _manifest => _upload!.info;

  /// The grants that will be created: each requested grant with its target, by index.
  List<(int, AppGrantAsk, AdminTarget)> get _chosen => [
    for (final (i, g) in _manifest.requestedGrants.indexed)
      if (_targetOf(i, g) case final t?) (i, g, t),
  ];

  AdminTarget? _targetOf(int index, AppGrantAsk ask) =>
      switch (ask.targetKind) {
        AdminTargetKind.instance => AdminTarget.instance,
        AdminTargetKind.space || AdminTargetKind.channel => _picked[index],
        // A kind this version cannot pick (null, host): the server's validation decides.
        _ => null,
      };

  DateTime? _expiresAt(BuildContext context) =>
      tokenExpiry(context, _lastDay.text, widget.timeZone);

  /// The text of a failed call, or null when the person cancelled the step-up.
  String? _failure(Object error) =>
      adminFailure(AppLocalizations.of(context), error)?.text;

  Future<void> _pick() async {
    final file = await widget.pickManifest();
    if (file == null || !mounted) return;
    setState(() {
      _fileName = file.name;
      _fileProblem = null;
      _errors = null;
      _checkFailure = null;
    });
    final ManifestUpload upload;
    try {
      upload = ManifestUpload.read(file.name, file.bytes);
    } on ManifestFileError catch (e) {
      setState(() => _fileProblem = e.problem);
      return;
    }
    setState(() => _busy = true);
    try {
      final check = await widget.repository.validateManifest(upload);
      if (!mounted) return;
      if (!check.valid) {
        setState(() => _errors = check.errors);
        return;
      }
      List<AdminTarget> targets;
      try {
        targets = await widget.repository.targets();
      } on Object {
        // Without the list every picker offers "Do not create" only; the grants can follow
        // under Access grants.
        targets = const [];
      }
      if (!mounted) return;
      final info = upload.info;
      setState(() {
        _upload = upload;
        _targets = targets;
        _scopes
          ..clear()
          ..addAll(info.requiredScopes)
          ..addAll(info.optionalScopes);
        _events
          ..clear()
          ..addAll(info.events);
        _picked.clear();
        _stage = _Stage.consent;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _checkFailure = _failure(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _installApp() async {
    final l = AppLocalizations.of(context);
    final m = _manifest;
    setState(() {
      _busy = true;
      _install = const _Waiting();
    });
    try {
      final app = await widget.repository.installApp(
        _upload!,
        // The server's order: every required scope, then the chosen optional ones.
        grantedScopes: [
          ...m.requiredScopes,
          for (final s in m.optionalScopes)
            if (_scopes.contains(s)) s,
        ],
        eventSubscriptions: [
          for (final e in m.events)
            if (_events.contains(e)) e,
        ],
        reason: _reason.text.trim(),
        purpose: l.adminAppPurposeInstall(m.name),
      );
      if (!mounted) return;
      setState(() {
        _app = app;
        _install = const _Finished();
        for (final (i, _, _) in _chosen) {
          _grantSteps[i] = const _Waiting();
        }
        _stage = _Stage.grants;
      });
      await _preview();
    } on Object catch (e) {
      final failure = _failure(e);
      if (mounted && failure != null) {
        setState(() => _install = _Failed(failure));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Asks the server what each chosen grant changes (`previewGrant`, nothing is stored).
  Future<void> _preview() async {
    final app = _app!;
    for (final (i, ask, target) in _chosen) {
      try {
        final preview = await widget.repository.previewGrant(
          _request(app, ask, target),
        );
        if (mounted) setState(() => _previews[i] = preview);
      } on Object catch (e) {
        if (mounted) {
          setState(() => _previewFailures[i] = _failure(e) ?? '');
        }
      }
    }
  }

  GrantRequest _request(AppRecord app, AppGrantAsk ask, AdminTarget target) =>
      GrantRequest(
        subjectKind: AdminSubjectKind.app,
        subjectId: app.id,
        target: target,
        level: ask.level,
        capabilities: ask.capabilities,
        reason: _reason.text.trim(),
      );

  /// Runs the steps that did not finish yet, in order: each grant, then the token. The first
  /// failure stops; a cancelled step-up stops without a failure.
  Future<void> _createRest() async {
    final l = AppLocalizations.of(context);
    final app = _app!;
    final name = _manifest.name;
    setState(() => _busy = true);
    try {
      for (final (i, ask, target) in _chosen) {
        if (_grantSteps[i] is _Finished) continue;
        try {
          await widget.repository.createGrant(
            _request(app, ask, target),
            purpose: l.adminAppPurposeGrant(name, targetLabel(l, target)),
          );
          if (!mounted) return;
          setState(() => _grantSteps[i] = const _Finished());
        } on Object catch (e) {
          final failure = _failure(e);
          if (mounted && failure != null) {
            setState(() => _grantSteps[i] = _Failed(failure));
          }
          return;
        }
      }
      if (!mounted) return;
      final dpop = _dpop.text.trim();
      // A local only: no field of this state ever holds the secret.
      final String secret;
      try {
        final answer = await widget.repository.createToken(
          app.id,
          name: _tokenName.text.trim(),
          expiresAt: _expiresAt(context)!,
          dpopJkt: dpop.isEmpty ? null : dpop,
          purpose: l.adminAppPurposeToken(name),
        );
        secret = answer.secret;
      } on Object catch (e) {
        final failure = _failure(e);
        if (mounted && failure != null) {
          setState(() => _token = _Failed(failure));
        }
        return;
      }
      if (!mounted) return;
      setState(() => _token = const _Finished());
      String? kid;
      try {
        final keys = await widget.repository.webhookKeys();
        kid = keys.where((k) => k.state == 'active').firstOrNull?.kid;
      } on Object {
        // The token is shown in any case: it exists only in this answer. The key ID is also
        // on the app page with the subscriptions.
        kid = null;
      }
      if (!mounted) return;
      await OnceOnlySecret.show(
        context,
        title: l.adminAppTokenSecretTitle(name),
        label: l.adminTokenSecretLabel,
        notice: l.adminAppTokenSecretOnce,
        secret: secret,
        keyPrefix: 'appToken',
        details: [
          if (kid != null)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(
                l.adminAppKeyId(kid),
                key: const ValueKey('appToken.keyId'),
              ),
            ),
        ],
      );
      if (mounted) Navigator.of(context).pop(true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _uninstall() async {
    final l = AppLocalizations.of(context);
    final app = _app!;
    setState(() {
      _busy = true;
      _uninstallFailure = null;
    });
    try {
      await widget.repository.uninstallApp(
        app.id,
        reason: _reason.text.trim(),
        purpose: l.adminAppPurposeUninstall(_manifest.name),
      );
      if (mounted) Navigator.of(context).pop(true);
    } on Object catch (e) {
      final failure = _failure(e);
      if (mounted && failure != null) {
        setState(() => _uninstallFailure = failure);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  bool get _failed =>
      _grantSteps.values.any((s) => s is _Failed) || _token is _Failed;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final installed = _app != null;
    return AlertDialog(
      key: const ValueKey('install.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        _stage == _Stage.grants
            ? l.adminAppGrantsStepTitle
            : l.adminAppInstallTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: switch (_stage) {
              _Stage.pick => _pickContent(context, l, colors),
              _Stage.consent => _consentContent(context, l, colors),
              _Stage.grants => _grantsContent(context, l, colors),
            },
          ),
        ),
      ),
      actions: [
        if (_stage == _Stage.grants && _failed) ...[
          TextButton(
            key: const ValueKey('install.uninstall'),
            onPressed: _busy ? null : _uninstall,
            child: Text(l.adminAppUninstall),
          ),
          FilledButton(
            key: const ValueKey('install.retry'),
            onPressed: _busy ? null : _createRest,
            child: Text(l.selfRetry),
          ),
        ] else ...[
          TextButton(
            key: const ValueKey('install.cancel'),
            onPressed: _busy
                ? null
                : () => Navigator.of(context).pop(installed ? true : null),
            child: Text(installed ? l.adminCodeClose : l.selfCancel),
          ),
          if (_stage == _Stage.consent)
            FilledButton(
              key: const ValueKey('install.submit'),
              onPressed: _canInstall(context) ? _installApp : null,
              child: Text(l.adminAppInstallSubmit),
            ),
          if (_stage == _Stage.grants)
            FilledButton(
              key: const ValueKey('install.createGrants'),
              onPressed: _busy || !_previewsReady ? null : _createRest,
              child: Text(l.adminAppCreateGrants),
            ),
        ],
      ],
    );
  }

  bool get _previewsReady => _chosen.every(
    (c) => _previews.containsKey(c.$1) || _previewFailures.containsKey(c.$1),
  );

  bool _canInstall(BuildContext context) =>
      !_busy &&
      _tokenName.text.trim().isNotEmpty &&
      _expiresAt(context) != null &&
      _reason.text.trim().isNotEmpty;

  List<Widget> _pickContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final danger = dsToneColors(context, DsTone.danger);
    return [
      Text(l.adminAppInstallPickBody, style: body),
      const SizedBox(height: DsSpace.s12),
      Align(
        alignment: AlignmentDirectional.centerStart,
        child: OutlinedButton(
          key: const ValueKey('install.pick'),
          onPressed: _busy ? null : _pick,
          child: Text(l.adminAppPick),
        ),
      ),
      if (_fileName case final name? when name.isNotEmpty) ...[
        const SizedBox(height: DsSpace.s8),
        Text(l.adminAppFileChosen(name), style: secondary),
      ],
      if (_busy) ...[
        const SizedBox(height: DsSpace.s8),
        Text(l.adminAppChecking, style: secondary),
      ],
      if (_fileProblem case final problem?) ...[
        const SizedBox(height: DsSpace.s8),
        DsBanner(
          key: const ValueKey('install.fileError'),
          tone: DsTone.danger,
          text: manifestFileProblemText(l, problem),
        ),
      ],
      if (_checkFailure case final failure?) ...[
        const SizedBox(height: DsSpace.s8),
        DsBanner(tone: DsTone.danger, text: failure),
      ],
      if (_errors case final errors?) ...[
        const SizedBox(height: DsSpace.s8),
        Container(
          key: const ValueKey('install.errors'),
          padding: const EdgeInsets.all(DsSpace.s8),
          decoration: BoxDecoration(
            color: danger.subtle,
            border: Border.all(
              color: danger.border,
              width: DsBorderWidth.hairline,
            ),
            borderRadius: BorderRadius.circular(DsRadius.md),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l.adminAppInvalid,
                style: dsTextStyle(DsType.bodyStrong)
                    .copyWith(color: danger.fg),
              ),
              for (final e in errors) ...[
                const SizedBox(height: DsSpace.s4),
                Text(
                  l.adminAppErrorLine(
                    e.pointer.isEmpty ? l.adminAppManifestRoot : e.pointer,
                    e.code,
                  ),
                  style: dsTextStyle(DsType.bodySm).copyWith(color: danger.fg),
                ),
                // The server's short English detail, unchanged (D-M2-24-3).
                if (e.detail case final detail? when detail.isNotEmpty)
                  Text(
                    detail,
                    locale: const Locale('en'),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: danger.fg),
                  ),
              ],
            ],
          ),
        ),
      ],
    ];
  }

  List<Widget> _consentContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final heading = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final dayText = _lastDay.text.trim();
    final validDay = dayText.isEmpty || _expiresAt(context) != null;
    return [
      ConsentView(
        manifest: _manifest,
        scopes: _scopes,
        events: _events,
        enabled: !_busy,
        onScope: (s, on) =>
            setState(() => on ? _scopes.add(s) : _scopes.remove(s)),
        onEvent: (e, on) =>
            setState(() => on ? _events.add(e) : _events.remove(e)),
        grantTarget: (i, ask) => _targetPicker(l, i, ask),
      ),
      const SizedBox(height: DsSpace.s16),
      Semantics(
        header: true,
        child: Text(l.adminAppConsentToken, style: heading),
      ),
      Text(l.adminAppTokenBody, style: secondary),
      TextField(
        key: const ValueKey('consent.tokenName'),
        controller: _tokenName,
        enabled: !_busy,
        decoration: InputDecoration(labelText: l.adminTokenName),
      ),
      TextField(
        key: const ValueKey('consent.tokenLastDay'),
        controller: _lastDay,
        enabled: !_busy,
        keyboardType: TextInputType.datetime,
        decoration: InputDecoration(
          labelText: l.adminTokenLastDay,
          errorText: validDay ? null : l.adminTokenLastDayInvalid,
        ),
      ),
      TextField(
        key: const ValueKey('consent.tokenDpop'),
        controller: _dpop,
        enabled: !_busy,
        decoration: InputDecoration(labelText: l.adminAppTokenDpop),
      ),
      const SizedBox(height: DsSpace.s8),
      TextField(
        key: const ValueKey('consent.reason'),
        controller: _reason,
        enabled: !_busy,
        decoration: InputDecoration(labelText: l.adminReasonRequired),
      ),
      if (_install is _Failed) ...[
        const SizedBox(height: DsSpace.s12),
        _stepLine(
          context,
          l,
          'install.step.install',
          l.adminAppStepInstall(_manifest.name),
          _install,
        ),
      ],
    ];
  }

  /// The target picker of a space or channel grant: the server's targets of that kind and "Do
  /// not create"; null for an instance grant (no picker) or a kind this version cannot pick.
  Widget? _targetPicker(AppLocalizations l, int index, AppGrantAsk ask) {
    final kind = ask.targetKind;
    if (kind != AdminTargetKind.space && kind != AdminTargetKind.channel) {
      return null;
    }
    const none = 'none';
    final options = [
      for (final t in _targets)
        if (t.kind == kind) t,
    ];
    return Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Row(
        children: [
          Text('${l.adminAppGrantTarget}: '),
          Flexible(
            child: DropdownButton<String>(
              key: ValueKey('consent.grant.$index'),
              isExpanded: true,
              value: _picked[index]?.key ?? none,
              onChanged: _busy
                  ? null
                  : (key) => setState(() {
                      final t = options.where((o) => o.key == key).firstOrNull;
                      if (t == null) {
                        _picked.remove(index);
                      } else {
                        _picked[index] = t;
                      }
                    }),
              items: [
                DropdownMenuItem(
                  key: ValueKey('consent.grant.$index.$none'),
                  value: none,
                  child: Text(l.adminAppGrantNone),
                ),
                for (final t in options)
                  DropdownMenuItem(
                    key: ValueKey('consent.grant.$index.${t.key}'),
                    value: t.key,
                    child: Text(targetLabel(l, t)),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  List<Widget> _grantsContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final m = _manifest;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final heading = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final chosen = _chosen;
    final names = AccessNames(targets: _targets);
    return [
      Text(l.adminAppGrantsStepBody(m.name), style: body),
      const SizedBox(height: DsSpace.s12),
      if (chosen.isEmpty) Text(l.adminAppNoGrants, style: body),
      for (final (i, _, target) in chosen)
        Padding(
          key: ValueKey('install.preview.$i'),
          padding: const EdgeInsets.only(bottom: DsSpace.s12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(targetLabel(l, target), style: heading),
              const SizedBox(height: DsSpace.s4),
              if (_previewFailures[i] case final failure?)
                DsBanner(tone: DsTone.warning, text: failure)
              else
                PreviewPanel(preview: _previews[i], names: names),
            ],
          ),
        ),
      const SizedBox(height: DsSpace.s8),
      _stepLine(
        context,
        l,
        'install.step.install',
        l.adminAppStepInstall(m.name),
        _install,
      ),
      for (final (i, _, target) in chosen)
        _stepLine(
          context,
          l,
          'install.step.grant.$i',
          l.adminAppStepGrant(targetLabel(l, target)),
          _grantSteps[i] ?? const _Waiting(),
        ),
      _stepLine(
        context,
        l,
        'install.step.token',
        l.adminAppStepToken(_tokenName.text.trim()),
        _token,
      ),
      if (_uninstallFailure case final failure?) ...[
        const SizedBox(height: DsSpace.s8),
        DsBanner(tone: DsTone.danger, text: failure),
      ],
    ];
  }

  Widget _stepLine(
    BuildContext context,
    AppLocalizations l,
    String key,
    String step,
    _Step status,
  ) {
    final colors = context.dsColors;
    final (text, color) = switch (status) {
      _Waiting() => (l.adminAppStepWaiting, colors.textSecondary),
      _Finished() => (l.adminAppStepFinished, colors.textPrimary),
      _Failed(:final reason) => (
        l.adminAppStepFailed(reason),
        dsToneColors(context, DsTone.danger).fg,
      ),
    };
    return Padding(
      key: ValueKey(key),
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Semantics(
        liveRegion: status is _Failed,
        child: Text(
          l.adminAppStepLine(step, text),
          style: dsTextStyle(DsType.bodySm).copyWith(color: color),
        ),
      ),
    );
  }
}

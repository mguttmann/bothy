// SPDX-License-Identifier: Apache-2.0

/// The dialog that creates an incoming webhook in one channel, with an existing app or, for
/// owners, the composed "incoming app" (11 8.1; brief M6-08 Design note 5, Ruling 5).
library;

import 'dart:math';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import '../self/self_feedback.dart';

/// Where one step of the "incoming app" stands.
enum _Step { waiting, done, failed }

/// The dialog "Create incoming webhook" for [channel]: the name and the app it writes as, an
/// existing active app from [apps] or, when [offerIncomingApp] (the server said the caller is
/// an owner), a "New incoming app".
///
/// "Create" is the one confirmation. With an existing app it runs `createIncomingWebhook`. A
/// new incoming app has no API of its own (M2-27), so the dialog composes it in this order:
/// `installApp` with the built-in manifest (slug `incoming-<8 characters [a-z0-9]>`, only
/// `messages:write`, no events, no hosts), `createGrant` with `write` on exactly [channel] for
/// that app, and `createIncomingWebhook` with its ID; every call carries the same purpose, so
/// the step-up the server asks for the first one covers the others. A failed step stops: the
/// progress names what succeeded and the server's reason, "Retry" runs the failed step and the
/// rest, and "Uninstall the incoming app" removes a half-installed app; nothing is retried
/// silently.
///
/// Pops the answer of `createIncomingWebhook`, whose address holds the token: the dialog keeps
/// no reference to it, the caller shows it once in `OnceOnlySecret`. [onChanged] runs after
/// the install and after an uninstall, which change the server before the dialog closes.
///
/// Keys: `incoming.dialog`, `incoming.name`, `incoming.mode.existing`, `incoming.mode.newApp`,
/// `incoming.app` with items `incoming.app.<appId>`, `incoming.reason`, `incoming.submit`,
/// `incoming.step.install`, `incoming.step.grant`, `incoming.step.webhook`, `incoming.retry`,
/// `incoming.uninstall`.
///
/// Example:
/// ```dart
/// final created = await CreateIncomingDialog.show(
///   context,
///   repository: repository,
///   channel: channel,
///   apps: activeApps,
///   offerIncomingApp: await repository.callerIsOwner(),
/// );
/// ```
class CreateIncomingDialog extends StatefulWidget {
  /// Creates the dialog for [channel].
  const CreateIncomingDialog({
    super.key,
    required this.repository,
    required this.channel,
    required this.apps,
    required this.offerIncomingApp,
    this.newSlugSuffix,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The channel the webhook writes into.
  final AdminTarget channel;

  /// The active apps a webhook may write as.
  final List<AppRecord> apps;

  /// Whether "New incoming app" is offered (only owners may install, 11 8.1).
  final bool offerIncomingApp;

  /// The random suffix of the slug of an incoming app, or null for eight characters of
  /// `[a-z0-9]` from a secure random source.
  final String Function()? newSlugSuffix;

  /// Runs after the dialog changed the server before closing, or null.
  final VoidCallback? onChanged;

  /// Shows the dialog over [context]; returns the created webhook with its address, or null.
  static Future<IncomingWebhookCreatedInfo?> show(
    BuildContext context, {
    required AdminRepository repository,
    required AdminTarget channel,
    required List<AppRecord> apps,
    required bool offerIncomingApp,
    String Function()? newSlugSuffix,
    VoidCallback? onChanged,
  }) => showDialog<IncomingWebhookCreatedInfo>(
    context: context,
    barrierDismissible: false,
    builder: (_) => CreateIncomingDialog(
      repository: repository,
      channel: channel,
      apps: apps,
      offerIncomingApp: offerIncomingApp,
      newSlugSuffix: newSlugSuffix,
      onChanged: onChanged,
    ),
  );

  @override
  State<CreateIncomingDialog> createState() => _CreateIncomingDialogState();
}

class _CreateIncomingDialogState extends State<CreateIncomingDialog> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _reason = TextEditingController();
  late bool _newApp = widget.apps.isEmpty && widget.offerIncomingApp;
  String? _appId;
  late String _suffix = _newSuffix();

  // The progress of the composed flow; the app ID is set once the install succeeded.
  bool _started = false;
  bool _busy = false;
  String? _installedAppId;
  _Step _install = _Step.waiting;
  _Step _grant = _Step.waiting;
  _Step _webhook = _Step.waiting;
  String? _failure;
  SelfNotice? _notice;

  @override
  void initState() {
    super.initState();
    for (final c in [_name, _reason]) {
      c.addListener(() => setState(() {}));
    }
  }

  @override
  void dispose() {
    _name.dispose();
    _reason.dispose();
    super.dispose();
  }

  String _newSuffix() {
    final fixed = widget.newSlugSuffix;
    if (fixed != null) return fixed();
    const alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789';
    final random = Random.secure();
    return String.fromCharCodes([
      for (var i = 0; i < 8; i++)
        alphabet.codeUnitAt(random.nextInt(alphabet.length)),
    ]);
  }

  /// The built-in manifest of Ruling 5: the minimum an incoming webhook needs.
  Map<String, Object?> _manifest(String name) => {
    'schema_version': 1,
    'slug': 'incoming-$_suffix',
    'name': name,
    'description': 'Incoming webhook $name',
    'publisher': 'Built-in',
    'version': '1.0.0',
    'min_api_version': '0.2',
    'scopes': {
      'required': ['messages:write'],
      'optional': <String>[],
    },
  };

  bool get _canSubmit {
    if (_busy || _started || _name.text.trim().isEmpty) return false;
    return _newApp ? _reason.text.trim().isNotEmpty : _appId != null;
  }

  /// Runs [call] as the step [mark] and returns whether it succeeded; a failure stores the
  /// server's reason (nothing for a cancelled step-up, which leaves the step waiting).
  Future<bool> _step(
    void Function(_Step) mark,
    Future<void> Function() call,
  ) async {
    final l = AppLocalizations.of(context);
    try {
      await call();
      if (mounted) setState(() => mark(_Step.done));
      return true;
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted) {
        setState(() {
          _failure = failure?.text;
          mark(failure == null ? _Step.waiting : _Step.failed);
        });
      }
      return false;
    }
  }

  Future<void> _run() async {
    final l = AppLocalizations.of(context);
    final name = _name.text.trim();
    final reason = _reason.text.trim();
    final purpose = l.adminIncomingPurposeCreate(widget.channel.path);
    setState(() {
      _busy = true;
      _started = true;
      _failure = null;
      _notice = null;
    });
    try {
      final String appId;
      if (_newApp) {
        if (_installedAppId == null &&
            !await _step((s) => _install = s, () async {
              final app = await widget.repository.installApp(
                ManifestUpload.ofObject(_manifest(name)),
                grantedScopes: const ['messages:write'],
                eventSubscriptions: const [],
                reason: reason,
                purpose: purpose,
              );
              _installedAppId = app.id;
              widget.onChanged?.call();
            })) {
          return;
        }
        appId = _installedAppId!;
        if (_grant != _Step.done &&
            !await _step((s) => _grant = s, () async {
              await widget.repository.createGrant(
                GrantRequest(
                  subjectKind: AdminSubjectKind.app,
                  subjectId: appId,
                  target: widget.channel,
                  level: AccessLevel.write,
                  reason: reason,
                ),
                purpose: purpose,
              );
            })) {
          return;
        }
      } else {
        appId = _appId!;
      }
      IncomingWebhookCreatedInfo? created;
      if (!await _step((s) => _webhook = s, () async {
        created = await widget.repository.createIncomingWebhook(
          widget.channel.id!,
          appId: appId,
          name: name,
          purpose: purpose,
        );
      })) {
        // With an existing app nothing else ran, so the form opens again for a correction.
        if (!_newApp && mounted) setState(() => _started = false);
        return;
      }
      // Popped at once: the address with the token never lands in a field of this state.
      if (mounted) Navigator.of(context).pop(created);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _uninstall() async {
    final l = AppLocalizations.of(context);
    final appId = _installedAppId;
    if (appId == null) return;
    setState(() => _busy = true);
    try {
      await widget.repository.uninstallApp(
        appId,
        reason: _reason.text.trim(),
        purpose: l.adminIncomingPurposeUninstall(_name.text.trim()),
      );
      widget.onChanged?.call();
      if (!mounted) return;
      setState(() {
        _installedAppId = null;
        _started = false;
        _install = _grant = _webhook = _Step.waiting;
        _failure = null;
        // A fresh slug: the uninstalled app keeps its own.
        _suffix = _newSuffix();
        _notice = SelfNotice(l.adminIncomingUninstalled);
      });
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted && failure != null) {
        setState(() => _notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String _stepText(AppLocalizations l, String label, _Step step) =>
      l.adminIncomingStepLine(label, switch (step) {
        _Step.done => l.adminIncomingStepDone,
        _Step.waiting => l.adminIncomingStepWaiting,
        _Step.failed => _failure ?? l.adminIncomingStepWaiting,
      });

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    final locked = _busy || _started;
    // A back action of the system or the browser must not close the dialog while a run
    // is in progress: the address it returns is shown once.
    return PopScope(
      canPop: !_busy,
      child: AlertDialog(
        key: const ValueKey('incoming.dialog'),
        backgroundColor: colors.surfaceRaised,
        title: Text(
          l.adminIncomingDialogTitle(widget.channel.path),
          style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
        ),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(l.adminIncomingDialogBody, style: body),
                if (_notice case final n?) ...[
                  const SizedBox(height: DsSpace.s8),
                  selfNoticeBanner(n),
                ],
                const SizedBox(height: DsSpace.s16),
                TextField(
                  key: const ValueKey('incoming.name'),
                  controller: _name,
                  enabled: !locked,
                  decoration: InputDecoration(labelText: l.adminIncomingName),
                ),
                const SizedBox(height: DsSpace.s16),
                Wrap(
                  spacing: DsSpace.s8,
                  children: [
                    ChoiceChip(
                      key: const ValueKey('incoming.mode.existing'),
                      label: Text(l.adminIncomingModeExisting),
                      selected: !_newApp,
                      onSelected: locked
                          ? null
                          : (_) => setState(() => _newApp = false),
                    ),
                    if (widget.offerIncomingApp)
                      ChoiceChip(
                        key: const ValueKey('incoming.mode.newApp'),
                        label: Text(l.adminIncomingModeNewApp),
                        selected: _newApp,
                        onSelected: locked
                            ? null
                            : (_) => setState(() => _newApp = true),
                      ),
                  ],
                ),
                const SizedBox(height: DsSpace.s8),
                if (_newApp) ...[
                  Text(l.adminIncomingNewAppBody, style: body),
                  TextField(
                    key: const ValueKey('incoming.reason'),
                    controller: _reason,
                    enabled: !locked,
                    decoration: InputDecoration(labelText: l.adminReason),
                  ),
                ] else if (widget.apps.isEmpty)
                  Text(l.adminIncomingNoApps, style: body)
                else
                  DropdownButton<String>(
                    key: const ValueKey('incoming.app'),
                    isExpanded: true,
                    value: _appId,
                    hint: Text(l.adminIncomingApp),
                    onChanged: locked
                        ? null
                        : (v) => setState(() => _appId = v),
                    items: [
                      for (final a in widget.apps)
                        DropdownMenuItem(
                          key: ValueKey('incoming.app.${a.id}'),
                          value: a.id,
                          child: Text(a.displayName),
                        ),
                    ],
                  ),
                if (_started && _newApp) ...[
                  const SizedBox(height: DsSpace.s16),
                  for (final (key, label, step) in [
                    ('install', l.adminIncomingStepInstall, _install),
                    (
                      'grant',
                      l.adminIncomingStepGrant(widget.channel.path),
                      _grant,
                    ),
                    ('webhook', l.adminIncomingStepWebhook, _webhook),
                  ])
                    Text(
                      _stepText(l, label, step),
                      key: ValueKey('incoming.step.$key'),
                      style: dsTextStyle(DsType.body).copyWith(
                        color: step == _Step.failed
                            ? colors.dangerFg
                            : colors.textPrimary,
                      ),
                    ),
                ] else if (_webhook == _Step.failed && _failure != null) ...[
                  const SizedBox(height: DsSpace.s8),
                  DsBanner(
                    tone: DsTone.danger,
                    text: _failure!,
                    liveRegion: true,
                  ),
                ],
              ],
            ),
          ),
        ),
        actions: [
          if (_started && _newApp && !_busy && _installedAppId != null)
            TextButton(
              key: const ValueKey('incoming.uninstall'),
              onPressed: _uninstall,
              child: Text(l.adminIncomingUninstall),
            ),
          TextButton(
            key: const ValueKey('incoming.cancel'),
            onPressed: _busy ? null : () => Navigator.of(context).pop(),
            child: Text(l.selfCancel),
          ),
          // A run that stopped (a failure, or a cancelled step-up) continues where it stopped.
          if (_started && _newApp && !_busy)
            FilledButton(
              key: const ValueKey('incoming.retry'),
              onPressed: _run,
              child: Text(l.adminIncomingRetry),
            )
          else
            FilledButton(
              key: const ValueKey('incoming.submit'),
              onPressed: _canSubmit ? _run : null,
              child: Text(l.adminIncomingSubmit),
            ),
        ],
      ),
    );
  }
}

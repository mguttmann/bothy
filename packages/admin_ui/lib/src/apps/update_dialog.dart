// SPDX-License-Identifier: Apache-2.0

/// The update of an app with a new manifest file: the diff, `upgradeAppManifest` and the new
/// consent (11 4.2; brief M6-08 Design note 2, Ruling 2).
library;

import 'package:api_client/api_client.dart' show ProblemFieldError;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import 'app_labels.dart';
import 'consent_view.dart';
import 'manifest_picker.dart';

/// The added and removed entries of one part of a manifest.
final class ManifestDiffPart {
  /// Creates the part.
  const ManifestDiffPart(this.added, this.removed);

  /// Of the entries [before] and [after] (each once, in their order): what [after] adds and
  /// what it drops.
  factory ManifestDiffPart.of(Iterable<String> before, Iterable<String> after) {
    final b = before.toSet();
    final a = after.toSet();
    return ManifestDiffPart(
      [
        for (final x in a)
          if (!b.contains(x)) x,
      ],
      [
        for (final x in b)
          if (!a.contains(x)) x,
      ],
    );
  }

  /// The entries the new manifest adds.
  final List<String> added;

  /// The entries the new manifest drops.
  final List<String> removed;

  /// Whether nothing changes.
  bool get isEmpty => added.isEmpty && removed.isEmpty;
}

/// The diff of two manifests of one app (11 4.2): scopes (required and optional together),
/// events, destinations of outgoing data, requested grants (by kind, level and capabilities)
/// and commands (by name).
///
/// Example: `ManifestDiff.of(app.manifest, upload.info).scopes.added`.
final class ManifestDiff {
  /// Creates the diff.
  const ManifestDiff({
    required this.scopes,
    required this.events,
    required this.hosts,
    required this.grants,
    required this.commands,
  });

  /// The diff of [before] and [after].
  factory ManifestDiff.of(AppManifestInfo before, AppManifestInfo after) {
    List<String> scopes(AppManifestInfo m) => [
      ...m.requiredScopes,
      ...m.optionalScopes,
    ];
    return ManifestDiff(
      scopes: ManifestDiffPart.of(scopes(before), scopes(after)),
      events: ManifestDiffPart.of(before.events, after.events),
      hosts: ManifestDiffPart.of(before.targetHosts, after.targetHosts),
      grants: ManifestDiffPart.of(
        before.requestedGrants.map((g) => g.key),
        after.requestedGrants.map((g) => g.key),
      ),
      commands: ManifestDiffPart.of(
        before.commands.map((c) => c.name),
        after.commands.map((c) => c.name),
      ),
    );
  }

  /// The scopes.
  final ManifestDiffPart scopes;

  /// The events.
  final ManifestDiffPart events;

  /// The destinations (hosts of the delivery addresses).
  final ManifestDiffPart hosts;

  /// The requested grants by [AppGrantAsk.key].
  final ManifestDiffPart grants;

  /// The commands by name.
  final ManifestDiffPart commands;

  /// Whether nothing of the five parts changes.
  bool get isEmpty =>
      scopes.isEmpty &&
      events.isEmpty &&
      hosts.isEmpty &&
      grants.isEmpty &&
      commands.isEmpty;

  /// Whether the new manifest asks for scopes or events the old one did not: they need the new
  /// consent (`app.scopes.update`).
  bool get needsConsent => scopes.added.isNotEmpty || events.added.isNotEmpty;

  /// Whether anything is removed: it takes effect at once.
  bool get removesAny =>
      scopes.removed.isNotEmpty ||
      events.removed.isNotEmpty ||
      hosts.removed.isNotEmpty ||
      grants.removed.isNotEmpty ||
      commands.removed.isNotEmpty;
}

enum _Stage { pick, diff, consent }

/// The update of [app]: the owner picks the new manifest file, the server validates it (a
/// refusal shows each failed rule), and the diff ([ManifestDiff]) says what is added and what
/// is removed, that removed items take effect at once, that new destinations are confirmed by
/// this update and that an update never adds a grant (added grant requests are created under
/// Access grants).
///
/// "Update" runs `upgradeAppManifest` with the granted scopes and the event subscriptions the
/// new manifest still names (11 4.2: removed ones end at once, the app keeps the rest). When
/// the new manifest asks for more, the new consent follows ([ConsentView] with the keys
/// `update.consent.*`): required scopes fixed, added scopes and events checked, the rest as
/// granted; "Save consent" runs `setAppScopes`, "Later" keeps the current scopes. Every call
/// carries its purpose; the step-up is the server's.
///
/// Pops true when the server changed, null otherwise.
///
/// Keys: `update.dialog`, `update.pick`, `update.fileError`, `update.errors`, `update.diff`,
/// `update.reason`, `update.submit`, `update.failure`, `update.consentSubmit`,
/// `update.consentLater`, `update.cancel`.
///
/// Example: `UpdateAppDialog.show(context, repository: admin, app: app, pickManifest: pick)`.
class UpdateAppDialog extends StatefulWidget {
  /// Creates the dialog.
  const UpdateAppDialog({
    super.key,
    required this.repository,
    required this.app,
    required this.pickManifest,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The app with its current manifest, scopes and events.
  final AppRecord app;

  /// The host's file dialog for a manifest.
  final ManifestPicker pickManifest;

  /// Shows the dialog over [context]; returns true when the server changed.
  static Future<bool?> show(
    BuildContext context, {
    required AdminRepository repository,
    required AppRecord app,
    required ManifestPicker pickManifest,
  }) => showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => UpdateAppDialog(
      repository: repository,
      app: app,
      pickManifest: pickManifest,
    ),
  );

  @override
  State<UpdateAppDialog> createState() => _UpdateAppDialogState();
}

class _UpdateAppDialogState extends State<UpdateAppDialog> {
  _Stage _stage = _Stage.pick;
  bool _busy = false;
  bool _changed = false;

  ManifestFileProblem? _fileProblem;
  List<ProblemFieldError>? _errors;
  String? _failure;

  ManifestUpload? _upload;
  ManifestDiff? _diff;
  final Set<String> _scopes = {};
  final Set<String> _events = {};
  final TextEditingController _reason = TextEditingController();

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

  String? _failureOf(Object error) =>
      adminFailure(AppLocalizations.of(context), error)?.text;

  Future<void> _pick() async {
    final file = await widget.pickManifest();
    if (file == null || !mounted) return;
    setState(() {
      _fileProblem = null;
      _errors = null;
      _failure = null;
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
      setState(() {
        _upload = upload;
        _diff = ManifestDiff.of(widget.app.manifest, upload.info);
        _stage = _Stage.diff;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _failure = _failureOf(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _update() async {
    final l = AppLocalizations.of(context);
    final app = widget.app;
    final next = _upload!.info;
    final named = {...next.requiredScopes, ...next.optionalScopes};
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final updated = await widget.repository.upgradeManifest(
        app.id,
        _upload!,
        // 11 4.2: what the new manifest no longer names ends at once; the rest stays granted.
        grantedScopes: [
          for (final s in app.grantedScopes)
            if (named.contains(s)) s,
        ],
        eventSubscriptions: [
          for (final e in app.eventSubscriptions)
            if (next.events.contains(e)) e,
        ],
        reason: _reason.text.trim(),
        purpose: l.adminAppPurposeUpdate(app.displayName),
      );
      if (!mounted) return;
      _changed = true;
      final diff = _diff!;
      if (!diff.needsConsent) {
        Navigator.of(context).pop(true);
        return;
      }
      setState(() {
        // Added scopes and events start checked: the new manifest asks for them.
        _scopes
          ..clear()
          ..addAll(updated.grantedScopes)
          ..addAll(diff.scopes.added);
        _events
          ..clear()
          ..addAll(updated.eventSubscriptions)
          ..addAll(diff.events.added);
        _stage = _Stage.consent;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _failure = _failureOf(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _consent() async {
    final l = AppLocalizations.of(context);
    final next = _upload!.info;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      await widget.repository.setScopes(
        widget.app.id,
        grantedScopes: [
          ...next.requiredScopes,
          for (final s in next.optionalScopes)
            if (_scopes.contains(s)) s,
        ],
        eventSubscriptions: [
          for (final e in next.events)
            if (_events.contains(e)) e,
        ],
        reason: _reason.text.trim(),
        purpose: l.adminAppPurposeScopes(widget.app.displayName),
      );
      if (mounted) Navigator.of(context).pop(true);
    } on Object catch (e) {
      if (mounted) setState(() => _failure = _failureOf(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final app = widget.app;
    return AlertDialog(
      key: const ValueKey('update.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        _stage == _Stage.consent
            ? l.adminAppNewConsentTitle(app.displayName)
            : l.adminAppUpdateTitle(app.displayName),
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              ...switch (_stage) {
                _Stage.pick => _pickContent(context, l, colors),
                _Stage.diff => _diffContent(context, l, colors),
                _Stage.consent => _consentContent(context, l, colors),
              },
              if (_failure case final failure?) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(
                  key: const ValueKey('update.failure'),
                  tone: DsTone.danger,
                  text: failure,
                  liveRegion: true,
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        if (_stage == _Stage.consent) ...[
          TextButton(
            key: const ValueKey('update.consentLater'),
            onPressed: _busy ? null : () => Navigator.of(context).pop(true),
            child: Text(l.adminAppNewConsentLater),
          ),
          FilledButton(
            key: const ValueKey('update.consentSubmit'),
            onPressed: _busy ? null : _consent,
            child: Text(l.adminAppNewConsentSubmit),
          ),
        ] else ...[
          TextButton(
            key: const ValueKey('update.cancel'),
            onPressed: _busy
                ? null
                : () => Navigator.of(context).pop(_changed ? true : null),
            child: Text(l.selfCancel),
          ),
          if (_stage == _Stage.diff)
            FilledButton(
              key: const ValueKey('update.submit'),
              onPressed: _busy || _reason.text.trim().isEmpty ? null : _update,
              child: Text(l.adminAppUpdate),
            ),
        ],
      ],
    );
  }

  List<Widget> _pickContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final danger = dsToneColors(context, DsTone.danger);
    final dangerText = dsTextStyle(DsType.bodySm).copyWith(color: danger.fg);
    return [
      Text(
        l.adminAppUpdateBody(widget.app.slug),
        style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
      ),
      const SizedBox(height: DsSpace.s12),
      Align(
        alignment: AlignmentDirectional.centerStart,
        child: OutlinedButton(
          key: const ValueKey('update.pick'),
          onPressed: _busy ? null : _pick,
          child: Text(l.adminAppPick),
        ),
      ),
      if (_busy) ...[
        const SizedBox(height: DsSpace.s8),
        Text(
          l.adminAppChecking,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
      ],
      if (_fileProblem case final problem?) ...[
        const SizedBox(height: DsSpace.s8),
        DsBanner(
          key: const ValueKey('update.fileError'),
          tone: DsTone.danger,
          text: manifestFileProblemText(l, problem),
        ),
      ],
      if (_errors case final errors?) ...[
        const SizedBox(height: DsSpace.s8),
        Container(
          key: const ValueKey('update.errors'),
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
                  style: dangerText,
                ),
                // The server's short English detail, unchanged (D-M2-24-3).
                if (e.detail case final detail? when detail.isNotEmpty)
                  Text(detail, locale: const Locale('en'), style: dangerText),
              ],
            ],
          ),
        ),
      ],
    ];
  }

  List<Widget> _diffContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final diff = _diff!;
    final next = _upload!.info;
    final old = widget.app.manifest;
    final heading = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final grantLabels = {
      for (final g in [...old.requestedGrants, ...next.requestedGrants])
        g.key: appGrantAskLabel(l, g),
    };

    Widget part(
      String title,
      ManifestDiffPart p,
      String Function(String) show,
    ) {
      if (p.isEmpty) return const SizedBox.shrink();
      return Padding(
        padding: const EdgeInsets.only(top: DsSpace.s12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(title, style: heading),
            if (p.added.isNotEmpty)
              Text(
                l.adminAppDiffAdded(p.added.map(show).join(', ')),
                style: body,
              ),
            if (p.removed.isNotEmpty)
              Text(
                l.adminAppDiffRemoved(p.removed.map(show).join(', ')),
                style: body,
              ),
          ],
        ),
      );
    }

    return [
      Column(
        key: const ValueKey('update.diff'),
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            header: true,
            child: Text(
              l.adminAppDiffTitle(widget.app.manifestVersion, next.version),
              style: heading,
            ),
          ),
          if (diff.isEmpty)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(l.adminAppDiffNone, style: body),
            ),
          part(l.adminAppDiffScopes, diff.scopes, (s) => s),
          part(l.adminAppDiffEvents, diff.events, (e) => e),
          part(l.adminAppConsentDestinations, diff.hosts, (h) => h),
          part(l.adminAppDiffGrants, diff.grants, (k) => grantLabels[k] ?? k),
          part(l.adminAppConsentCommands, diff.commands, (c) => '/$c'),
          if (diff.removesAny) ...[
            const SizedBox(height: DsSpace.s12),
            DsBanner(tone: DsTone.warning, text: l.adminAppDiffRemovedNow),
          ],
          if (diff.hosts.added.isNotEmpty) ...[
            const SizedBox(height: DsSpace.s8),
            DsBanner(tone: DsTone.warning, text: l.adminAppDiffHostsConfirmed),
          ],
          if (diff.grants.added.isNotEmpty) ...[
            const SizedBox(height: DsSpace.s8),
            DsBanner(tone: DsTone.info, text: l.adminAppDiffGrantsLater),
          ],
          if (diff.needsConsent) ...[
            const SizedBox(height: DsSpace.s8),
            DsBanner(tone: DsTone.info, text: l.adminAppDiffNewConsent),
          ],
        ],
      ),
      const SizedBox(height: DsSpace.s12),
      TextField(
        key: const ValueKey('update.reason'),
        controller: _reason,
        enabled: !_busy,
        decoration: InputDecoration(labelText: l.adminReasonRequired),
      ),
    ];
  }

  List<Widget> _consentContent(
    BuildContext context,
    AppLocalizations l,
    DsColors colors,
  ) {
    final next = _upload!.info;
    return [
      Text(
        l.adminAppNewConsentBody(next.version),
        style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
      ),
      ConsentView(
        manifest: next,
        scopes: _scopes,
        events: _events,
        choicesOnly: true,
        enabled: !_busy,
        keyPrefix: 'update.consent',
        onScope: (s, on) =>
            setState(() => on ? _scopes.add(s) : _scopes.remove(s)),
        onEvent: (e, on) =>
            setState(() => on ? _events.add(e) : _events.remove(e)),
      ),
    ];
  }
}

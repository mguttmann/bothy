// SPDX-License-Identifier: Apache-2.0

/// The branding of the instance: product name, logo and accent (01 12; ADR 0044; brief M6-09
/// Design note 2; Ruling D-M6-09-2).
library;

import 'dart:typed_data';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/instance_repository.dart';
import '../self/self_feedback.dart';
import 'accent_picker.dart';
import 'instance_common.dart';
import 'setting_kinds.dart';

/// Product name, logo and accent of the instance.
///
/// The name field has no counter of its own: the limit `branding.product_name_max_chars` is a
/// server key the API does not deliver, so the server's `400` (too long, or a control or bidi
/// character, SEC-IN-4) stands at the field. The accent is chosen with [AccentPicker] from the
/// seven palette keys, previewed in light and dark ([AccentPreview]). "Save" states the change
/// and sends `updateBranding` with `If-Match` on the branding version; a `412` shows the newer
/// branding next to the input and asks again. The logo is uploaded through [pickLogo]
/// (`uploadBrandingLogo`, last write wins), previewed from the server's variant
/// (`getBrandingLogo`, `404` until the file is checked) and removed with `deleteBrandingLogo`.
/// Every change has step-up (05 `instance.branding.update`). [onBrandingChanged] runs after a
/// change, so the host can show the new branding (the console reads discovery again).
///
/// Keys: `branding.name`, `branding.save`, `branding.conflict`, `branding.error`,
/// `branding.logo.preview`, `branding.logo.pending`, `branding.logo.none`,
/// `branding.logo.refresh`, `branding.logo.upload`, `branding.logo.remove`.
class BrandingSection extends StatefulWidget {
  /// Creates the section.
  const BrandingSection({
    super.key,
    required this.repository,
    this.pickLogo,
    this.onBrandingChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host's file dialog for the logo, or null (no upload).
  final LogoPicker? pickLogo;

  /// Runs after a saved change of the branding or the logo.
  final VoidCallback? onBrandingChanged;

  @override
  State<BrandingSection> createState() => _BrandingSectionState();
}

class _BrandingSectionState extends State<BrandingSection> {
  BrandingState? _branding;
  Uint8List? _logo;
  Object? _error;
  final TextEditingController _name = TextEditingController();
  String? _accent;
  String? _nameError;
  String? _accentError;
  SelfNotice? _notice;
  BrandingState? _conflict;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final overview = await widget.repository.instanceOverview();
      final logo = overview.branding.logoFileId == null
          ? null
          : await widget.repository.logo();
      if (!mounted) return;
      setState(() {
        _branding = overview.branding;
        _logo = logo;
        _error = null;
        _name.text = overview.branding.productName;
        _accent = overview.branding.accentColor;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _refreshLogo() async {
    try {
      final logo = await widget.repository.logo();
      if (mounted) setState(() => _logo = logo);
    } on Object catch (e) {
      if (!mounted) return;
      final failure = selfFailure(AppLocalizations.of(context), e);
      if (failure != null) {
        setState(() => _notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    }
  }

  bool _changed(BrandingState b) =>
      _name.text != b.productName ||
      (_accent != null && _accent != b.accentColor);

  Future<void> _save() async {
    final l = AppLocalizations.of(context);
    final base = _branding!;
    final accent = _accent ?? base.accentColor ?? accentPaletteKeys.first;
    final lines = [
      if (_name.text != base.productName)
        l.instanceBrandingNameChange(base.productName, _name.text),
      if (accent != base.accentColor)
        l.instanceBrandingAccentChange(
          accentName(l, base.accentColor ?? ''),
          accentName(l, accent),
        ),
    ];
    final confirmed = await ConfirmChange.show(
      context,
      title: l.instanceBrandingConfirmTitle,
      body: [...lines, l.instanceBrandingEveryone].join('\n'),
      confirmLabel: l.instanceSave,
    );
    if (confirmed == null || !mounted) return;
    setState(() {
      _busy = true;
      _nameError = null;
      _accentError = null;
      _notice = null;
    });
    try {
      final saved = await widget.repository.updateBranding(
        productName: _name.text,
        accentColor: accent,
        version: base.version,
        purpose: l.stepUpPurposeBranding,
      );
      if (!mounted) return;
      setState(() {
        _branding = saved;
        _conflict = null;
        _name.text = saved.productName;
        _accent = saved.accentColor;
        _notice = SelfNotice(l.instanceBrandingSaved);
      });
      widget.onBrandingChanged?.call();
    } on InstanceVersionConflict<BrandingState> catch (c) {
      // The input stays; the next save names the newer version.
      if (mounted) {
        setState(() {
          _conflict = c.current;
          _branding = c.current;
        });
      }
    } on Object catch (e) {
      if (!mounted) return;
      final fields = fieldErrorsOf(e);
      final nameError = fieldErrorAt(l, fields, '/product_name');
      final accentError = fieldErrorAt(l, fields, '/accent_color');
      final failure = nameError == null && accentError == null
          ? selfFailure(l, e)
          : null;
      setState(() {
        _nameError = nameError;
        _accentError = accentError;
        if (failure != null) {
          _notice = SelfNotice(failure.text, tone: DsTone.danger);
        }
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _upload(LogoPicker pick) async {
    final l = AppLocalizations.of(context);
    final file = await pick();
    if (file == null || !mounted) return;
    final confirmed = await ConfirmChange.show(
      context,
      title: l.instanceLogoConfirmTitle,
      body: l.instanceLogoConfirmBody(file.name),
      confirmLabel: l.instanceLogoUpload,
    );
    if (confirmed == null || !mounted) return;
    await _run(() async {
      final saved = await widget.repository.uploadLogo(
        file,
        purpose: l.stepUpPurposeLogo,
      );
      final logo = await widget.repository.logo();
      if (!mounted) return;
      setState(() {
        _branding = saved;
        _logo = logo;
        _notice = SelfNotice(l.instanceLogoUploaded);
      });
      widget.onBrandingChanged?.call();
    });
  }

  Future<void> _remove() async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.instanceLogoRemoveTitle,
      body: l.instanceLogoRemoveBody,
      confirmLabel: l.instanceLogoRemove,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _run(() async {
      await widget.repository.removeLogo(purpose: l.stepUpPurposeLogoRemove);
      final overview = await widget.repository.instanceOverview();
      if (!mounted) return;
      setState(() {
        _branding = overview.branding;
        _logo = null;
        _notice = SelfNotice(l.instanceLogoRemoved);
      });
      widget.onBrandingChanged?.call();
    });
  }

  Future<void> _run(Future<void> Function() change) async {
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _notice = null;
    });
    try {
      await change();
    } on Object catch (e) {
      final failure = selfFailure(l, e);
      if (mounted && failure != null) {
        setState(() => _notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final branding = _branding;
    final error = _error;
    if (error != null) {
      return ProblemBanner(problem: loadProblem(error), onRetry: _load);
    }
    if (branding == null) {
      return const Center(child: CircularProgressIndicator());
    }
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final heading = dsTextStyle(DsType.heading)
        .copyWith(color: colors.textPrimary);
    final conflict = _conflict;
    final accent = _accent;
    final pick = widget.pickLogo;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_notice case final n?) selfNoticeBanner(n),
        if (conflict != null)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s12),
            child: DsBanner(
              key: const ValueKey('branding.conflict'),
              tone: DsTone.warning,
              text: l.instanceBrandingConflict(
                conflict.productName,
                accentName(l, conflict.accentColor ?? ''),
              ),
              liveRegion: true,
            ),
          ),
        Semantics(
          header: true,
          child: Text(l.instanceBrandingName, style: heading),
        ),
        const SizedBox(height: DsSpace.s8),
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 480),
          child: TextField(
            key: const ValueKey('branding.name'),
            controller: _name,
            enabled: !_busy,
            decoration: InputDecoration(
              labelText: l.instanceBrandingName,
              helperText: l.instanceBrandingNameHelp,
              helperMaxLines: 3,
              errorText: _nameError,
              errorMaxLines: 3,
            ),
            onChanged: (_) => setState(() => _nameError = null),
          ),
        ),
        const SizedBox(height: DsSpace.s24),
        Semantics(
          header: true,
          child: Text(l.instanceAccentLabel, style: heading),
        ),
        const SizedBox(height: DsSpace.s4),
        Text(l.instanceAccentHelp, style: secondary),
        const SizedBox(height: DsSpace.s8),
        AccentPicker(
          selected: accent,
          onChanged: _busy ? null : (k) => setState(() => _accent = k),
        ),
        if (_accentError case final e?)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              e,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.dangerFg),
            ),
          ),
        const SizedBox(height: DsSpace.s12),
        AccentPreview(
          accent: accent ?? accentPaletteKeys.first,
          productName: _name.text,
        ),
        const SizedBox(height: DsSpace.s12),
        Align(
          alignment: Alignment.centerLeft,
          child: FilledButton(
            key: const ValueKey('branding.save'),
            onPressed: _busy || !_changed(branding) ? null : _save,
            child: Text(l.instanceSave),
          ),
        ),
        const SizedBox(height: DsSpace.s24),
        Semantics(
          header: true,
          child: Text(l.instanceLogoTitle, style: heading),
        ),
        const SizedBox(height: DsSpace.s8),
        _logoView(context, branding),
        const SizedBox(height: DsSpace.s8),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('branding.logo.upload'),
              onPressed: _busy || pick == null ? null : () => _upload(pick),
              child: Text(l.instanceLogoUpload),
            ),
            if (branding.logoFileId != null)
              OutlinedButton(
                key: const ValueKey('branding.logo.remove'),
                onPressed: _busy ? null : _remove,
                child: Text(l.instanceLogoRemove),
              ),
          ],
        ),
      ],
    );
  }

  Widget _logoView(BuildContext context, BrandingState branding) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final logo = _logo;
    if (branding.logoFileId == null) {
      return Text(
        l.instanceLogoNone,
        key: const ValueKey('branding.logo.none'),
        style: secondary,
      );
    }
    if (logo == null) {
      return Wrap(
        key: const ValueKey('branding.logo.pending'),
        spacing: DsSpace.s8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Text(l.instanceLogoPending, style: secondary),
          TextButton(
            key: const ValueKey('branding.logo.refresh'),
            onPressed: _busy ? null : _refreshLogo,
            child: Text(l.instanceLogoRefresh),
          ),
        ],
      );
    }
    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        key: const ValueKey('branding.logo.preview'),
        constraints: const BoxConstraints(maxWidth: 160, maxHeight: 160),
        decoration: BoxDecoration(
          border: Border.all(color: colors.borderDefault),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        padding: const EdgeInsets.all(DsSpace.s8),
        child: Image.memory(
          logo,
          semanticLabel: l.instanceLogoTitle,
          fit: BoxFit.contain,
          errorBuilder: (context, _, _) =>
              Text(l.instanceLogoPending, style: secondary),
        ),
      ),
    );
  }
}

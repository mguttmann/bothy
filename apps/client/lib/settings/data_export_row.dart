// SPDX-License-Identifier: Apache-2.0

/// The row "Export of your data" of the settings section Privacy
/// (docs/spec/01-product.md 5.1 D8 "[Export]"; docs/spec/20-privacy.md 5.1;
/// docs/decisions.d/AUD-client-settings-01.md).
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart' show activityText;
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart'
    show AuthException, StepUpCancelled, StepUpUnavailable;
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart' show CancelToken;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../files/attachment_mapping.dart' show fileSizeLabel;
import 'data_export.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// The file name the save dialog suggests for the archive of [export], for example
/// `export-2026-11-12.zip` (the day of the request in UTC).
String dataExportFileName(DataExport export) {
  final d = export.requestedAt.toUtc();
  return 'export-${d.year.toString().padLeft(4, '0')}-'
      '${d.month.toString().padLeft(2, '0')}-'
      '${d.day.toString().padLeft(2, '0')}.zip';
}

/// Requests the export of the own data, tells where the request stands and saves the archive
/// of a completed one.
///
/// The row reads the newest export request when it appears and after every action; "Check
/// again" reads it once more while the request waits for an owner or for the job. A build
/// without a dialog to save a file (the phones) shows the state of the request and names the
/// desktop app for the download. Nothing shows before the login.
class DataExportRow extends ConsumerStatefulWidget {
  /// Creates the row.
  const DataExportRow({super.key});

  @override
  ConsumerState<DataExportRow> createState() => _DataExportRowState();
}

class _DataExportRowState extends ConsumerState<DataExportRow> {
  DataExport? _export;
  bool _loaded = false;
  bool _busy = false;
  bool _downloading = false;
  String? _error;
  String? _saved;
  CancelToken? _cancel;

  @override
  void initState() {
    super.initState();
    // The texts of a failure need the localizations, which the first frame brings.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) unawaited(_read());
    });
  }

  @override
  void dispose() {
    _cancel?.cancel();
    super.dispose();
  }

  static String _reason(AppLocalizations l, ApiProblem? problem) =>
      problem == null
      ? l.selfOffline
      : problemText(l, problem.code, reason: problem.reason);

  /// Runs [step] with the buttons off; [failed] names a refusal, a session that cannot
  /// confirm a step-up or a lost session. A cancelled step-up says nothing.
  Future<void> _run(
    Future<void> Function(DataExportPort port) step, {
    required String Function(String reason) failed,
  }) async {
    final port = ref.read(dataExportPortProvider);
    if (port == null || _busy) return;
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _error = null;
      _saved = null;
    });
    String? error;
    try {
      await step(port);
    } on DataExportRefused catch (e) {
      error = failed(_reason(l, e.problem));
    } on DataExportBrokeOff {
      error = l.exportBrokeOff;
    } on StepUpCancelled {
      // The person declined the confirmation: nothing was changed and nothing is to say.
    } on StepUpUnavailable {
      // A device without a step-up key and without a password (08 6.2): the reason, as the
      // device screens give it.
      error = failed(
        problemText(l, 'forbidden', reason: 'step_up_not_possible'),
      );
    } on AuthException {
      error = failed(l.problemSessionRevoked);
    }
    if (!mounted) return;
    setState(() {
      _busy = false;
      _downloading = false;
      _error = error;
    });
  }

  Future<void> _read() {
    final l = AppLocalizations.of(context);
    return _run((port) async {
      final export = await port.latest();
      if (!mounted) return;
      setState(() {
        _export = export;
        _loaded = true;
      });
    }, failed: l.settingsDataExportLoadFailed);
  }

  Future<void> _request() {
    final l = AppLocalizations.of(context);
    return _run((port) async {
      final export = await port.request(
        reason: l.settingsDataExportReason,
        purpose: l.stepUpPurposeDataExportRequest,
      );
      if (mounted) setState(() => _export = export);
    }, failed: l.settingsDataExportRequestFailed);
  }

  Future<void> _download(DataExport export) async {
    final l = AppLocalizations.of(context);
    final locale = Localizations.localeOf(context).toString();
    final path = await ref
        .read(appPortsProvider)
        .files
        .picker
        .saveLocation(dataExportFileName(export));
    if (path == null || !mounted) return;
    final cancel = _cancel = CancelToken();
    await _run((port) async {
      if (mounted) setState(() => _downloading = true);
      try {
        final bytes = await port.download(
          export,
          path: path,
          purpose: l.stepUpPurposeDataExportDownload,
          cancel: cancel,
        );
        if (mounted) {
          setState(
            () =>
                _saved = l.exportDone(fileSizeLabel(l, bytes, locale: locale)),
          );
        }
      } on DataExportRefused {
        // The archive may be gone meanwhile (`410 gone`): show where the request stands now.
        try {
          final now = await port.latest();
          if (mounted) setState(() => _export = now);
        } on Object {
          // The refusal below is what the person reads.
        }
        rethrow;
      }
    }, failed: l.settingsDataExportDownloadFailed);
    _cancel = null;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    // Another session (a login after this row appeared): what was read belongs to the one
    // before, so the row starts over.
    ref.listen(dataExportPortProvider, (_, port) {
      _cancel?.cancel();
      setState(() {
        _export = null;
        _loaded = false;
        _error = null;
        _saved = null;
      });
      if (port != null) unawaited(_read());
    });
    if (ref.watch(dataExportPortProvider) == null) {
      return const SizedBox.shrink();
    }
    final ports = ref.watch(appPortsProvider);
    final canSave = !ports.formFactor.isPhone;
    final zone = ref.watch(personTimeZoneProvider);
    String when(DateTime at) =>
        activityText(context, at, now: ports.clock.now(), timeZone: zone);
    final export = _export;
    final state = switch (export) {
      null => null,
      DataExport(phase: DataExportPhase.waiting) => l.settingsDataExportWaiting(
        when(export.requestedAt),
      ),
      DataExport(phase: DataExportPhase.preparing) =>
        l.settingsDataExportPreparing(when(export.requestedAt)),
      DataExport(phase: DataExportPhase.ready, :final availableUntil?) =>
        canSave
            ? l.settingsDataExportReady(when(availableUntil))
            : l.settingsDataExportReadyNoSave(when(availableUntil)),
      DataExport(phase: DataExportPhase.ready) => null,
      DataExport(phase: DataExportPhase.expired) => l.settingsDataExportExpired(
        when(export.requestedAt),
      ),
      DataExport(phase: DataExportPhase.rejected) =>
        l.settingsDataExportRejected(when(export.requestedAt)),
      DataExport(phase: DataExportPhase.cancelled) =>
        l.settingsDataExportCancelled(when(export.requestedAt)),
      DataExport(phase: DataExportPhase.failed) => l.settingsDataExportFailed(
        when(export.requestedAt),
      ),
    };
    final ready = export?.phase == DataExportPhase.ready;
    final open = export?.open ?? false;
    return SettingRow(
      label: l.settingsDataExport,
      hint: l.settingsDataExportHint,
      // The state, the buttons and the notices are nodes of their own for screen readers.
      mergeSemantics: false,
      control: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          key: const ValueKey('settings.dataExport'),
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_error case final error?)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s8),
                child: DsBanner(
                  key: const ValueKey('settings.dataExport.error'),
                  tone: DsTone.danger,
                  text: error,
                  liveRegion: true,
                ),
              ),
            if (_saved case final saved?)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s8),
                child: DsBanner(
                  key: const ValueKey('settings.dataExport.saved'),
                  tone: DsTone.info,
                  text: saved,
                  liveRegion: true,
                ),
              ),
            if (_downloading)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s8),
                child: Text(
                  l.settingsDataExportDownloading,
                  key: const ValueKey('settings.dataExport.downloading'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.textSecondary),
                ),
              )
            else if (state != null)
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s8),
                child: Text(
                  state,
                  key: const ValueKey('settings.dataExport.state'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
            // Before the first answer there is nothing to act on; after a failed first read
            // "Check again" reads once more.
            if (!_loaded)
              if (_error != null)
                OutlinedButton(
                  key: const ValueKey('settings.dataExport.check'),
                  onPressed: _busy ? null : _read,
                  child: Text(l.settingsDataExportCheck),
                )
              else
                const SizedBox.shrink()
            else if (open)
              OutlinedButton(
                key: const ValueKey('settings.dataExport.check'),
                onPressed: _busy ? null : _read,
                child: Text(l.settingsDataExportCheck),
              )
            else if (ready)
              if (canSave)
                OutlinedButton(
                  key: const ValueKey('settings.dataExport.download'),
                  onPressed: _busy ? null : () => _download(export!),
                  child: Text(l.settingsDataExportDownload),
                )
              else
                const SizedBox.shrink()
            else
              OutlinedButton(
                key: const ValueKey('settings.dataExport.request'),
                onPressed: _busy ? null : _request,
                child: Text(l.settingsDataExportRequest),
              ),
          ],
        ),
      ),
    );
  }
}

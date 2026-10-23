// SPDX-License-Identifier: Apache-2.0

/// The export of a channel's history (brief M4-33 Design note 3; 07 2.4; 05 `export.channel`;
/// openapi `exportChannel`): the range dialog, the save dialog of the platform, the stream
/// written unchanged into the chosen file with a byte progress, and cancel.
///
/// The step-up is the server's demand: the request carries its purpose text and the client's
/// `StepUpInterceptor` asks when the server answers `401 step_up_required` (15 1). Nothing of
/// the export stays in the app: no cache, no copy, and a cancelled or broken export deletes
/// its partial file (docs/DECISIONS.md D-M4-33-3).
library;

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart' show ApiClient, ApiProblem;
import 'package:auth/auth.dart' show AuthException, stepUpPurposeExtra;
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../files/attachment_mapping.dart' show fileSizeLabel;
import '../settings/settings_providers.dart';

/// How an export ended without a complete file.
sealed class ChannelExportFailure implements Exception {
  const ChannelExportFailure();
}

/// The person cancelled; the partial file is deleted.
final class ChannelExportCancelled extends ChannelExportFailure {
  /// Creates the failure.
  const ChannelExportCancelled();
}

/// The stream broke off after its first byte; the partial file is deleted.
final class ChannelExportBrokeOff extends ChannelExportFailure {
  /// Creates the failure.
  const ChannelExportBrokeOff();
}

/// The server refused the export (`400`, `403`, `404`, ...) with [problem].
final class ChannelExportRefused extends ChannelExportFailure {
  /// Creates the failure.
  const ChannelExportRefused(this.problem);

  /// The server's problem, or null without an answer.
  final ApiProblem? problem;
}

/// Streams `exportChannel` into a file.
///
/// Example:
/// ```dart
/// final bytes = await ChannelExporter(session.api).run(
///   channel: general,
///   from: from,
///   to: to,
///   path: '/tmp/general.ndjson',
///   purpose: l.stepUpPurposeChannelExport('general'),
///   onProgress: (n) => print(n),
/// );
/// ```
final class ChannelExporter {
  /// Creates the exporter over the client of the signed-in session (DPoP, refresh and the
  /// step-up interceptor apply).
  ChannelExporter(this._api);

  final ApiClient _api;

  /// The security requirement of `exportChannel` as the generated client sends it, so the
  /// auth interceptors sign this request too.
  static const Map<String, Object?> _secure = {
    'secure': <Map<String, String>>[
      {
        'type': 'apiKey',
        'name': 'consoleSession',
        'keyName': '__Host-console_session',
        'where': '',
      },
      {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
      {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
    ],
  };

  /// Writes the history of [channel] from [from] (inclusive) to [to] (exclusive) byte for
  /// byte into the file at [path] and returns the number of bytes; [onProgress] gets the
  /// bytes written so far. [cancel] aborts the stream.
  ///
  /// Throws [ChannelExportCancelled] after [cancel], [ChannelExportBrokeOff] when the stream
  /// ended with an error, [ChannelExportRefused] with the server's problem; an error of the
  /// login layer (the step-up cancelled, a lost session) passes as it is. In every failure
  /// the file at [path] is deleted.
  Future<int> run({
    required ChannelId channel,
    required DateTime from,
    required DateTime to,
    required String path,
    required String purpose,
    required void Function(int bytes) onProgress,
    CancelToken? cancel,
  }) async {
    final Response<ResponseBody> response;
    try {
      response = await _api.dio.get<ResponseBody>(
        '/channels/${channel.value}/export',
        queryParameters: {
          'from': from.toUtc().toIso8601String(),
          'to': to.toUtc().toIso8601String(),
        },
        options: Options(
          responseType: ResponseType.stream,
          extra: {..._secure, stepUpPurposeExtra: purpose},
        ),
        cancelToken: cancel,
      );
    } on DioException catch (e) {
      if (CancelToken.isCancel(e)) throw const ChannelExportCancelled();
      final cause = e.error;
      if (cause is AuthException) throw cause;
      throw ChannelExportRefused(ApiProblem.of(e));
    }
    final file = File(path);
    final sink = file.openWrite();
    var written = 0;
    try {
      await for (final chunk in response.data!.stream) {
        if (cancel?.isCancelled ?? false) throw const ChannelExportCancelled();
        sink.add(chunk);
        written += chunk.length;
        onProgress(written);
      }
      await sink.flush();
      await sink.close();
      if (cancel?.isCancelled ?? false) throw const ChannelExportCancelled();
      return written;
    } on Object catch (e) {
      await _discard(sink, file);
      if (e is ChannelExportCancelled || (cancel?.isCancelled ?? false)) {
        throw const ChannelExportCancelled();
      }
      throw const ChannelExportBrokeOff();
    }
  }

  static Future<void> _discard(IOSink sink, File file) async {
    try {
      await sink.close();
    } on Object {
      // The file is deleted next; a failed close changes nothing.
    }
    if (file.existsSync()) await file.delete();
  }
}

/// Opens the export dialog of [channel] (only reachable with the capability `export`).
Future<void> showChannelExport(BuildContext context, Channel channel) =>
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (_) => ChannelExportDialog(channel: channel),
    );

/// The file name of an export of [channel] for the days [from] to [to] (inclusive), for
/// example `general-2026-10-01-2026-10-20.ndjson`.
String channelExportFileName(Channel channel, DateTime from, DateTime to) {
  String day(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-'
      '${d.month.toString().padLeft(2, '0')}-'
      '${d.day.toString().padLeft(2, '0')}';
  final name = channel.name.replaceAll(RegExp(r'[^A-Za-z0-9._-]'), '_');
  return '$name-${day(from)}-${day(to)}.ndjson';
}

enum _Phase { range, running, ended }

/// The dialog: the first and the last day of the range in the person's time zone, then the
/// save dialog of the platform, then the progress with Cancel, then the result.
class ChannelExportDialog extends ConsumerStatefulWidget {
  /// Creates the dialog of [channel].
  const ChannelExportDialog({super.key, required this.channel});

  /// The channel.
  final Channel channel;

  @override
  ConsumerState<ChannelExportDialog> createState() =>
      _ChannelExportDialogState();
}

class _ChannelExportDialogState extends ConsumerState<ChannelExportDialog> {
  late DateTime _from;
  late DateTime _to;
  _Phase _phase = _Phase.range;
  int _bytes = 0;
  String? _notice;
  bool _noticeIsError = false;
  CancelToken? _cancel;

  @override
  void initState() {
    super.initState();
    final today = ref.read(appPortsProvider).clock.now();
    _to = DateTime(today.year, today.month, today.day);
    _from = _to.subtract(const Duration(days: 30));
  }

  Future<void> _pick({required bool first}) async {
    final now = ref.read(appPortsProvider).clock.now();
    final day = await showDatePicker(
      context: context,
      initialDate: first ? _from : _to,
      firstDate: DateTime(2000),
      lastDate: DateTime(now.year + 1, 12, 31),
    );
    if (day == null || !mounted) return;
    setState(() => first ? _from = day : _to = day);
  }

  Future<void> _start() async {
    final l = AppLocalizations.of(context);
    final locale = Localizations.localeOf(context);
    final services = ref.read(signedInServicesProvider);
    if (services == null) return;
    final path = await ref
        .read(appPortsProvider)
        .files
        .picker
        .saveLocation(channelExportFileName(widget.channel, _from, _to));
    if (path == null || !mounted) return;
    final zone = ref.read(personTimeZoneProvider);
    final format = L10nFormat(locale);
    final cancel = CancelToken();
    setState(() {
      _phase = _Phase.running;
      _bytes = 0;
      _notice = null;
      _cancel = cancel;
    });
    String? notice;
    var error = false;
    try {
      final bytes = await ChannelExporter(services.auth.api).run(
        channel: widget.channel.id,
        // The days are the person's: from the start of the first to the start of the day
        // after the last (the end is exclusive).
        from: format.dayStart(
          _from.year,
          _from.month,
          _from.day,
          timeZone: zone,
        ),
        to: format.dayStart(_to.year, _to.month, _to.day + 1, timeZone: zone),
        path: path,
        purpose: l.stepUpPurposeChannelExport(widget.channel.name),
        onProgress: (n) {
          if (mounted) setState(() => _bytes = n);
        },
        cancel: cancel,
      );
      notice = l.exportDone(fileSizeLabel(l, bytes, locale: locale.toString()));
    } on ChannelExportCancelled {
      notice = l.exportCancelled;
    } on ChannelExportBrokeOff {
      notice = l.exportBrokeOff;
      error = true;
    } on ChannelExportRefused catch (e) {
      final p = e.problem;
      notice = p == null
          ? l.exportBrokeOff
          : problemText(l, p.code, reason: p.reason);
      error = true;
    } on AuthException {
      // A cancelled step-up or a lost session: back to the range, nothing was written.
      if (mounted) setState(() => _phase = _Phase.range);
      return;
    }
    if (mounted) {
      setState(() {
        _phase = _Phase.ended;
        _notice = notice;
        _noticeIsError = error;
        _cancel = null;
      });
    }
  }

  @override
  void dispose() {
    _cancel?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final material = MaterialLocalizations.of(context);
    final locale = Localizations.localeOf(context).toString();
    final invalid = _to.isBefore(_from);
    Widget day(String label, DateTime value, {required bool first}) =>
        SettingDayRow(
          key: ValueKey(first ? 'export.from' : 'export.to'),
          label: label,
          value: material.formatMediumDate(value),
          onTap: _phase == _Phase.range ? () => _pick(first: first) : null,
        );
    return AlertDialog(
      key: const ValueKey('export.dialog'),
      title: Text(l.exportTitle(widget.channel.name)),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            day(l.exportFrom, _from, first: true),
            day(l.exportTo, _to, first: false),
            if (invalid && _phase == _Phase.range)
              Text(
                l.exportRangeInvalid,
                key: const ValueKey('export.rangeInvalid'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.dangerFg),
              ),
            if (_phase == _Phase.running) ...[
              const SizedBox(height: DsSpace.s12),
              const LinearProgressIndicator(),
              const SizedBox(height: DsSpace.s8),
              Text(
                l.exportProgress(fileSizeLabel(l, _bytes, locale: locale)),
                key: const ValueKey('export.progress'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
            ],
            if (_notice case final n?) ...[
              const SizedBox(height: DsSpace.s12),
              Text(
                n,
                key: const ValueKey('export.notice'),
                style: dsTextStyle(DsType.body).copyWith(
                  color: _noticeIsError ? colors.dangerFg : colors.textPrimary,
                ),
              ),
            ],
          ],
        ),
      ),
      actions: [
        if (_phase == _Phase.running)
          TextButton(
            key: const ValueKey('export.cancel'),
            onPressed: () => _cancel?.cancel(),
            child: Text(l.exportCancel),
          )
        else if (_phase == _Phase.ended)
          TextButton(
            key: const ValueKey('export.close'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.exportClose),
          )
        else ...[
          TextButton(
            key: const ValueKey('export.cancel'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.exportCancel),
          ),
          FilledButton(
            key: const ValueKey('export.start'),
            onPressed: invalid ? null : () => unawaited(_start()),
            child: Text(l.exportStart),
          ),
        ],
      ],
    );
  }
}

/// A labelled day that opens the date picker on tap.
class SettingDayRow extends StatelessWidget {
  /// Creates the row.
  const SettingDayRow({
    super.key,
    required this.label,
    required this.value,
    required this.onTap,
  });

  /// The label.
  final String label;

  /// The formatted day.
  final String value;

  /// Opens the picker; null disables the row.
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
    child: Row(
      children: [
        SizedBox(
          width: 80,
          child: Text(
            label,
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: context.dsColors.textPrimary),
          ),
        ),
        OutlinedButton(onPressed: onTap, child: Text(value)),
      ],
    ),
  );
}

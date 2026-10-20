// SPDX-License-Identifier: Apache-2.0

/// The export of the audit log: the server's NDJSON stream with hashes written unchanged, and a
/// CSV converted on the device from the same stream (docs/spec/01-product.md 5.2 A6;
/// ADR 0021; brief M4-29 Design note 3).
///
/// Nothing is kept in the app: the bytes go from the response to the file the person chose.
/// The NDJSON keeps every hash link, so a file cut short is detectable offline; a CSV loses
/// that property and serves spreadsheets only.
library;

import 'dart:async';
import 'dart:convert';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../self/self_feedback.dart';
import 'audit_repository.dart';

/// Asks the person where to save a file named [suggestedName] (the save dialog of the host)
/// and returns the sink of that file, or null when the person cancelled.
///
/// The host supplies it because a save dialog needs the platform (the desktop app's file
/// panel, the console's download); a host without one gets no export buttons.
///
/// Example (desktop host with a file panel plugin):
/// ```dart
/// AuditSaveTarget target = (name) async {
///   final path = await pickSavePath(name);
///   return path == null ? null : File(path).openWrite();
/// };
/// ```
typedef AuditSaveTarget = Future<StreamConsumer<List<int>>?> Function(
  String suggestedName,
);

/// The two formats of the export.
enum AuditExportFormat {
  /// The server's NDJSON with hashes, unchanged.
  ndjson,

  /// A CSV converted on the device.
  csv,
}

/// The CSV columns in their order (brief M4-29 Design note 3); the header row uses the field
/// names of `AuditEvent`, so a spreadsheet and the NDJSON name the same things.
const List<String> auditCsvColumns = [
  'occurred_at',
  'actor_principal_id',
  'action',
  'decision',
  'resource_kind',
  'resource_id',
  'deny_reason',
  'correlation_id',
  'seq',
  'hash',
  'prev_hash',
];

/// One CSV cell for [value] (RFC 4180): a text that starts with `=`, `+`, `-` or `@` gets a
/// leading apostrophe so a spreadsheet does not run it as a formula; a cell with a comma, a
/// quote, a carriage return or a line feed is quoted, its quotes doubled.
///
/// Example: `auditCsvCell('=1+1')` is `'=1+1`; `auditCsvCell('a,"b"')` is `"a,""b"""`.
String auditCsvCell(Object? value) {
  var text = value == null ? '' : '$value';
  if (text.isNotEmpty && '=+-@'.contains(text[0])) text = "'$text";
  if (text.contains(RegExp('[,"\r\n]'))) {
    return '"${text.replaceAll('"', '""')}"';
  }
  return text;
}

/// The CSV row of one `AuditEvent` [event] in the order of [auditCsvColumns], ended by CRLF.
String auditCsvRow(Map<String, Object?> event) =>
    '${[for (final c in auditCsvColumns) auditCsvCell(event[c])].join(',')}\r\n';

/// Converts the NDJSON [ndjson] (one `AuditEvent` per line, in chunks of any size) into CSV
/// bytes: the header row, then one row per line. An empty line is skipped; a line that is not
/// a JSON object ends the stream with a [FormatException].
Stream<List<int>> auditNdjsonToCsv(Stream<List<int>> ndjson) async* {
  yield utf8.encode('${auditCsvColumns.join(',')}\r\n');
  final lines = ndjson.transform(utf8.decoder).transform(const LineSplitter());
  await for (final line in lines) {
    if (line.trim().isEmpty) continue;
    final event = jsonDecode(line);
    if (event is! Map<String, Object?>) {
      throw const FormatException('an export line is not a JSON object');
    }
    yield utf8.encode(auditCsvRow(event));
  }
}

/// The file name of an export of [from] to [to] in [format], for example
/// `audit-2026-10-16-2026-10-18.ndjson` (UTC days).
String auditExportFileName(
  DateTime from,
  DateTime to,
  AuditExportFormat format,
) {
  String day(DateTime t) => t.toUtc().toIso8601String().substring(0, 10);
  final ext = format == AuditExportFormat.csv ? 'csv' : 'ndjson';
  return 'audit-${day(from)}-${day(to)}.$ext';
}

/// "Export" and "Export as CSV" for the time range [from] to [to] of the filter.
///
/// Without a time range the buttons say so instead of exporting the whole log. Each export
/// asks [saveTarget] for the file, then streams `exportAuditEvents` into it; a stream that
/// breaks off says that the file is incomplete. The step-up of `audit.read` comes from the
/// server through [repository].
class AuditExportBar extends StatefulWidget {
  /// Creates the bar.
  const AuditExportBar({
    super.key,
    required this.repository,
    required this.saveTarget,
    required this.from,
    required this.to,
  });

  /// The audit log's calls.
  final AuditRepository repository;

  /// The save dialog of the host.
  final AuditSaveTarget saveTarget;

  /// The start of the filter's time range, or null.
  final DateTime? from;

  /// The end of the filter's time range (exclusive), or null.
  final DateTime? to;

  @override
  State<AuditExportBar> createState() => _AuditExportBarState();
}

class _AuditExportBarState extends State<AuditExportBar> {
  bool _busy = false;
  SelfNotice? _notice;

  Future<void> _export(AuditExportFormat format) async {
    final l = AppLocalizations.of(context);
    final from = widget.from;
    final to = widget.to;
    if (from == null || to == null) {
      setState(() => _notice = SelfNotice(l.auditExportNeedsRange));
      return;
    }
    final sink = await widget.saveTarget(auditExportFileName(from, to, format));
    if (sink == null || !mounted) return;
    setState(() {
      _busy = true;
      _notice = null;
    });
    var opened = false;
    SelfNotice? notice;
    try {
      final ndjson = widget.repository.exportAudit(
        from: from,
        to: to,
        purpose: l.stepUpPurposeAuditExport,
      );
      final bytes = format == AuditExportFormat.csv
          ? auditNdjsonToCsv(ndjson)
          : ndjson;
      // A sink does not always report an error of the source (a stream controller passes it
      // on instead), so the error is caught here and the sink sees the bytes up to it.
      Object? broke;
      await sink.addStream(
        bytes
            .map((chunk) {
              opened = true;
              return chunk;
            })
            .handleError((Object e) => broke = e),
      );
      if (broke case final e?) throw e;
      notice = SelfNotice(l.auditExportDone);
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      notice = failure == null
          ? null
          : SelfNotice(
              opened || e is FormatException
                  ? l.auditExportFailed
                  : failure.text,
              tone: DsTone.danger,
            );
    } finally {
      try {
        await sink.close();
      } on Object {
        // The file could not be closed; the notice of the export stands.
      }
    }
    if (mounted) {
      setState(() {
        _busy = false;
        _notice = notice;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('audit.export'),
              onPressed: _busy ? null : () => _export(AuditExportFormat.ndjson),
              child: Text(l.auditExport),
            ),
            OutlinedButton(
              key: const ValueKey('audit.exportCsv'),
              onPressed: _busy ? null : () => _export(AuditExportFormat.csv),
              child: Text(l.auditExportCsv),
            ),
          ],
        ),
        if (_notice case final n?)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: KeyedSubtree(
              key: const ValueKey('audit.exportNotice'),
              child: selfNoticeBanner(n),
            ),
          ),
      ],
    );
  }
}

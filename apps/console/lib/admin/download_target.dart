// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';

import '../platform/browser.dart';

/// The save target of the audit export in the console (A6 of 01 5.2, brief M6-04 Design note
/// 4): the bytes of the export are collected in memory and offered as one browser download when
/// the export closes the file; an export that discards it offers no download. The media type
/// follows the file name (`.csv` or `.ndjson`).
///
/// Example: `AuditLogView(saveTarget: downloadSaveTarget(environment.downloads!), ...)`.
AuditSaveTarget downloadSaveTarget(FileDownloads downloads) =>
    (name) async => _DownloadSink(name, downloads);

/// Collects the chunks of the export and saves them once on [close]; [discard] forgets them.
class _DownloadSink implements AuditSaveFile {
  _DownloadSink(this.name, this.downloads);

  final String name;
  final FileDownloads downloads;
  final BytesBuilder _bytes = BytesBuilder(copy: false);
  bool _closed = false;

  @override
  Future<void> addStream(Stream<List<int>> stream) =>
      stream.forEach(_bytes.add);

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    final type = name.endsWith('.csv')
        ? 'text/csv'
        : name.endsWith('.ndjson')
        ? 'application/x-ndjson'
        : 'application/octet-stream';
    downloads.save(name, type, _bytes.takeBytes());
  }

  @override
  Future<void> discard() async {
    _closed = true;
    _bytes.clear();
  }
}
